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
                    │        + render-context filter│
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

Native NeXAS sources publish directly into the internal dialogue pipeline. They do not loop back through UDP. The NeXAS render-context filter is isolated from the Ren'Py UDP path.

## Project layout

```text
main.go                         application entry point
internal/app/                   application lifecycle and HTTP/UDP wiring
internal/source/nexas/          generic NeXAS runtime, process observer, render-context filter, hook resolver, and source profile registry
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

Ren'Py detection is based on game layout/runtime evidence. NeXAS detection is profile-driven and fail-closed: a registered game profile must validate its expected executable/build before live hooking is enabled.

The NeXAS instruction signature is intentionally **not** validated against the raw executable file during game discovery. The signature used by text-hooking tools describes the loaded process image. Under Wine/Proton, the final executable code visible at runtime can differ from the raw on-disk bytes, so YomiRelay resolves the signature only after the game process has been mapped.

## AQUARIUM live NeXAS hook

For the currently verified AQUARIUM Steam build, YomiRelay:

1. detects the installed game through Steam,
2. validates the NeXAS-compatible executable fingerprint and exact supported SHA-256,
3. waits for `Aquarium.exe` under Steam Proton,
4. reads only executable pages inside the loaded PE image while resolving the known NeXAS instruction signature,
5. stops signature scanning after the build-specific runtime hook address is validated,
6. attaches Linux `perf_event_open` execute breakpoints to the game threads,
7. samples both EAX (the text pointer) and ESI (the current NeXAS render-object instance),
8. reads the first word of the ESI object and, when it points back into the loaded AQUARIUM image, uses that vtable/class pointer as the stable render-context key,
9. normalizes and coalesces text independently per stable render context,
10. selects the dialogue render context only after repeated sentence-like Japanese lines are observed,
11. drops other contexts such as standalone character-name widgets, autosave notices, settings, and menu text,
12. appends accepted dialogue directly to canonical Reader history.

The runtime signature scan in step 4 is an **attachment-time operation only**. It is not the old dialogue memory scanner and it does not poll game text. Once the instruction address resolves, dialogue capture is event-driven through the execution breakpoint.

The render-context selector does not blacklist literal strings such as `設定` or `オートセーブ`. It groups changing ESI object instances by their stable in-module vtable/class pointer when possible, so recreation of the body-text object does not make later dialogue disappear. If the first object word cannot be read or is not an in-module class pointer, YomiRelay falls back to the raw ESI instance for that sample. A small number of candidate lines are buffered while the dialogue class is selected.

The AQUARIUM profile deliberately uses a build-specific instruction signature. For the verified Steam build, the expected sequence includes `mov eax,[esi+0xA4]`; only the relative `call` displacement is wildcarded. The verified Proton image exposes two copies of this short signature, so the profile validates and chooses the known dialogue hook RVA rather than guessing between them.

The hook does not patch `Aquarium.exe`, inject a DLL, modify game files, or continuously scan process memory for dialogue. Hardware breakpoint file descriptors are owned by the YomiRelay process and disappear when YomiRelay exits.

If the executable hash changes, the preferred runtime hook disappears, or kernel perf policy blocks the observer, YomiRelay fails closed and leaves the game untouched.

### Why the signature is resolved at runtime

An earlier YomiRelay build validated the NeXAS signature against the on-disk `Aquarium.exe`. That produced this false-negative state on the recognized Steam build:

```text
The game build was recognized, but its NeXAS hook signature did not validate.
```

That check was in the wrong layer. NeXAS text-hook signatures are process-memory signatures: the relevant instruction sequence should be searched after Wine/Proton has loaded the PE image. YomiRelay now accepts the verified build during discovery and resolves the hook address from the loaded executable image when AQUARIUM starts.

This does **not** weaken build safety. The exact executable SHA-256 allowlist is still checked before the native source starts, and the runtime matcher still validates the build-specific preferred hook site before any breakpoint is installed.

### Expected logs

Before the game starts:

```text
[nexas] AQUARIUM profile ready: sha256=... image-size=... preferred-rva=...
[nexas] waiting for AQUARIUM Steam/Proton process
```

After the process and runtime hook are available:

```text
[nexas] resolved runtime hook: game=AQUARIUM pid=... rva=... address=...
[nexas] attached execution hook: game=AQUARIUM pid=... address=...
```

After enough story text has been observed to identify the body-text renderer:

```text
[nexas] selected dialogue render context: pid=... class-vtable=0x...
```

If the object cannot be grouped by an in-module vtable, the diagnostic instead says `instance-esi=0x...`. The selector intentionally waits for repeated dialogue-like observations instead of treating the first Japanese string as story text.

### Linux permissions

The live hook relies on Linux `perf_event_open` and same-user process memory reads. YomiRelay requests a user-space-only perf breakpoint and does not change kernel security settings automatically.

Useful diagnostics:

```sh
cat /proc/sys/kernel/perf_event_paranoid
uname -a
```

If the runtime address resolves but breakpoint creation fails with `perf hook permission denied`, the signature itself worked; the remaining issue is local perf policy.

## Reader / Yomitan

Reader dialogue is ordinary DOM text with Japanese language metadata. YomiRelay does not integrate with Yomitan APIs; Yomitan works through its normal browser text scanning behavior.

Dialogue history is kept in memory and bounded per game. `Clear History` clears only the selected game's canonical history.

## Ren'Py hooks

Ren'Py hook installation writes only the managed YomiRelay `.rpy` hook inside the detected game's `game/` directory. Existing unmanaged game scripts are not overwritten. Restart a Ren'Py game after installing the hook.

NeXAS-specific render-context selection is not applied to Ren'Py dialogue. Ren'Py callback events continue directly through the UDP receiver into the canonical dialogue store.

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
3. confirm AQUARIUM is shown as `NeXAS` / `Live native hook (automatic)` rather than `Unavailable`
4. open YomiRelay Reader and select AQUARIUM
5. start AQUARIUM through Steam/Proton
6. confirm `[nexas] resolved runtime hook` appears
7. confirm `[nexas] attached execution hook` appears
8. advance at least two normal story lines and confirm `[nexas] selected dialogue render context` appears
9. when possible, confirm the selected diagnostic is `class-vtable=...` rather than `instance-esi=...`
10. advance Japanese story dialogue for at least 20 lines, including short lines and a change of speaker
11. confirm story lines arrive automatically and in event order even if NeXAS recreates its render object
12. confirm standalone character names are not emitted as separate dialogue entries
13. trigger autosave and confirm its notification is not emitted
14. open Settings and confirm its labels/help text are not emitted
15. confirm Yomitan can scan the rendered Japanese story text
16. stop YomiRelay while AQUARIUM remains open and confirm the game continues normally
17. start YomiRelay again and confirm context selection and capture resume

For a failed test, include every `[nexas]` log line and note whether the selected context says `class-vtable` or `instance-esi`.