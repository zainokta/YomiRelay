# YomiRelay

YomiRelay is a local visual-novel dialogue relay that puts Japanese text into ordinary selectable browser DOM for tools such as Yomitan. Ren'Py uses a generated `.rpy` callback hook; AQUARIUM uses an experimental live NeXAS execution hook on Linux/Steam Proton. No OCR, clipboard monitoring, memory polling, or manual native preview is used.

## Requirements and quick start

- Go 1.23 or newer
- Node.js 22.22.2 or newer, npm
- Linux, Windows, or macOS for the main app
- Linux + Steam Proton for the current AQUARIUM NeXAS hook

For development, run `./run.sh`, then open **http://127.0.0.1:5173**. The script installs frontend dependencies when needed, builds the AQUARIUM helper, and starts the Go backend plus Octane/Vite frontend together. No native feature flag or second terminal is required.

For a production build, run `./build.sh`, then keep `dist/yomirelay` and `dist/yomirelay-aquarium` together. Start `./dist/yomirelay` and open **http://127.0.0.1:17321**. The frontend is embedded into the main binary and needs no Node.js at runtime.

Translation is optional. Install the Codex CLI, sign in with your ChatGPT account, and make `codex` available on `PATH` if you want English translations. YomiRelay does not check this at startup; the Reader starts in Japanese-only mode.

The backend defaults to HTTP `127.0.0.1:17321` and loopback UDP `127.0.0.1:17322`. `YOMIRELAY_HTTP_ADDR` and `YOMIRELAY_UDP_ADDR` may override these addresses, but they must remain loopback addresses.

## Safety and discovery

Steam discovery checks known platform Steam roots and configured `steamapps/libraryfolders.vdf` / `appmanifest_*.acf` files. It does not recursively scan the disk.

Ren'Py hook installation writes only `<install>/game/_yomirelay_hook.rpy`, verifies ownership markers, refuses unsafe paths, and can be removed from the Games page. Restart the game after installing a Ren'Py hook.

AQUARIUM does **not** receive a patched executable, injected DLL, proxy DLL, or modified game file. On Linux/Proton the helper uses Linux `perf_event_open` with a kernel-managed execute breakpoint at a verified NeXAS instruction. The breakpoint is owned by the helper file descriptors, so closing or killing the helper removes the hook in the kernel rather than leaving an `INT3` byte inside the game process.

The AQUARIUM executable must match the investigated SHA-256 and the NeXAS instruction signature must resolve exactly once. If either check fails, YomiRelay fails closed and does not attach a hook.

The Reader renders normal light-DOM text with `lang="ja"`, so Yomitan can inspect it through ordinary browser scanning. Canonical dialogue is kept in memory (up to 1000 entries per game) and is not persisted.

## AQUARIUM live NeXAS hook

For the supported AQUARIUM Steam build (App ID `2515070`), YomiRelay automatically starts `yomirelay-aquarium` when the backend starts. The helper waits for `Aquarium.exe` to appear under Steam Proton, validates the process identity, then attaches the execution hook to each current game thread and new threads discovered while the game is running.

At the verified NeXAS text dispatch point, the x86 text pointer is sampled from `EAX`. The helper reads at most 8192 bytes from that pointer, normalizes known NeXAS control tags, parses speaker/dialogue text, coalesces short progressive updates, and sends the resulting line over the existing loopback UDP protocol:

```text
Aquarium.exe
    ↓ NeXAS execute hook
YomiRelay native helper
    ↓ UDP 127.0.0.1:17322
Go backend
    ↓ Store + SSE
Reader
    ↓
Yomitan
```

The source is event-driven. It does not scan process memory repeatedly and there is no manual native-preview UI.

### Linux permissions

The hook relies on the kernel perf/hardware-breakpoint interface for a same-user Steam process. If the kernel or local security policy denies `perf_event_open`, the helper exits with a clear `perf hook permission denied` log message. YomiRelay does not change `perf_event_paranoid`, ptrace policy, capabilities, or any other system security setting automatically.

Useful diagnostic commands are:

```sh
cat /proc/sys/kernel/perf_event_paranoid
uname -a
```

## Manual acceptance test

1. Run `./run.sh`.
2. Open `http://127.0.0.1:5173`.
3. Confirm Ren'Py games still support Install Hook / Remove Hook and Reader behaves as before.
4. Confirm AQUARIUM appears as `NeXAS` with source `Live native hook (automatic)` when its executable build is supported.
5. Open AQUARIUM in Reader.
6. Start AQUARIUM through Steam/Proton if it is not already running.
7. Check backend logs for `[nexas] waiting for AQUARIUM Steam/Proton process`, then `[nexas] attached NeXAS execution hook`.
8. Advance normal Japanese story dialogue.
9. Confirm each accepted line is appended automatically to Reader history in event order.
10. Confirm speaker and Japanese text are selectable normal DOM and Yomitan can scan them.
11. Advance at least 20 lines and check for duplicate/typewriter spam or missing narration.
12. Open/close backlog and menus and note any text pollution for follow-up filtering.
13. Stop YomiRelay while AQUARIUM remains open and confirm the game continues normally.
14. Start YomiRelay again, advance another line, and confirm capture resumes.
15. Confirm `Clear History` works for AQUARIUM exactly like other canonical dialogue sources.

## Build details

`./build.sh` installs frontend dependencies when needed, builds the Octane/Vite client, runs `go test ./...`, and writes:

```text
dist/yomirelay
dist/yomirelay-aquarium
```

The source-specific layout is intentionally separated:

```text
cmd/yomirelay/                 main application and native-source supervisor
cmd/yomirelay-aquarium/        Linux/Proton NeXAS execution-hook helper
internal/source/aquarium/      AQUARIUM build verification, hook signature, text normalization
internal/dialogue/             canonical chronological dialogue store
internal/events/               canonical SSE event broker
```

`YOMIRELAY_AQUARIUM_HELPER` remains an optional developer override for the helper path. Normal `./run.sh` and production sibling-binary usage do not require setting it.

## Current AQUARIUM limitations

This is still experimental until verified against long real-game sessions. The current implementation is intentionally specific to the investigated AQUARIUM executable build and its x86 NeXAS hook signature. A Steam update, different executable, unexpected control tag, or Proton/kernel perf restriction may disable capture. The hook fails closed rather than guessing an address.

If a real-game test shows missed narration, duplicate rendering calls, backlog/menu pollution, or the sampled register differs on the installed build, capture/filter rules should be adjusted from observed data before generalizing this adapter to other NeXAS games.
