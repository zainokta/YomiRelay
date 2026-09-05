# YomiRelay

YomiRelay is a local visual-novel dialogue relay that puts Japanese text into ordinary selectable browser DOM for tools such as Yomitan. Ren'Py games use the normal YomiRelay dialogue pipeline; AQUARIUM/NeXAS currently has an experimental read-only memory preview in Reader. No OCR or clipboard monitoring is used.

## Requirements and quick start

- Go 1.23 or newer
- Node.js 22.22.2 or newer, npm
- Linux, Windows, or macOS for the main app
- Linux + Steam Proton for the current AQUARIUM native preview

For development, run `./run.sh`, then open **http://127.0.0.1:5173**. The script installs frontend dependencies when needed, builds the AQUARIUM native preview helper, and starts the Go backend plus Octane/Vite frontend together. No native debug flag or second terminal is required.

For a production build, run `./build.sh`, then keep `dist/yomirelay` and `dist/yomirelay-aquarium` together. Start `./dist/yomirelay` and open **http://127.0.0.1:17321**. The frontend is embedded into the main binary and needs no Node.js at runtime.

Translation is optional. Install the Codex CLI, sign in with your ChatGPT account, and make `codex` available on `PATH` if you want English translations. YomiRelay does not check this at startup; the Reader starts in Japanese-only mode. If the CLI is missing or not authenticated, enabling translation simply returns the Reader to Japanese-only mode. See the [Codex CLI guide](https://learn.chatgpt.com/docs/codex/cli) for setup.

The backend defaults to HTTP `127.0.0.1:17321` and loopback UDP `127.0.0.1:17322`. `YOMIRELAY_HTTP_ADDR` and `YOMIRELAY_UDP_ADDR` may override these addresses, but they must remain loopback addresses.

## Safety and discovery

Steam discovery checks known platform Steam roots and only configured `steamapps/libraryfolders.vdf` files and `appmanifest_*.acf` manifests. It does not recursively scan the disk. A game is identified as Ren'Py from a real `game` directory plus runtime, archive, or script evidence; its name is never a signal.

Ren'Py hook installation writes only `<install>/game/_yomirelay_hook.rpy`. It uses the two YomiRelay ownership markers, refuses symbolic links and unmanaged files, and atomically updates managed files. Removing a hook also requires ownership markers. Restart the game after installing a hook. Canonical dialogue is kept in memory (up to 1000 entries per game) and is not persisted.

The Reader renders normal light-DOM text with `lang="ja"`, so Yomitan can inspect it through ordinary browser scanning. YomiRelay does not call Yomitan APIs. Optional translation uses only the local `codex` executable after the user enables it.

AQUARIUM is different: its current experimental source is a read-only process-memory snapshot under Linux/Proton. It does not inject code, patch the game, write game files, or publish memory candidates into canonical Reader history. Memory candidates can be stale, duplicated, backlog-related, or out of story order, so they are intentionally shown only in the experimental Reader preview.

## Manual acceptance test

1. Install Go, Node.js 22.22.2 or newer, and npm.
2. Clone the repository and change into its directory.
3. Run `./run.sh`.
4. Confirm the native helper, backend, and Vite startup labels are printed.
5. Open `http://127.0.0.1:5173`.
6. Confirm Steam roots and configured libraries are discovered without a whole-disk scan.
7. Confirm an installed Ren'Py game appears, install its hook, restart the game, and verify dialogue appears as selectable Japanese DOM text in Reader.
8. Confirm Yomitan can scan the Japanese text normally.
9. Confirm stopping YomiRelay never blocks the Ren'Py game.
10. Confirm `Clear History` affects only the selected game's canonical dialogue.

For AQUARIUM on Linux/Steam Proton:

11. Start AQUARIUM through Steam.
12. Confirm AQUARIUM appears with engine `NeXAS` and source `Experimental native preview`.
13. Open AQUARIUM in Reader.
14. Click **Scan native text**.
15. Confirm the preview reports the process, executable hash, memory read size, and displayable candidates.
16. Confirm candidate speaker/text is normal selectable DOM with `lang="ja"` and Yomitan can scan it.
17. Confirm the warning says memory candidates are not chronological and are never stored in Reader history or sent to the translation queue.
18. Scan again and confirm the preview is replaced rather than appended to canonical history.
19. Confirm `GET /api/dialogues?gameId=2515070` remains unaffected by native scans.
20. Stop YomiRelay while AQUARIUM is running and confirm AQUARIUM is unaffected.

## Build details

`./build.sh` installs frontend dependencies when needed, builds the Octane/Vite client, runs `go test ./...`, and writes:

```text
dist/yomirelay
dist/yomirelay-aquarium
```

The frontend uses relative `/api/...` requests and one shared `/api/events` stream. The native preview helper is a separate process so read-only game-memory inspection stays isolated from the HTTP backend.

The source-specific layout is intentionally separated:

```text
cmd/yomirelay/                 main application
cmd/yomirelay-aquarium/        AQUARIUM native preview helper
internal/source/aquarium/      AQUARIUM/NeXAS detection and candidate parsing
internal/dialogue/             canonical chronological dialogue store
internal/events/               canonical SSE event broker
```

`YOMIRELAY_AQUARIUM_HELPER` remains available only as an optional developer override for the helper path. Normal `./run.sh` and `./build.sh` usage does not require setting it.

## AQUARIUM native preview status

Steam discovery identifies AQUARIUM (App ID `2515070`) by its verified NeXAS-compatible archive/PE fingerprint. The investigated executable hash is allowlisted before native memory inspection runs.

Live chronological NeXAS hooking is not implemented yet. The current helper scans readable anonymous memory from the Steam Proton `Aquarium.exe` process and finds speaker-tagged candidate strings. Because memory address order is not story order, YomiRelay does **not** turn these candidates into `dialogue.Dialogue`, does **not** append them to history, and does **not** publish them over the canonical SSE stream.

No environment feature flag is required. Open AQUARIUM in Reader and use **Scan native text**. If the helper cannot access the Proton process or the executable version is unsupported, Reader shows the error without changing the game.
