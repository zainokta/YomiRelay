# YomiRelay

YomiRelay is a local visual-novel dialogue relay that renders Japanese dialogue as ordinary selectable browser DOM, so browser tools such as Yomitan can inspect it directly.

Current sources:

- Ren'Py: generated `.rpy` callback hook → loopback UDP → YomiRelay.
- NeXAS: native execution-hook adapters inside the same YomiRelay process. AQUARIUM / あくありうむ。 (Steam App ID `2515070`) is the first supported profile on Linux + Steam Proton.

There is no OCR, clipboard polling, manual memory preview, delta memory polling, or separate native helper binary.

## Quick start

Requirements:

- Go 1.23+
- Node.js 22.22.2+
- npm

Development:

```sh
./run.sh
```

Open `http://127.0.0.1:5173`.

Production build:

```sh
./build.sh
./dist/yomirelay
```

Open `http://127.0.0.1:17321`.

`dist/yomirelay` is the only runtime binary.

## Architecture

```text
Ren'Py game
    ↓ generated callback hook
loopback UDP
    ↓
                    ┌──────────────────────────────┐
NeXAS game ───────►│ YomiRelay                    │
 execution hook    │                              │
                    │ source manager               │
                    │   ├─ Ren'Py UDP receiver     │
                    │   └─ NeXAS native profiles   │
                    │            ↓                 │
                    │       canonical Dialogue     │
                    │            ↓                 │
                    │       Store + SSE broker     │
                    └────────────┬─────────────────┘
                                 ↓
                              Reader
                                 ↓
                              Yomitan
```

Native NeXAS sources publish directly into the internal dialogue pipeline. They do not loop back through UDP.

## Project layout

```text
main.go                         application entry point
internal/app/                   application lifecycle and HTTP/UDP wiring
internal/source/nexas/          generic NeXAS runtime and source profile registry
internal/source/nexas/aquarium/ AQUARIUM-specific build/signature/text profile
internal/games/                 detected game registry
internal/dialogue/              canonical bounded history
internal/events/                SSE broker
internal/receiver/              Ren'Py UDP receiver
frontend/                       Octane browser UI
```

There is intentionally no `cmd/yomirelay-aquarium` helper. Adding another supported NeXAS title should be implemented as another profile in the NeXAS source catalog rather than creating another executable.

## Steam discovery

YomiRelay discovers Steam roots and configured `steamapps/libraryfolders.vdf` / `appmanifest_*.acf` files. It does not recursively scan the whole disk.

Ren'Py detection is based on game layout/runtime evidence. NeXAS detection is profile-driven and fail-closed: a registered game profile must validate its expected executable/build and hook signature before live hooking is enabled.

## AQUARIUM live NeXAS hook

For the currently verified AQUARIUM Steam build, YomiRelay:

1. detects the installed game through Steam,
2. validates the NeXAS-compatible executable fingerprint,
3. validates the known hook signature,
4. waits for `Aquarium.exe` under Steam Proton,
5. attaches Linux `perf_event_open` execute breakpoints to the game threads,
6. reads the text pointer from the sampled x86 register state,
7. normalizes NeXAS control tags,
8. coalesces progressive/typewriter updates,
9. appends accepted dialogue directly to canonical Reader history.

The hook does not patch `Aquarium.exe`, inject a DLL, modify game files, or poll the process memory continuously. Hardware breakpoint file descriptors are owned by the YomiRelay process and disappear when YomiRelay exits.

If the executable hash/signature changes or kernel perf policy blocks the observer, YomiRelay fails closed and leaves the game untouched.

Useful diagnostics:

```sh
cat /proc/sys/kernel/perf_event_paranoid
uname -a
```

Expected logs include:

```text
[nexas] AQUARIUM hook ready: ...
[nexas] waiting for AQUARIUM Steam/Proton process
[nexas] attached execution hook: game=AQUARIUM pid=... address=...
```

## Reader / Yomitan

Reader dialogue is ordinary DOM text with Japanese language metadata. YomiRelay does not integrate with Yomitan APIs; Yomitan works through its normal browser text scanning behavior.

Dialogue history is kept in memory and bounded per game. `Clear History` clears only the selected game's canonical history.

## Ren'Py hooks

Ren'Py hook installation writes only the managed YomiRelay `.rpy` hook inside the detected game's `game/` directory. Existing unmanaged game scripts are not overwritten. Restart a Ren'Py game after installing the hook.

## Configuration

Defaults:

```text
HTTP 127.0.0.1:17321
UDP  127.0.0.1:17322
```

Optional environment variables:

```text
YOMIRELAY_HTTP_ADDR
YOMIRELAY_UDP_ADDR
```

Both addresses must remain loopback addresses.

## Manual AQUARIUM test

1. `git pull`
2. `./run.sh`
3. open YomiRelay Reader and select AQUARIUM
4. start AQUARIUM through Steam/Proton
5. confirm the `[nexas] attached execution hook` log appears
6. advance Japanese story dialogue for at least 20 lines
7. confirm lines arrive automatically and in event order
8. confirm Yomitan can scan the rendered Japanese text
9. check for duplicate/typewriter spam, missing narration, or backlog/menu pollution
10. stop YomiRelay while AQUARIUM remains open and confirm the game continues normally
11. start YomiRelay again and confirm capture resumes after the next dialogue event

If a real-game test shows a mismatch, report the `[nexas]` logs and the behavior observed in Reader.
