# Codex-backed Japanese translation design

**Date:** 2026-08-28  
**Status:** Design approved; written spec ready for review

## Goal

Add an optional English-learning layer to the existing YomiRelay Reader. The
Reader must continue displaying live Ren'Py Japanese dialogue immediately, while
an explicitly enabled feature asks the user's locally authenticated Codex CLI
to provide:

- an English translation for each complete Japanese sentence; and
- kana plus a concise English meaning for each Japanese word.

The feature is English-only for this version and is disabled by default.

## Decisions

- Use the locally installed and ChatGPT-authenticated `codex` CLI, not a
  browser API or an OpenAI API key.
- Use the exact model `gpt-5.6-luna` for translation runs.
- Invoke `codex exec` non-interactively, one sentence at a time, with a small
  sequential queue in the Reader.
- Keep translation results in memory only. Do not add persistence.
- Translate existing history progressively after enabling, then translate new
  dialogue as it arrives.
- Keep Japanese as ordinary selectable light-DOM text with `lang="ja"`.
- Treat every Codex process failure, authentication failure, timeout, malformed
  response, or invalid segment result as translation unavailability. The UI
  silently disables translation and keeps reading Japanese dialogue.

`codex exec` is the appropriate local boundary because it is documented for
non-interactive scripted runs, accepts prompts through stdin, supports a model
override, and can run ephemerally in a read-only sandbox:

- <https://learn.chatgpt.com/docs/non-interactive-mode>
- <https://learn.chatgpt.com/docs/developer-commands?surface=cli>

## User experience

The Reader adds an `Enable English translation` button beside the existing game
selector and history controls.

### Disabled state

- The button is present but off on initial Reader load.
- Loading the Reader, selecting a game, receiving dialogue, and scrolling the
  Reader perform no Codex work.
- Each dialogue line shows its existing Japanese text and timestamp exactly as
  before.

### Enabled state

- Clicking the button changes it to `Disable translation`.
- Current history is queued for translation in display order.
- Existing Japanese lines remain visible immediately while translations arrive
  progressively.
- New dialogue is appended immediately and queued behind pending translation
  work.
- A successful line renders its English sentence below the Japanese sentence.
- Japanese word segments expose kana and English meaning on hover and keyboard
  focus.
- The existing auto-scroll and `Jump to latest` behavior remains unchanged.

### Disabled again

- Clicking `Disable translation` hides translated presentation and stops applying
  queued results.
- The Japanese source remains visible and live reception continues.
- Re-enabling may reuse in-memory results already obtained during the current
  backend process or Reader session.

### Automatic failure

If translation cannot start or a translation request fails, the Reader:

- turns the feature off;
- hides or ignores translation results that are not already applied;
- makes no translation error banner, toast, or per-line error message; and
- continues rendering the Japanese dialogue stream.

The backend may log a short failure reason without logging the source sentence
or the user's credentials.

## Architecture

```text
Reader toggle
      |
      v
frontend translation queue ---> POST /api/translate ---> Go API
                                      |
                                      v
                            codex exec --model gpt-5.6-luna
                                      |
                                      v
                           validated JSON translation result
                                      |
                                      v
                    Japanese DOM + gloss spans + English sentence
```

The existing dialogue stream remains the source of truth for reading. The
translation layer is an asynchronous decoration over that stream and never
blocks UDP ingestion, SSE delivery, history loading, or rendering.

## Backend design

### Translation model

Add `internal/translation` with the smallest types needed by the API:

```go
type Segment struct {
    Text    string `json:"text"`
    Kana    string `json:"kana"`
    Meaning string `json:"meaning"`
}

type Result struct {
    Translation string    `json:"translation"`
    Segments    []Segment `json:"segments"`
}
```

The package owns prompt construction, the `codex exec` invocation, JSON
decoding, result validation, and an in-memory cache keyed by game ID and source
text. The cache is bounded to 1000 entries, matching the in-memory dialogue
history limit, and is cleared when the process exits.

The concrete runner invokes the executable named `codex` from `PATH`. It passes
the source through stdin and never builds a shell command string from dialogue
content. Each run uses an empty temporary working directory plus:

```text
codex exec --cd <empty-temp-dir> --ephemeral --sandbox read-only \
  --model gpt-5.6-luna --skip-git-repo-check --color never -
```

The temporary working directory prevents game dialogue from being interpreted
as instructions about the YomiRelay repository. The prompt explicitly marks
the Japanese sentence as untrusted data, requests translation only, and
requires one JSON object with no Markdown fences or explanatory text.

The command uses the user's existing Codex authentication. YomiRelay neither
reads nor stores Codex credentials.

### Result contract and validation

For the source sentence `カフェ。`, the model must return:

```json
{
  "translation": "English sentence",
  "segments": [
    {"text":"カフェ","kana":"かふぇ","meaning":"cafe"},
    {"text":"。","kana":"","meaning":""}
  ]
}
```

Validation rejects a result when:

- `translation` is empty or only whitespace;
- `segments` is empty;
- any segment has empty `text`;
- a non-whitespace, non-punctuation segment has empty `kana` or empty
  `meaning`; or
- concatenating segment `text` values does not exactly equal the original
  Japanese sentence.

Whitespace and punctuation may be segments with empty kana and meaning. Word
segments carry both values. Exact source reconstruction prevents a model
response from dropping, duplicating, or reordering visible Japanese text.

### HTTP endpoint

Add:

```text
POST /api/translate
Content-Type: application/json
```

Request:

```json
{
  "gameId": "123456",
  "text": "カフェのドアを押し開けると、いつものコーヒーの香りがふわりと鼻腔をくすぐった。"
}
```

The handler requires a known decimal game ID, a non-empty sentence, and a
bounded request body. It calls the translator with the request context so a
disconnected or cancelled browser request can terminate the subprocess.

Success response:

```json
{
  "translation": "When I pushed open the cafe door, the familiar aroma of coffee gently tickled my nose.",
  "segments": [
    {"text":"カフェ","kana":"かふぇ","meaning":"cafe"},
    {"text":"の","kana":"の","meaning":"of"},
    {"text":"ドア","kana":"どあ","meaning":"door"},
    {"text":"を","kana":"を","meaning":"object marker"},
    {"text":"押し開けると","kana":"おしあけると","meaning":"when I pushed open"},
    {"text":"、","kana":"","meaning":""},
    {"text":"いつもの","kana":"いつもの","meaning":"familiar"},
    {"text":"コーヒー","kana":"こーひー","meaning":"coffee"},
    {"text":"の","kana":"の","meaning":"of"},
    {"text":"香り","kana":"かおり","meaning":"aroma"},
    {"text":"が","kana":"が","meaning":"subject marker"},
    {"text":"ふわりと","kana":"ふわりと","meaning":"gently"},
    {"text":"鼻腔","kana":"びくう","meaning":"nasal passages"},
    {"text":"を","kana":"を","meaning":"object marker"},
    {"text":"くすぐった","kana":"くすぐった","meaning":"tickled"},
    {"text":"。","kana":"","meaning":""}
  ]
}
```

Translation-unavailable responses use the existing JSON error envelope and a
503 status. The frontend deliberately treats this response like every other
translation failure and does not render it as a user-facing error.

The existing API remains local-only, and this endpoint does not accept a game
filesystem path or expose any Codex credential data.

## Frontend design

`ReaderPage` owns:

- the translation enabled flag, initially `false`;
- a translation map keyed by the stable dialogue timestamp plus source text;
- one sequential queue for pending lines; and
- cancellation/ignore state for work after manual disable or automatic failure.

The existing `Dialogue[]` state is not mutated to contain translation fields.
This keeps history merging, SSE buffering, clear-history behavior, and scroll
logic unchanged.

`DialogueLine` receives an optional `Result` and renders:

```html
<div class="dialogue-text" lang="ja">
  <span class="gloss-word" tabindex="0">カフェ</span>のドア。
</div>
<div class="dialogue-translation" lang="en">The cafe door.</div>
```

Each glossable word has a hover/focus tooltip containing kana and meaning.
Punctuation and whitespace remain normal visible text without a tooltip. The
Japanese sentence remains selectable as a single ordinary DOM text flow; no
canvas, SVG text, image, character-by-character rendering, or Yomitan API is
introduced.

The translation control uses a real button with an accessible pressed state.
While the queue is active it may expose a compact `Translating…` status, but it
does not replace or delay the Japanese dialogue.

## Error handling and safety

- Missing `codex` executable: disable translation silently.
- Codex not authenticated: disable translation silently.
- Non-zero Codex exit: disable translation silently.
- Context cancellation or timeout: ignore the result and leave Japanese text.
- Invalid JSON or source mismatch: disable translation silently.
- Translation API unavailable: disable translation silently.
- Dialogue continues through all of the above because translation is never on
  the receiver or SSE critical path.
- Codex runs with read-only sandboxing, an empty working directory, ephemeral
  session storage, and no shell interpolation of dialogue.
- The request body is capped at 16 KiB, the source sentence at 8192 UTF-8
  bytes, Codex runs have a 30-second context timeout, and captured stdout is
  capped at 256 KiB.

## Testing and acceptance

### Backend tests

Add focused tests for:

- valid JSON result decoding;
- exact segment reconstruction;
- malformed JSON, empty translation, empty segments, and source mismatch;
- successful Codex invocation using a fake executable or injected command
  function;
- missing executable and non-zero exit handling;
- API success response and unknown/invalid game ID rejection; and
- request cancellation not leaving a running translation subprocess.

Existing backend tests must continue to pass.

### Frontend checks

Run the existing TypeScript typecheck and production build. Manually verify:

1. The Reader starts with translation off and no Codex process is started.
2. Enabling translation progressively decorates existing lines.
3. New Ren'Py lines continue appearing before their translations finish.
4. Hovering or focusing a word shows kana and English meaning.
5. The complete English sentence appears below the Japanese sentence.
6. Disabling hides the translation layer while Japanese remains live.
7. Removing or breaking `codex` automatically returns the Reader to Japanese-only
   mode without an error message.
8. Scrolling, history loading, clear history, game switching, and Yomitan's
   ordinary selectable-text behavior remain intact.

Required commands:

```text
go test ./...
npm run typecheck
npm run build
```

## Scope boundaries

This feature does not add:

- an OpenAI API key setting;
- direct ChatGPT web automation;
- a Codex app-server integration;
- OCR, clipboard capture, screenshots, or process injection;
- dictionary APIs, Yomitan APIs, Anki export, or vocabulary tracking;
- persistent translations, accounts, remote access, or a target-language
  selector; or
- a background translation worker beyond the single sequential in-process
  queue.

The lazier app-server and multi-language options can be added only when
single-process translation latency or English-only scope becomes a demonstrated
limitation.
