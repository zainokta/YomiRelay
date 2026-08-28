# YomiRelay

YomiRelay is a local Ren'Py dialogue relay: it sends installed Ren'Py Steam-game dialogue to ordinary selectable browser text on localhost. It is not an OCR, translator, dictionary, clipboard, process-injection, or remote-access tool.

## Requirements and quick start

- Go 1.23 or newer
- Node.js 22.22.2 or newer, npm
- Linux, Windows, or macOS

For development, run `./dev.sh`, then open **http://127.0.0.1:17321**. For a production binary, run `./build.sh`, then `./dist/yomirelay` and open the same URL. The frontend uses Vite only during development/build; the binary embeds its static assets and needs no Node.js at runtime.

The backend defaults to HTTP `127.0.0.1:17321` and loopback UDP `127.0.0.1:17322`. `YOMIRELAY_HTTP_ADDR` and `YOMIRELAY_UDP_ADDR` may override these addresses, but they must remain loopback addresses.

## Safety and discovery

Steam discovery checks known platform Steam roots and only configured `steamapps/libraryfolders.vdf` files and `appmanifest_*.acf` manifests. It does not recursively scan the disk. A game is identified as Ren'Py from a real `game` directory plus runtime, archive, or script evidence; its name is never a signal.

Hook installation writes only `<install>/game/_yomirelay_hook.rpy`. It uses the two YomiRelay ownership markers, refuses symbolic links and unmanaged files, and atomically updates managed files. Removing a hook also requires ownership markers. Restart the game after installing a hook. Dialogue is kept in memory (up to 1000 entries per game) and is not persisted.

The Reader renders normal light-DOM text with `lang="ja"`, so Yomitan can inspect it through its ordinary browser scanning behavior. YomiRelay does not call Yomitan APIs. The MVP has no OCR, screenshots, clipboard monitoring, process scanning/injection, Textractor, translation, dictionaries, furigana, Anki, accounts, authentication, cloud sync, persistence, game launching, or non-Ren'Py source.

## Manual acceptance test

1. Install Go, Node.js 22.22.2 or newer, and npm.
2. Clone the repository and change into its directory.
3. Run `./dev.sh`.
4. Confirm the backend and Vite startup labels are printed.
5. Open `http://127.0.0.1:17321` in a browser.
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

## Build details

`./build.sh` installs frontend dependencies when needed, builds the Octane/Vite client, runs `go test ./...`, and writes `dist/yomirelay`. The frontend uses relative `/api/...` requests and one shared `/api/events` stream. No persistence or remote binding is included.
