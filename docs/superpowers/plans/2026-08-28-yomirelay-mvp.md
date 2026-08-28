# YomiRelay MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build YomiRelay, a local Go executable and Octane web application that safely relays installed Ren'Py Steam-game dialogue into ordinary browser text over localhost UDP and SSE.

**Architecture:** A Go backend discovers Steam libraries, detects Ren'Py layouts, writes one ownership-marked hook per selected game, validates loopback UDP packets, stores bounded normalized per-game history, and publishes SSE. A pinned Octane/Vite SPA uses only relative API URLs, renders selected dialogue as normal light-DOM text, and is embedded into the production binary through `embed.FS`.

**Tech Stack:** Go standard library; `golang.org/x/sys/windows/registry` only in the Windows Steam locator; TypeScript; Octane `0.1.48`; `@octanejs/vite-plugin` `0.1.48`; Vite `8.2.2`; TypeScript `7.0.2`; npm.

## Global Constraints

- Bind HTTP to `127.0.0.1:17321` and UDP to `127.0.0.1:17322` by default; accept only `YOMIRELAY_HTTP_ADDR` and `YOMIRELAY_UDP_ADDR` as simple overrides.
- Never add OCR, clipboard scraping, process scanning/injection, Textractor, translation, dictionary logic, direct Yomitan APIs, persistence, remote access, or non-Ren'Py sources.
- The browser supplies app IDs only; no API accepts a filesystem path. Decimal app IDs must match the backend's discovered game registry.
- Steam discovery may enumerate only Steam roots, configured libraries, and `steamapps/appmanifest_*.acf`; do not recursively scan the disk.
- Detect Ren'Py from filesystem evidence, never a game name. Require a `game` directory plus corroborating runtime/archive/script evidence.
- A hook is exactly `<install>/game/_yomirelay_hook.rpy`, begins with both YomiRelay ownership-marker lines, and may overwrite or delete only a managed regular file. Refuse symbolic links.
- The hook uses documented `config.all_character_callbacks`, sends one non-blocking UTF-8 JSON UDP datagram to loopback per statement, and swallows every transmission failure.
- A valid packet has version `1`, non-empty `gameId`, `gameName`, and `text`, and a valid Unix-second timestamp. Malformed packets are logged and discarded.
- Keep at most 1000 dialogue entries per game. A game is active for 30 seconds after a valid packet. Slow or disconnected SSE clients must not block ingestion or other clients.
- Render dialogue as selectable ordinary light-DOM text with `lang="ja"`; do not use canvas, SVG text, images, character-per-span markup, or inaccessible shadow DOM.
- Require Node.js `>= 22.22.2`, pin frontend dependency versions, commit `frontend/package-lock.json`, and use documented Octane APIs only.
- Production `dist/yomirelay` must include built frontend assets through `embed.FS` and run without Node.js, npm, Vite, or Octane tooling.
- Every task starts with a focused failing test where the task contains non-trivial Go behavior, runs its focused test before and after implementation, runs `go test ./...` before commit, and commits one reviewable change.

---

## File ownership map

| Path | Responsibility |
| --- | --- |
| `cmd/yomirelay/main.go` | Parse environment configuration, initialize dependencies, run receiver and HTTP server, coordinate shutdown. |
| `internal/platform/*` | Isolated Steam root lookup for Linux, macOS, and Windows. |
| `internal/steam/*` | Limited VDF parsing and deterministic configured-library/manifest discovery. |
| `internal/games/*` | Ren'Py signal detection and concurrency-safe discovered-game registry. |
| `internal/hook/*` | Generated Ren'Py source plus safe ownership-checked installation/removal. |
| `internal/dialogue/*` | Normalized dialogue model, bounded per-game history, and activity state. |
| `internal/receiver/*` | Packet schema validation and loopback UDP listener. |
| `internal/events/*` | Bounded non-blocking dialogue subscriptions for SSE. |
| `internal/api/*` | JSON REST/SSE routes, consistent errors, and no-path-input security boundary. |
| `internal/web/*` | Embedded Vite asset filesystem and SPA static serving. |
| `frontend/src/api/*` | Typed relative REST client and shared EventSource abstraction. |
| `frontend/src/games/*` | Games list, hook actions, and reader navigation. |
| `frontend/src/reader/*` | Selected-game history, live event consumption, DOM dialogue entries, and scroll policy. |
| `frontend/src/styles/app.css` | Compact responsive dark-friendly presentation only. |
| `dev.sh`, `build.sh` | Prerequisite validation, dependency bootstrap, concurrent development lifecycle, and production build. |
| `README.md` | Setup, workflow, ports, safety model, limitations, and manual acceptance test. |

## Task 1: Bootstrap the executable and embedded static-asset boundary

**Files:**
- Create: `.gitignore`
- Create: `go.mod`
- Create: `cmd/yomirelay/main.go`
- Create: `internal/web/assets.go`
- Create: `internal/web/assets_test.go`
- Create: `internal/web/static/index.html`

**Interfaces:**
- Produces `web.Handler() http.Handler`, which serves `index.html` for non-API routes and embedded assets under `/assets/`.
- Produces `main` as the sole production executable entry point.
- Later tasks add API routing below `/api/` without changing static-asset behavior.

- [ ] **Step 1: Initialize the Go module and ignored build artefacts**

Create `go.mod` with the local module name and the single Windows-only dependency:

```go
module yomirelay

go 1.23.0

require golang.org/x/sys v0.35.0
```

Create `.gitignore` containing:

```gitignore
codedb.snapshot
dist/
frontend/node_modules/
frontend/dist/
internal/web/static/*
!internal/web/static/index.html
```

Run `go mod tidy` after the Windows locator is added in Task 3; preserve the declared `golang.org/x/sys v0.35.0` dependency unless a tested compatibility fix requires a deliberate version update. Do not create any runtime configuration file.

- [ ] **Step 2: Write a failing embedded-asset test**

Write `internal/web/assets_test.go` before `assets.go`:

```go
package web

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

func TestHandlerFallsBackToEmbeddedIndex(t *testing.T) {
    response := httptest.NewRecorder()
    Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/reader", nil))

    if response.Code != http.StatusOK {
        t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
    }
    if !strings.Contains(response.Body.String(), "YomiRelay") {
        t.Fatalf("fallback did not return the embedded index: %q", response.Body.String())
    }
}
```

Add the smallest committed placeholder `internal/web/static/index.html` containing `<title>YomiRelay</title>` and update `.gitignore` to retain that named file instead of `.gitkeep`; `go:embed` needs a real non-hidden file before the frontend build exists.

- [ ] **Step 3: Run the focused test and confirm it fails**

Run:

```bash
go test ./internal/web -run '^TestHandlerFallsBackToEmbeddedIndex$' -count=1
```

Expected: compilation failure because `Handler` is undefined.

- [ ] **Step 4: Implement the minimal embedded handler**

Create `internal/web/assets.go` using `//go:embed all:static`, `fs.Sub`, `http.FileServer`, and `fs.ReadFile`. For a request that maps to an embedded file, serve the file. For a path without an extension that does not map to a file, serve `static/index.html` with `Content-Type: text/html; charset=utf-8`. Return `404` rather than fallback for paths containing an extension that are absent. The complete public boundary is:

```go
package web

import "net/http"

func Handler() http.Handler
```

Create a minimal `cmd/yomirelay/main.go` that logs `YomiRelay bootstrap` and blocks only after later tasks add server startup; it must compile now without opening a port.

- [ ] **Step 5: Run focused and broad verification**

Run:

```bash
gofmt -w cmd/yomirelay/main.go internal/web
go test ./internal/web -run '^TestHandlerFallsBackToEmbeddedIndex$' -count=1
go test ./...
```

Expected: all tests pass.

- [ ] **Step 6: Commit the bootstrap boundary**

```bash
git add .gitignore go.mod cmd/yomirelay/main.go internal/web
git commit -m "build: bootstrap embedded web executable"
```

## Task 2: Parse Steam VDF and app manifests from fixtures

**Files:**
- Create: `internal/steam/vdf.go`
- Create: `internal/steam/vdf_test.go`
- Create: `internal/steam/testdata/libraryfolders.vdf`
- Create: `internal/steam/testdata/appmanifest_111.acf`
- Create: `internal/steam/testdata/appmanifest_222.acf`

**Interfaces:**
- Produces `type Value map[string]Value`, `func Parse(data []byte) (Value, error)`, and `func (Value) String(key string) (string, bool)`.
- Produces `func ParseManifest(data []byte) (Manifest, error)` where `Manifest` has `AppID`, `Name`, and `InstallDir` strings.
- Task 3 consumes parser output without relying on raw file layout.

- [ ] **Step 1: Write VDF and manifest fixtures**

Create `libraryfolders.vdf` with a quoted `libraryfolders` object that contains library `"0"` at `/home/test/.local/share/Steam` and library `"1"` at `/mnt/visual-novels`, with escaped backslashes in one unused label value. Create app manifests with the standard root `AppState`, numeric app IDs `111` and `222`, names including `Fake Ren'Py`, and install directories `FakeRenPy` and `NotRenPy`.

- [ ] **Step 2: Write the failing parser tests**

Write these tests in `internal/steam/vdf_test.go`:

```go
func TestParseLibraryFoldersWithNestedObjects(t *testing.T) {
    data := mustReadFixture(t, "libraryfolders.vdf")
    root, err := Parse(data)
    if err != nil { t.Fatal(err) }
    folders, ok := root.Object("libraryfolders")
    if !ok { t.Fatal("libraryfolders object missing") }
    first, ok := folders.Object("0")
    if !ok { t.Fatal("library 0 missing") }
    if got, _ := first.String("path"); got != "/home/test/.local/share/Steam" {
        t.Fatalf("path = %q", got)
    }
    second, _ := folders.Object("1")
    if got, _ := second.String("path"); got != "/mnt/visual-novels" {
        t.Fatalf("path = %q", got)
    }
}

func TestParseManifest(t *testing.T) {
    manifest, err := ParseManifest(mustReadFixture(t, "appmanifest_111.acf"))
    if err != nil { t.Fatal(err) }
    if manifest != (Manifest{AppID: "111", Name: "Fake Ren'Py", InstallDir: "FakeRenPy"}) {
        t.Fatalf("manifest = %#v", manifest)
    }
}

func TestParseRejectsUnclosedObject(t *testing.T) {
    if _, err := Parse([]byte("\"root\" { \"key\" \"value\"")); err == nil {
        t.Fatal("Parse accepted unclosed object")
    }
}
```

Include a `mustReadFixture` helper that calls `os.ReadFile(filepath.Join("testdata", name))` and `t.Helper()`.

- [ ] **Step 3: Run focused tests and confirm failure**

Run:

```bash
go test ./internal/steam -run '^(TestParseLibraryFoldersWithNestedObjects|TestParseManifest|TestParseRejectsUnclosedObject)$' -count=1
```

Expected: package compilation failure because parser symbols do not exist.

- [ ] **Step 4: Implement only the Steam KeyValues subset**

Implement a byte lexer that skips ASCII whitespace and `//` line comments, reads braces, and reads quoted tokens with `\"` and `\\` escapes. Parse a document as key/value pairs where a value is either a quoted scalar or a recursive `{ ... }` object. Reject unexpected EOF, an unmatched brace, a missing value, and an unsupported unquoted token with a descriptive error. Do not add generic KeyValues features beyond this grammar.

Use this exact data representation and helpers:

```go
type Value struct {
    Scalar string
    Object map[string]Value
}

func Parse(data []byte) (Value, error)
func (v Value) Object(key string) (Value, bool)
func (v Value) String(key string) (string, bool)

type Manifest struct { AppID, Name, InstallDir string }
func ParseManifest(data []byte) (Manifest, error)
```

`ParseManifest` requires all three string fields from `AppState`; it returns an error if any is absent or empty.

- [ ] **Step 5: Run focused and broad verification**

Run:

```bash
gofmt -w internal/steam
go test ./internal/steam -run '^(TestParseLibraryFoldersWithNestedObjects|TestParseManifest|TestParseRejectsUnclosedObject)$' -count=1
go test ./...
```

Expected: all tests pass.

- [ ] **Step 6: Commit VDF parsing**

```bash
git add internal/steam
git commit -m "feat: parse Steam libraries and manifests"
```

## Task 3: Discover configured Steam libraries and installed applications

**Files:**
- Create: `internal/platform/locator.go`
- Create: `internal/platform/steam_linux.go`
- Create: `internal/platform/steam_darwin.go`
- Create: `internal/platform/steam_windows.go`
- Create: `internal/steam/discovery.go`
- Create: `internal/steam/discovery_test.go`

**Interfaces:**
- Produces `platform.SteamLocator` with `FindSteamRoots() ([]string, error)`.
- Produces `steam.Discover(roots []string) ([]Installation, error)`.
- `Installation` has `AppID`, `Name`, and `InstallPath`; Task 4 takes these records as detection input.

- [ ] **Step 1: Write a temporary multi-library discovery test**

Write `TestDiscoverIncludesManifestsFromEveryLibrary` that creates this temporary tree:

```text
<root>/steamapps/libraryfolders.vdf
<root>/steamapps/appmanifest_111.acf
<root>/steamapps/common/FakeRenPy
<second>/steamapps/appmanifest_222.acf
<second>/steamapps/common/NotRenPy
```

Use `filepath.Join` for every path. Write a dynamic `libraryfolders.vdf` whose `path` entries are the two temporary absolute paths, replacing `\` with `\\` before quoting. Assert `Discover([]string{root})` returns exactly two installations, with `111` resolved under the root and `222` resolved under the second library. Add `TestDiscoverSkipsMalformedManifestAndKeepsOtherGames`, asserting one broken manifest does not erase the valid application.

- [ ] **Step 2: Run focused tests and confirm failure**

Run:

```bash
go test ./internal/steam -run '^TestDiscover' -count=1
```

Expected: compilation failure because `Discover` is undefined.

- [ ] **Step 3: Implement deterministic discovery**

Implement:

```go
type Installation struct {
    AppID       string
    Name        string
    InstallPath string
}

func Discover(roots []string) ([]Installation, error)
```

For each existing root, include the root itself as a library, parse only `<root>/steamapps/libraryfolders.vdf` if present, append valid numeric library entries, clean and deduplicate paths, and glob only `steamapps/appmanifest_*.acf` in each library. Parse each manifest and join `steamapps/common/<installdir>` using `filepath.Join`. Sort results by app ID and log malformed manifest errors through the caller-provided logger or a package-level `log.Printf`, but continue with valid manifests. A missing root, libraryfolders file, or manifest collection is not fatal.

Implement platform files with build tags. Each implementation returns existing, cleaned, deduplicated paths. Linux and macOS derive home with `os.UserHomeDir`; Windows reads `Software\\Valve\\Steam` values `SteamPath` then uses `C:\\Program Files (x86)\\Steam` fallback through `golang.org/x/sys/windows/registry`. `locator.go` exports only the shared interface and constructor, never `runtime.GOOS` switches.

- [ ] **Step 4: Run focused and cross-package verification**

Run:

```bash
gofmt -w internal/platform internal/steam
go test ./internal/steam -run '^TestDiscover' -count=1
go test ./...
GOOS=windows GOARCH=amd64 go build ./internal/platform ./internal/steam
GOOS=darwin GOARCH=arm64 go build ./internal/platform ./internal/steam
```

Expected: native tests pass; cross-platform commands compile the production packages without executing foreign test binaries.

- [ ] **Step 5: Commit discovery and locators**

```bash
git add go.mod go.sum internal/platform internal/steam
git commit -m "feat: discover Steam libraries across platforms"
```

## Task 4: Detect Ren'Py games and maintain the discovered-game registry

**Files:**
- Create: `internal/games/detect.go`
- Create: `internal/games/detect_test.go`
- Create: `internal/games/registry.go`
- Create: `internal/games/registry_test.go`

**Interfaces:**
- Produces `games.IsRenPy(installPath string) bool` and `games.Game` API records.
- Produces `(*games.Registry).Refresh() error`, `Get(appID string) (Game, bool)`, and `List() []Game`.
- Task 5 reads a game only through `Registry.Get`; Task 8 serializes `Game` records.

- [ ] **Step 1: Write failing detector and registry tests**

Write `TestIsRenPyRequiresGameDirectoryAndCorroboratingSignal`, creating four temporary installations: `game/` plus `renpy/` (true), `game/script.rpy` (true), `game/archive.rpa` (true), and a bare `game/` (false). Add a directory named `DefinitelyNotRenPy` with `game/` only to prove the name is ignored.

Write `TestRegistryRefreshListsOnlyDetectedGames` with an injected discovery function returning one true and one false `steam.Installation`. Assert `List()` contains only the true record with `Engine == "renpy"`, `HookInstalled == false`, and `Active == false`.

- [ ] **Step 2: Run focused tests and confirm failure**

Run:

```bash
go test ./internal/games -run '^(TestIsRenPy|TestRegistryRefresh)' -count=1
```

Expected: compilation failure because package symbols do not exist.

- [ ] **Step 3: Implement detection and registry seams**

Implement detection using `os.Stat` and `filepath.Glob` only within the install root:

```go
func IsRenPy(installPath string) bool

type Game struct {
    AppID         string     `json:"appId"`
    Name          string     `json:"name"`
    InstallPath   string     `json:"installPath"`
    Engine        string     `json:"engine"`
    HookInstalled bool       `json:"hookInstalled"`
    Active        bool       `json:"active"`
    LastSeen      *time.Time `json:"lastSeen,omitempty"`
}

type DiscoverFunc func() ([]steam.Installation, error)
func NewRegistry(discover DiscoverFunc, hooks HookStatusFunc, activity ActivityFunc) *Registry
```

Require `game` plus any one of `renpy` directory, `game/*.rpa`, `game/*.rpy`, `lib/py2-linux-x86_64/renpy`, `lib/py3-linux-x86_64/renpy`, `renpy.exe`, or `renpy.sh`. Inject hook/activity lookups to prevent cycles. `Refresh` replaces the map only after a successful discovery pass and detects hook status using the manager function. `List` returns a copy sorted by case-insensitive name then app ID.

- [ ] **Step 4: Run focused and broad verification**

Run:

```bash
gofmt -w internal/games
go test ./internal/games -run '^(TestIsRenPy|TestRegistryRefresh)' -count=1
go test ./...
```

Expected: all tests pass.

- [ ] **Step 5: Commit game detection**

```bash
git add internal/games
git commit -m "feat: detect installed RenPy games"
```

## Task 5: Generate and safely manage Ren'Py hooks

**Files:**
- Create: `internal/hook/template.go`
- Create: `internal/hook/manager.go`
- Create: `internal/hook/manager_test.go`

**Interfaces:**
- Produces `hook.Render(appID, gameName string) ([]byte, error)`.
- Produces `hook.Manager` methods `Installed(games.Game) bool`, `Install(games.Game) error`, and `Remove(games.Game) error`.
- Task 8 maps exported sentinel errors to `HOOK_FILE_CONFLICT`, `HOOK_NOT_MANAGED`, and `HOOK_PATH_UNSAFE` responses.

- [ ] **Step 1: Write failing template and manager tests**

Write `TestRenderIncludesOwnershipAndGameMetadata`, which renders ID `111` and Japanese name `影の物語`, checks both marker lines, `config.all_character_callbacks.append`, `127.0.0.1`, UDP port `17322`, and that the raw Japanese name is JSON-escaped correctly in the Python payload.

Write temporary-directory tests:

```go
func TestInstallCreatesAndUpdatesManagedHook(t *testing.T)
func TestInstallRefusesUnmanagedConflict(t *testing.T)
func TestRemoveDeletesOnlyManagedHook(t *testing.T)
func TestRemoveRefusesUnmanagedHook(t *testing.T)
func TestInstallRefusesGameDirectorySymlinkOutsideInstall(t *testing.T)
func TestInstallRefusesHookSymlink(t *testing.T)
```

For the managed update test, write an old marker-bearing file then assert `Install` replaces it with current content. For conflicts, write `init python: pass` with no marker and assert file bytes are unchanged. For the game-directory escape test, make `<install>/game` a symbolic link to a directory outside the temporary install tree; expect `ErrUnsafePath` and assert no hook file is created in that outside directory.

- [ ] **Step 2: Run focused tests and confirm failure**

Run:

```bash
go test ./internal/hook -run '^(TestRender|TestInstall|TestRemove)' -count=1
```

Expected: compilation failure because `Render` and `Manager` do not exist.

- [ ] **Step 3: Implement the Python 2/3-compatible template**

Implement `Render` using `encoding/json` to produce string literals for app ID and game name; do not interpolate unescaped values. The template must contain this behavior:

```python
init python:
    import json
    import socket
    import time

    def _yomirelay_callback(event, what=None, start=None, **kwargs):
        try:
            if event != "show" or start != 0:
                return
            # Clean recognized ASCII Ren'Py tags without removing ordinary text.
            text = _yomirelay_clean(what or "")
            if not text:
                return
            packet = {"v": 1, "gameId": YOMIRELAY_GAME_ID,
                      "gameName": YOMIRELAY_GAME_NAME, "text": text,
                      "timestamp": int(time.time())}
            if kwargs.get("who"):
                packet["speaker"] = kwargs["who"]
            sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
            sock.setblocking(0)
            encoded = json.dumps(packet, ensure_ascii=False)
            if isinstance(encoded, _yomirelay_text_type):
                encoded = encoded.encode("utf-8")
            sock.sendto(encoded, ("127.0.0.1", 17322))
            sock.close()
        except Exception:
            pass

    config.all_character_callbacks.append(_yomirelay_callback)
```

Emit the Go-JSON-quoted constants `YOMIRELAY_GAME_ID` and `YOMIRELAY_GAME_NAME` before the callback. Before serializing, define `_yomirelay_text_type` with `unicode` in Python 2 and `str` in Python 3 using a `try/except NameError` block, so JSON bytes are correct on both Ren'Py generations. Use a small character scan in `_yomirelay_clean`, not a broad regex: when `{` is followed by `/` or `[A-Za-z_]`, find the next `}` and remove it only if the tag body contains ASCII tag syntax characters. Otherwise copy the brace unchanged. Keep all ordinary Unicode runes and punctuation. The literal template itself must start with the two ownership markers.

- [ ] **Step 4: Implement ownership, path, and atomic-write rules**

Export:

```go
var (
    ErrFileConflict = errors.New("hook file exists but is not managed by YomiRelay")
    ErrNotManaged   = errors.New("hook file is not managed by YomiRelay")
    ErrUnsafePath   = errors.New("unsafe hook path")
)

type Manager struct{}
func (Manager) HookPath(game games.Game) (string, error)
func (Manager) Installed(game games.Game) bool
func (Manager) Install(game games.Game) error
func (Manager) Remove(game games.Game) error
```

`HookPath` rejects a non-Ren'Py record, checks `game` is a real directory inside the cleaned installation root, and returns only `<install>/game/_yomirelay_hook.rpy`. For an existing target, use `os.Lstat` and reject non-regular files. The two ownership lines must both be present before update or removal. Use `os.CreateTemp(targetDir, ".yomirelay-*")`, `Chmod(0644)`, `Write`, `Sync`, `Close`, then `Rename`; remove the temporary file on every error path. `Installed` returns false on any missing, unowned, or unsafe target.

- [ ] **Step 5: Run focused and broad verification**

Run:

```bash
gofmt -w internal/hook
go test ./internal/hook -run '^(TestRender|TestInstall|TestRemove)' -count=1
go test ./...
```

Expected: all tests pass.

- [ ] **Step 6: Commit safe hook management**

```bash
git add internal/hook
git commit -m "feat: safely install RenPy dialogue hook"
```

## Task 6: Validate normalized dialogue and retain bounded per-game history

**Files:**
- Create: `internal/dialogue/store.go`
- Create: `internal/dialogue/store_test.go`
- Create: `internal/receiver/packet.go`
- Create: `internal/receiver/packet_test.go`

**Interfaces:**
- Produces `dialogue.Dialogue`, `dialogue.Store`, and `receiver.ParsePacket`.
- `receiver.ParsePacket` validates UDP bytes before Task 7's listener publishes them.
- Task 8 calls `Store.List`, `Store.Clear`, and `Store.Activity`.

- [ ] **Step 1: Write failing packet and store tests**

Write `TestParsePacketPreservesJapaneseUTF8` with bytes encoding:

```json
{"v":1,"gameId":"111","gameName":"影の物語","speaker":"林黙","text":"今日はどうする？","timestamp":1787895000}
```

Assert all normalized fields and `Timestamp.Equal(time.Unix(1787895000, 0))`. Add table-driven malformed JSON, version `2`, missing `text`, missing `gameName`, zero timestamp, and invalid UTF-8 cases; each must return an error.

Write `TestStoreBoundsHistoryPerGame` that inserts 1001 records for `111` and one for `222`, then asserts game `111` retains entries 2 through 1001 and game `222` remains exactly one record. Write `TestStoreClearDoesNotAffectOtherGame` and `TestStoreActivityExpiresAfterThirtySeconds` using injected `now func() time.Time`.

- [ ] **Step 2: Run focused tests and confirm failure**

Run:

```bash
go test ./internal/dialogue ./internal/receiver -run '^(TestParsePacket|TestStore)' -count=1
```

Expected: compilation failure because packages and symbols do not exist.

- [ ] **Step 3: Implement packet validation**

Implement:

```go
type packet struct {
    Version   int64  `json:"v"`
    GameID    string `json:"gameId"`
    GameName  string `json:"gameName"`
    Speaker   string `json:"speaker"`
    Text      string `json:"text"`
    Timestamp int64  `json:"timestamp"`
}

func ParsePacket(data []byte) (dialogue.Dialogue, error)
```

Reject `!utf8.Valid(data)`, packets exceeding `MaxDatagramSize` (defined as `8192`), a JSON decoder error, trailing JSON, version other than one, any required trimmed string empty, timestamp less than one. Use `json.Decoder` with `DisallowUnknownFields` only if the generated hook and documented future-source seam are not harmed; the chosen behavior must be covered by a test. Preserve text and speaker exactly rather than trimming Japanese dialogue.

- [ ] **Step 4: Implement the store**

Implement the public model and concurrency boundary:

```go
type Dialogue struct {
    GameID string `json:"gameId"`
    GameName string `json:"gameName"`
    Speaker string `json:"speaker,omitempty"`
    Text string `json:"text"`
    Timestamp time.Time `json:"timestamp"`
}

type Store struct { /* mutex, entries, lastSeen, now */ }
func NewStore(limit int, now func() time.Time) *Store
func (s *Store) Append(d Dialogue)
func (s *Store) List(gameID string) []Dialogue
func (s *Store) Clear(gameID string)
func (s *Store) Activity(gameID string) (lastSeen time.Time, active bool, known bool)
```

Copy slices before returning. Append updates last-seen using injected current time, retains only the newest `limit` records for that game, and never mutates a caller-owned dialogue value.

- [ ] **Step 5: Run focused and broad verification**

Run:

```bash
gofmt -w internal/dialogue internal/receiver
go test ./internal/dialogue ./internal/receiver -run '^(TestParsePacket|TestStore)' -count=1
go test ./...
```

Expected: all tests pass.

- [ ] **Step 6: Commit dialogue normalization**

```bash
git add internal/dialogue internal/receiver/packet.go internal/receiver/packet_test.go
git commit -m "feat: validate and retain bounded dialogue history"
```

## Task 7: Broadcast dialogue without blocking UDP reception

**Files:**
- Create: `internal/events/broker.go`
- Create: `internal/events/broker_test.go`
- Create: `internal/receiver/udp.go`
- Create: `internal/receiver/udp_test.go`

**Interfaces:**
- Produces `events.Broker.Subscribe() (id uint64, events <-chan dialogue.Dialogue, cancel func())` and `Publish(dialogue.Dialogue)`.
- Produces `receiver.Listen(ctx, addr string, onDialogue func(dialogue.Dialogue)) error`.
- Task 8 subscribes SSE handlers and Task 9 starts `Listen` in its own goroutine.

- [ ] **Step 1: Write failing broker and UDP tests**

Write `TestBrokerFansOutInOrder` that subscribes two consumers, publishes one Japanese record, and receives an equal record from each with a one-second test timeout. Write `TestBrokerDropsFullSubscriberWithoutBlockingPublisher`: create a broker with queue size one, subscribe without reading, publish three records in a goroutine, and assert publication returns before 100 ms and the cancellation is idempotent.

Write `TestListenReceivesValidPacketAndIgnoresMalformedPacket` that starts a listener on `127.0.0.1:0`, sends malformed bytes followed by a valid packet through `net.DialUDP`, and waits to receive exactly one normalized dialogue. The listener constructor must expose its bound address to the test rather than hardcoding a port.

- [ ] **Step 2: Run focused tests and confirm failure**

Run:

```bash
go test ./internal/events ./internal/receiver -run '^(TestBroker|TestListen)' -count=1
```

Expected: compilation failure because broker and listener symbols do not exist.

- [ ] **Step 3: Implement bounded fanout**

Implement:

```go
type Broker struct { /* mutex, nextID, subscribers, queueSize */ }
func NewBroker(queueSize int) *Broker
func (b *Broker) Subscribe() (uint64, <-chan dialogue.Dialogue, func())
func (b *Broker) Publish(dialogue.Dialogue)
```

`Publish` copies the current subscriber map under a lock, then uses a non-blocking send to every channel. If a channel is full, remove it under the lock and close it exactly once. `cancel` uses `sync.Once` so handler defer and overflow cannot double-close. Never hold the broker mutex while sending.

- [ ] **Step 4: Implement the loopback-safe UDP listener**

Implement:

```go
type Listener struct { conn *net.UDPConn }
func Listen(ctx context.Context, addr string, onDialogue func(dialogue.Dialogue)) (*Listener, error)
func (l *Listener) Addr() net.Addr
func (l *Listener) Wait() error
func (l *Listener) Close() error
```

Parse `addr` with `net.ResolveUDPAddr`, reject addresses whose IP is not loopback, and call `net.ListenUDP`. In the read goroutine allocate `MaxDatagramSize+1` bytes, discard packets with the extra byte set, pass packet bytes to `ParsePacket`, log validation failures, and invoke `onDialogue` only for valid data. `Close` unblocks reads; `Wait` returns nil for normal context/close shutdown and returns a real socket error otherwise.

- [ ] **Step 5: Run focused and broad verification**

Run:

```bash
gofmt -w internal/events internal/receiver
go test ./internal/events ./internal/receiver -run '^(TestBroker|TestListen)' -count=1
go test ./...
```

Expected: all tests pass.

- [ ] **Step 6: Commit realtime backend primitives**

```bash
git add internal/events internal/receiver/udp.go internal/receiver/udp_test.go
git commit -m "feat: receive and broadcast dialogue events"
```

## Task 8: Expose safe REST, SSE, and health APIs

**Files:**
- Create: `internal/api/server.go`
- Create: `internal/api/server_test.go`

**Interfaces:**
- Produces `api.New(Dependencies) http.Handler` mounted below `/api/` by Task 9.
- Consumes registry, hook manager, dialogue store, and broker abstractions from Tasks 4 through 7.
- Produces JSON game, dialogue, health, error, and SSE responses consumed by Task 10.

- [ ] **Step 1: Write failing API tests**

Construct an `httptest` API with a fake registry containing app `111` and a real temporary hook manager/store/broker. Write tests for:

```go
func TestGamesReturnsDetectedGamesAndRefreshes(t *testing.T)
func TestInstallHookRejectsUnknownAndTraversalIDs(t *testing.T)
func TestHookConflictUsesConsistentJSONError(t *testing.T)
func TestDialoguesAreFilteredAndClearIsIsolated(t *testing.T)
func TestEventsStreamsDialogueToMultipleClients(t *testing.T)
func TestHealthReturnsOK(t *testing.T)
```

For the unknown/traversal test, send `/api/games/999/hook` and `/api/games/..%2Fetc%2Fpasswd/hook`; both must return 404 or 400 JSON envelopes and cannot cause a file creation. For conflict, pre-create an unmanaged hook and assert status `409`, code `HOOK_FILE_CONFLICT`, and unmodified file bytes. For SSE, open two `httptest` connections, publish `{GameID:"111", Text:"日本語"}`, and read a named `event: dialogue` block from each.

- [ ] **Step 2: Run focused tests and confirm failure**

Run:

```bash
go test ./internal/api -run '^(TestGames|TestInstallHook|TestHookConflict|TestDialogues|TestEvents|TestHealth)' -count=1
```

Expected: compilation failure because `api.New` is undefined.

- [ ] **Step 3: Implement routing and JSON errors**

Implement dependencies using narrow interfaces:

```go
type Games interface {
    Refresh() error
    Get(string) (games.Game, bool)
    List() []games.Game
}
type Hooks interface {
    Install(games.Game) error
    Remove(games.Game) error
}
type Dependencies struct {
    Games Games
    Hooks Hooks
    Store *dialogue.Store
    Broker *events.Broker
    Logger *log.Logger
}
func New(deps Dependencies) http.Handler
```

Add helpers:

```go
func writeJSON(w http.ResponseWriter, status int, v any)
func writeError(w http.ResponseWriter, status int, code, message string)
func validAppID(s string) bool
```

`validAppID` permits one or more ASCII decimal digits only. Match hook routes with `strings.TrimPrefix` and `strings.Split`, requiring exactly `games/{id}/hook`; reject encoded or extra segments. Return `405` with JSON error for wrong methods. `GET /api/games?refresh=1` calls `Refresh`; only registry records are returned. `GET /api/dialogues` requires a valid `gameId`; `DELETE` clears only that key. SSE sets `Content-Type: text/event-stream`, `Cache-Control: no-cache`, and `X-Accel-Buffering: no`, subscribes to the broker, serializes each record with `json.Marshal`, flushes after each event, and defers cancel when request context finishes. `/api/health` returns `{"status":"ok"}`.

Map `hook.ErrFileConflict` to `409 HOOK_FILE_CONFLICT`, `hook.ErrNotManaged` to `409 HOOK_NOT_MANAGED`, `hook.ErrUnsafePath` to `400 HOOK_PATH_UNSAFE`, missing game to `404 GAME_NOT_FOUND`, and unexpected errors to `500 INTERNAL_ERROR` while logging them.

- [ ] **Step 4: Run focused and broad verification**

Run:

```bash
gofmt -w internal/api
go test ./internal/api -run '^(TestGames|TestInstallHook|TestHookConflict|TestDialogues|TestEvents|TestHealth)' -count=1
go test ./...
```

Expected: all tests pass.

- [ ] **Step 5: Commit the HTTP API**

```bash
git add internal/api
git commit -m "feat: expose local game and dialogue API"
```

## Task 9: Assemble and run the local production server

**Files:**
- Modify: `cmd/yomirelay/main.go`
- Create: `cmd/yomirelay/main_test.go`
- Modify: `internal/web/assets.go`

**Interfaces:**
- Produces `Run(ctx context.Context, cfg Config, logger *log.Logger) error` for testable application startup.
- Mounts API below `/api/` and static fallback elsewhere.
- Starts the Task 7 receiver and publishes each valid event to both Task 6 store and Task 7 broker.

- [ ] **Step 1: Write the failing configuration and routing tests**

Write `TestConfigFromEnvUsesLoopbackDefaults` with `t.Setenv` asserting HTTP `127.0.0.1:17321` and UDP `127.0.0.1:17322`. Add `TestConfigFromEnvRejectsNonLoopbackAddresses`, setting `YOMIRELAY_HTTP_ADDR=0.0.0.0:17321` and expecting an error. Add `TestRootHandlerKeepsAPISeparateFromSPA`, building an application router with fake dependencies and asserting `/api/health` returns JSON while `/reader` returns the embedded index.

- [ ] **Step 2: Run focused tests and confirm failure**

Run:

```bash
go test ./cmd/yomirelay -run '^(TestConfigFromEnv|TestRootHandler)' -count=1
```

Expected: compilation failure because config and root-router symbols do not exist.

- [ ] **Step 3: Implement composition and graceful shutdown**

Use:

```go
type Config struct { HTTPAddr, UDPAddr string }
func ConfigFromEnv(getenv func(string) string) (Config, error)
func RootHandler(apiHandler, staticHandler http.Handler) http.Handler
func Run(ctx context.Context, cfg Config, logger *log.Logger) error
```

`ConfigFromEnv` substitutes defaults for empty values, parses each `host:port`, and rejects a host that is not `localhost`, `127.0.0.1`, or IPv6 loopback. `Run` creates the platform locator, discovers Steam roots, logs roots/libraries/game counts, creates registry/hook/store/broker/API, begins UDP listening, and runs `http.Server` on the HTTP address. The UDP callback calls `store.Append` then `broker.Publish`; it does not log dialogue text. On context cancellation, call `Shutdown` with a five-second timeout and close the UDP listener. Return server failures other than `http.ErrServerClosed` and listener failures. In `main`, derive a `signal.NotifyContext` for `SIGINT` and `SIGTERM`, use `log.New(os.Stderr, "[backend] ", log.LstdFlags)`, and call `log.Fatal` only for a returned startup/runtime error.

`RootHandler` dispatches `/api/` to the API handler and all other requests to `web.Handler()`.

- [ ] **Step 4: Run focused, broad, and race verification**

Run:

```bash
gofmt -w cmd/yomirelay internal/web
go test ./cmd/yomirelay -run '^(TestConfigFromEnv|TestRootHandler)' -count=1
go test ./...
go test -race ./internal/dialogue ./internal/events ./internal/api
```

Expected: all tests and the selected race checks pass.

- [ ] **Step 5: Commit executable assembly**

```bash
git add cmd/yomirelay internal/web/assets.go
git commit -m "feat: run local YomiRelay server"
```

## Task 10: Build the documented Octane client with one shared dialogue stream

**Files:**
- Create: `frontend/package.json`
- Create: `frontend/package-lock.json`
- Create: `frontend/tsconfig.json`
- Create: `frontend/vite.config.ts`
- Create: `frontend/index.html`
- Create: `frontend/src/main.ts`
- Create: `frontend/src/App.tsrx`
- Create: `frontend/src/api/client.ts`
- Create: `frontend/src/api/events.ts`
- Create: `frontend/src/games/GamesPage.tsrx`
- Create: `frontend/src/games/GameCard.tsrx`
- Create: `frontend/src/reader/ReaderPage.tsrx`
- Create: `frontend/src/reader/DialogueList.tsrx`
- Create: `frontend/src/reader/DialogueLine.tsrx`
- Create: `frontend/src/styles/app.css`
- Modify: `internal/web/static/index.html` through Vite output only

**Interfaces:**
- Consumes exactly the JSON API types and `dialogue` SSE event from Task 8.
- Produces a browser client that uses `fetch("/api/...")` and `new EventSource("/api/events")`, never a hardcoded backend origin.
- Vite outputs to `../internal/web/static` for Task 12 embedding.

- [ ] **Step 1: Verify and lock documented Octane/Vite setup**

Use the current official Octane quick-start and TSRX documentation already recorded in the design: import `createRoot` from `octane`, mount to `#app`, use `.tsrx` components, use `useState` and documented `useEffect`, and configure `octane()` from `@octanejs/vite-plugin`. Do not add React, JSX runtime packages, UI kits, or a router dependency.

Create `frontend/package.json` with exact versions and scripts:

```json
{
  "private": true,
  "engines": { "node": ">=22.22.2" },
  "scripts": { "dev": "vite", "build": "vite build", "typecheck": "tsc --noEmit" },
  "dependencies": { "octane": "0.1.48" },
  "devDependencies": {
    "@octanejs/vite-plugin": "0.1.48",
    "typescript": "7.0.2",
    "vite": "8.2.2"
  }
}
```

Generate the lockfile with:

```bash
cd frontend
npm install --package-lock-only --save-exact
```

- [ ] **Step 2: Implement typed relative API and stream modules**

In `client.ts`, define exact client types matching API JSON and functions `listGames(refresh = false)`, `installHook(appID)`, `removeHook(appID)`, `listDialogues(gameID)`, and `clearDialogues(gameID)`. All call an internal `request<T>(path, init?)` using relative paths, parse the standard error envelope, and throw `Error` with the backend message.

In `events.ts`, implement:

```ts
export type Unsubscribe = () => void;
export function createDialogueStream(): {
  subscribe(listener: (dialogue: Dialogue) => void): Unsubscribe;
  close(): void;
};
```

Construct one `EventSource("/api/events")` lazily on first subscriber, register `addEventListener("dialogue", ...)`, JSON-parse a `Dialogue`, fan out to a `Set` of listeners, and close/reset the EventSource when the final listener unsubscribes. Log stream errors with `console.warn`; do not create a connection per line component.

- [ ] **Step 3: Implement Octane entry and game section**

Create `main.ts` using the documented entry point:

```ts
import { createRoot } from "octane";
import { App } from "./App.tsrx";
import "./styles/app.css";

createRoot(document.getElementById("app")!).render(App, {});
```

`App.tsrx` owns `selectedGameId`, current section (`"games" | "reader"`), a single stream instance, and callbacks passed to child components. `GamesPage.tsrx` fetches game records on first effect, refreshes on button click, displays errors, and passes an `openReader(gameId)` callback. `GameCard.tsrx` shows name, app ID, install path, Ren'Py detection, hook state, activity, last-seen time, and buttons. After successful installation, set the exact notice `Restart the game to activate the hook.`

Use one list item/card per game; no canvas, SVG text, or external CSS framework.

- [ ] **Step 4: Implement the Reader with normal text nodes and scroll policy**

`ReaderPage.tsrx` receives `gameId` and stream, fetches that ID's history when selected, subscribes once through `useEffect`, and only appends live records whose `gameId` equals the selected ID. It exposes a selector, Clear History, an error/status region, and Jump to latest. `DialogueList.tsrx` owns the scroll container and calculates `nearBottom` with a 72-pixel threshold in its normal `scroll` handler. On each rendered update it calls `scrollTo({ top: scrollHeight, behavior: "auto" })` only if it was near the bottom; when not near the bottom it shows Jump to latest and never forces a scroll. `DialogueLine.tsrx` renders one speaker div only for a nonempty speaker and one exact text container:

```html
<div class="dialogue-text" lang="ja">{props.dialogue.text}</div>
```

Do not split `props.dialogue.text` into characters, spans, SVG nodes, or HTML injection. Keep it as a normal text interpolation.

- [ ] **Step 5: Configure Vite and build/type-check the client**

Create `vite.config.ts`:

```ts
import { defineConfig } from "vite";
import { octane } from "@octanejs/vite-plugin";

export default defineConfig({
  plugins: [octane()],
  server: { host: "127.0.0.1", proxy: { "/api": "http://127.0.0.1:17321" } },
  build: { outDir: "../internal/web/static", emptyOutDir: true }
});
```

Create `index.html` with only a normal `<div id="app"></div>` and module `<script src="/src/main.ts"></script>`. Build the responsive CSS with readable system font stacks, dark colors via `prefers-color-scheme`, a high-contrast selected-game heading, a scrollable Reader, visible controls, focus indicators, and `user-select: text` on dialogue.

Run:

```bash
cd frontend
npm ci
npm run typecheck
npm run build
```

Expected: the Octane compiler/type checker succeeds and `../internal/web/static/index.html` plus hashed assets exist.

- [ ] **Step 6: Verify generated UI invariants and commit**

Run:

```bash
cd ..
grep -R '127.0.0.1:17321' frontend/src && exit 1 || true
grep -R 'canvas\|<svg\|innerHTML' frontend/src && exit 1 || true
grep -R 'lang="ja"' frontend/src/reader/DialogueLine.tsrx
git restore --source=HEAD -- internal/web/static/index.html
git clean -fdX internal/web/static
git add frontend
```

Do not commit Vite output; `build.sh` regenerates it immediately before the binary is built. Then commit the frontend source and lockfile:

```bash
git commit -m "feat: add Octane games and reader UI"
```

## Task 11: Add robust development/build scripts and practical README

**Files:**
- Create: `dev.sh`
- Create: `build.sh`
- Create: `README.md`
- Modify: `.gitignore`

**Interfaces:**
- `./dev.sh` starts the backend on port 17321 and Vite on port 5173 in one terminal, then removes both child processes on signal or failure.
- `./build.sh` writes a tested `dist/yomirelay` binary containing static assets.
- README supplies every setup and manual acceptance instruction needed by a user.

- [ ] **Step 1: Write shell smoke tests before scripts**

Create a temporary test block run directly in the shell while scripts are absent:

```bash
bash -n dev.sh
bash -n build.sh
```

Expected: failure because both script files do not exist. Record this failure in the commit message body or task log; no test framework is needed for thin developer scripts.

- [ ] **Step 2: Implement common prerequisite behavior in each script**

Both scripts start exactly:

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
```

Each checks `command -v go`, `node`, and `npm`, exits with `Error: <command> is required.` when absent, extracts `node --version` after removing the leading `v`, and compares dot-separated numeric components against `22.22.2`. For an insufficient version print:

```text
Error: Node.js >= 22.22.2 is required.
Installed: <actual-version>
```

Each changes to `$ROOT_DIR/frontend` and runs `npm ci` only if `node_modules` is absent.

- [ ] **Step 3: Implement concurrent `dev.sh` lifecycle**

Start the backend and Vite as direct child processes so cleanup targets their actual process IDs; retain distinguishable output by printing startup labels and by keeping the backend logger's `[backend]` prefix:

```bash
printf '[backend] starting Go server\n'
( cd "$ROOT_DIR" && exec go run ./cmd/yomirelay ) &
BACKEND_PID=$!
printf '[frontend] starting Vite server\n'
( cd "$ROOT_DIR/frontend" && exec npm run dev ) &
FRONTEND_PID=$!
```

Implement an idempotent `cleanup` that sends `TERM` to nonempty live PIDs, waits for both, and ignores already-exited children. Install `trap cleanup EXIT INT TERM`. Use `wait -n "$BACKEND_PID" "$FRONTEND_PID"`, retain its status without violating `set -e`, call cleanup, and exit with the failing status. Print `YomiRelay development processes started.` before wait. Ensure an error before one PID is assigned still runs cleanup safely.

- [ ] **Step 4: Implement `build.sh` and README**

`build.sh` runs frontend `npm run build`, returns to root, runs `go test ./...`, creates `dist`, executes `go build -o dist/yomirelay ./cmd/yomirelay`, and prints `Built: $ROOT_DIR/dist/yomirelay`.

Write a concise README beginning with the required YomiRelay summary, requirements, `./dev.sh`, `./build.sh`, `./dist/yomirelay`, and `http://127.0.0.1:17321`. Document Steam-root and library manifest discovery, filesystem-based Ren'Py detection, marker-protected hook install/removal, restart requirement, HTTP/UDP ports, ordinary Yomitan browser scanning, Linux/Windows/macOS support, no persistence, and MVP non-goals. Include all 26 supplied manual acceptance steps verbatim enough to be executable, including scroll preservation and game responsiveness after stopping YomiRelay.

- [ ] **Step 5: Run script, build, test, and binary smoke verification**

Run:

```bash
chmod +x dev.sh build.sh
bash -n dev.sh
bash -n build.sh
./build.sh
./dist/yomirelay > /tmp/yomirelay-smoke.log 2>&1 &
APP_PID=$!
trap 'kill "$APP_PID" 2>/dev/null || true' EXIT
for _ in $(seq 1 20); do
  curl --fail --silent http://127.0.0.1:17321/api/health && break
  sleep 0.25
done
kill "$APP_PID"
wait "$APP_PID" 2>/dev/null || true
```

Expected: build completes, all Go tests pass, health returns `{"status":"ok"}`, and the binary starts without Node/npm present in its execution path.

- [ ] **Step 6: Commit tooling and documentation**

```bash
git add .gitignore dev.sh build.sh README.md
git commit -m "docs: add YomiRelay workflow and acceptance guide"
```

## Task 12: Perform final integration, cross-platform compile, and safety review

**Files:**
- Modify only files whose validation exposes an actual defect.
- Do not add scope, source types, or user-facing features during this task.

**Interfaces:**
- Validates the complete MVP assembled by Tasks 1 through 11.
- Produces validation evidence for the final completion report.

- [ ] **Step 1: Run the complete test and formatting suite**

Run:

```bash
gofmt -w cmd internal
go test ./...
go test -race ./internal/dialogue ./internal/events ./internal/api
cd frontend && npm ci && npm run typecheck && npm run build
```

Expected: all commands exit zero. If `gofmt` changes tracked files, rerun the full Go suite before proceeding.

- [ ] **Step 2: Verify production embedding and API behavior**

Run:

```bash
cd ..
./build.sh
./dist/yomirelay > /tmp/yomirelay-final.log 2>&1 &
APP_PID=$!
trap 'kill "$APP_PID" 2>/dev/null || true' EXIT
sleep 1
curl --fail --silent http://127.0.0.1:17321/api/health
curl --fail --silent http://127.0.0.1:17321/ | grep -q '<div id="app"></div>'
curl --silent --output /dev/null --write-out '%{http_code}\n' -X POST http://127.0.0.1:17321/api/games/..%2Fetc%2Fpasswd/hook | grep -Eq '400|404'
kill "$APP_PID"
wait "$APP_PID" 2>/dev/null || true
```

Expected: health is 200, embedded root page exists, traversal-shaped app ID is rejected, and no child process remains.

- [ ] **Step 3: Cross-compile platform-specific Steam code**

Run:

```bash
GOOS=windows GOARCH=amd64 go build ./...
GOOS=darwin GOARCH=arm64 go build ./...
GOOS=linux GOARCH=amd64 go build ./...
```

Expected: each target compiles all production packages without attempting to execute a foreign-platform test binary.

- [ ] **Step 4: Review changed scope and commit verified fixes only**

Run:

```bash
git status --short
git diff --check
git log --oneline --max-count=12
```

Confirm no generated `node_modules`, arbitrary filesystem endpoint, non-loopback default, OCR/process/clipboard code, or direct Yomitan integration is tracked. If validation changed source, commit that focused repair with a message naming the corrected behavior. Otherwise do not create an empty commit.

## Plan self-review

### Specification coverage

- Steam root discovery, configured library parsing, app manifest enumeration, and fixtures: Tasks 2 and 3.
- Filesystem-only Ren'Py detection and display registry: Task 4.
- Ownership-marked hook template, documented callback use, safe updates/removal, and no-gameplay-interference transport: Task 5.
- UDP schema validation, UTF-8, bounded isolated history, 30-second activity: Task 6.
- Loopback-only listening and nonblocking SSE fanout: Task 7.
- Required REST/SSE/health endpoints, path safety, and error envelope: Task 8.
- Default loopback server configuration, lifecycle, embedding, and diagnostics: Task 9.
- Pinned documented Octane app, normal Japanese DOM text, selected-game filtering, and scroll policy: Task 10.
- One-terminal development, production build, README, and manual acceptance: Task 11.
- Whole-system tests, binary smoke, cross-platform compilation, and scope review: Task 12.

### Type consistency

- Steam discovery creates `steam.Installation`; `games.Registry` consumes it through an injected `DiscoverFunc`.
- Hook manager accepts `games.Game`; API resolves that record before calling it.
- Receiver creates `dialogue.Dialogue`; the composition root appends it to `dialogue.Store` then publishes it through `events.Broker`.
- API and frontend both serialize/consume `Dialogue` JSON using `gameId`, `gameName`, optional `speaker`, `text`, and RFC3339 timestamp output.
- Frontend is the sole owner of EventSource lifecycle; line components receive an already-normalized `Dialogue` only.

### Scope and placeholder review

The plan creates only the MVP files listed in the approved design and contains no additional dialogue source, persistence layer, remote protocol, browser-extension API, or dictionary behavior. Every implementation task identifies its files, interfaces, failing-test command, minimum implementation boundary, verification command, and commit command.
