# YomiRelay

YomiRelay is a local Ren'Py dialogue relay: it sends installed Ren'Py Steam-game dialogue to ordinary selectable browser text on localhost. The Reader can optionally ask a locally authenticated Codex CLI for English translations and Japanese word glosses. Experimental AQUARIUM/NeXAS detection and read-only native diagnostics are also available. They do not yet provide live Reader dialogue. No OCR or clipboard monitoring is used.

## Requirements and quick start

- Go 1.23 or newer
- Node.js 22.22.2 or newer, npm
- Linux, Windows, or macOS

For development, run `./run.sh`, then open **http://127.0.0.1:5173**. For a production binary, run `./build.sh`, then `./dist/yomirelay` and open **http://127.0.0.1:17321**. The frontend uses Vite only during development/build; the binary embeds its static assets and needs no Node.js at runtime.

Translation is optional. Install the Codex CLI, sign in with your ChatGPT account, and make `codex` available on `PATH` if you want English translations. YomiRelay does not check this at startup; the Reader starts in Japanese-only mode. If the CLI is missing or not authenticated, enabling translation simply returns the Reader to Japanese-only mode. See the [Codex CLI guide](https://learn.chatgpt.com/docs/codex/cli) for setup.

The backend defaults to HTTP `127.0.0.1:17321` and loopback UDP `127.0.0.1:17322`. `YOMIRELAY_HTTP_ADDR` and `YOMIRELAY_UDP_ADDR` may override these addresses, but they must remain loopback addresses.

## Safety and discovery

Steam discovery checks known platform Steam roots and only configured `steamapps/libraryfolders.vdf` files and `appmanifest_*.acf` manifests. It does not recursively scan the disk. A game is identified as Ren'Py from a real `game` directory plus runtime, archive, or script evidence; its name is never a signal.

Hook installation writes only `<install>/game/_yomirelay_hook.rpy`. It uses the two YomiRelay ownership markers, refuses symbolic links and unmanaged files, and atomically updates managed files. Removing a hook also requires ownership markers. Restart the game after installing a hook. Dialogue is kept in memory (up to 1000 entries per game) and is not persisted.

The Reader renders normal light-DOM text with `lang="ja"`, so Yomitan can inspect it through its ordinary browser scanning behavior. YomiRelay does not call Yomitan APIs. Optional translation uses only the local `codex` executable after the user enables it; the MVP has no OCR, screenshots, clipboard monitoring, process scanning/injection, Textractor, dictionary APIs, furigana, Anki, accounts, authentication, cloud sync, persistence, game launching, or non-Ren'Py source.

## Manual acceptance test

1. Install Go, Node.js 22.22.2 or newer, and npm.
2. Clone the repository and change into its directory.
3. Run `./run.sh`.
4. Confirm the backend and Vite startup labels are printed.
5. Open `http://127.0.0.1:5173` in a browser (development frontend; production uses `http://127.0.0.1:17321`).
6. Confirm the YomiRelay Games page loads as normal HTML.
7. Confirm the page is reachable only on the configured local address.
8. Confirm Steam roots and configured libraries are discovered without a whole-disk scan.
9. Confirm an installed Ren'Py game with a `game` directory and runtime/archive/script evidence appears.
10. Confirm a non-Ren'Py installation or bare `game` directory does not appear.
11. Confirm the game card shows its name, app ID, install path, Ren'Py engine, and hook state.
12. Click Refresh and confirm discovery completes without losing valid games.
13. Click Install Hook for a detected game.
14. Confirm the notice says `Restart the game to activate the hook.`
15. Confirm the hook exists at `<install>/game/_yomirelay_hook.rpy` and begins with both ownership markers.
16. Confirm an unmanaged existing hook is refused and remains byte-for-byte unchanged.
17. Restart the selected Ren'Py game.
18. Open Reader and select the game.
19. Advance dialogue and confirm each statement appears as selectable Japanese light-DOM text.
20. Select a dialogue phrase with the browser and confirm ordinary Yomitan scanning can inspect it.
21. Open Reader in two browser windows and confirm both receive live dialogue.
22. Scroll away from the bottom, advance dialogue, and confirm the existing scroll position is preserved and Jump to latest appears.
23. Return to the bottom or click Jump to latest and confirm new dialogue follows the latest entry.
24. Stop YomiRelay while the game is running and confirm gameplay continues without waiting for the relay.
25. Restart YomiRelay, advance dialogue, and confirm reception resumes; confirm Clear History affects only the selected game.
26. Stop the game, click Remove Hook, and confirm only the managed hook is removed; confirm an unmanaged hook is never deleted.
27. Confirm the Reader starts with `Enable English translation` off and no Codex process is started.
28. Click `Enable English translation` and confirm existing history is translated progressively without hiding Japanese text.
29. Advance the game and confirm each new Japanese line appears before its English translation finishes.
30. Hover or keyboard-focus a glossed Japanese word and confirm kana plus its English meaning appear.
31. Confirm the full English sentence appears below the Japanese sentence.
32. Click `Disable translation` and confirm translations/tooltips hide while Japanese dialogue continues.
33. Remove `codex` from `PATH` or use an unauthenticated CLI, enable translation, and confirm the button returns to off with no translation error while dialogue continues.

## Build details

`./build.sh` installs frontend dependencies when needed, builds the Octane/Vite client, runs `go test ./...`, and writes `dist/yomirelay`. The frontend uses relative `/api/...` requests and one shared `/api/events` stream. Translation is not required for startup; it invokes `codex exec` only after the Reader button is enabled. No persistence or remote binding is included.

## AQUARIUM native diagnostics (experimental)

Steam discovery finds AQUARIUM (App ID `2515070`) automatically. The Games page shows its NeXAS engine and executable support status.
Live dialogue capture is not ready. Install Hook stays disabled, and install/remove requests return `SOURCE_UNAVAILABLE` without writing game files.

Build the backend and native helper with `./build.sh`. Then enable diagnostics:

```sh
YOMIRELAY_NATIVE_DEBUG=1 ./dist/yomirelay
```

Open Games and select **Inspect native source** on AQUARIUM. The separate helper reads the running Proton process without injection or debugger attachment.
The panel shows UTF-8 memory candidates, the process ID, and executable hash. Candidates can include old script, backlog, and menu text.
They never enter Reader history. Inspection is manual and temporary, not the final live source.

For development with the existing `./run.sh`, build the helper first:

```sh
go build -o dist/yomirelay-aquarium ./native/aquarium
YOMIRELAY_NATIVE_DEBUG=1 YOMIRELAY_AQUARIUM_HELPER="$PWD/dist/yomirelay-aquarium" ./run.sh
```

Diagnostics require Linux, the investigated executable hash, and permission to read the Proton process.
Run YomiRelay outside containers or sandboxes that hide host processes. The helper reports access errors and does not change system security settings.
Only one bounded inspection runs at a time. Closing YomiRelay cannot interrupt gameplay because the helper only reads memory.

No files are installed in the game directory, so this diagnostic stage needs no ownership manifest or game-file removal.
A future installer must add ownership tracking before it writes native hook files.
See [the engine investigation](docs/aquarium-engine-investigation.md) for binary evidence and pending live-capture acceptance tests.
