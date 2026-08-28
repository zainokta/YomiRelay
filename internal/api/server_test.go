package api

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"yomirelay/internal/dialogue"
	"yomirelay/internal/events"
	"yomirelay/internal/games"
	"yomirelay/internal/hook"
)

type fakeGames struct {
	mu        sync.Mutex
	games     map[string]games.Game
	refreshes int
}

func (f *fakeGames) Refresh() error { f.mu.Lock(); f.refreshes++; f.mu.Unlock(); return nil }
func (f *fakeGames) Get(id string) (games.Game, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	game, ok := f.games[id]
	return game, ok
}
func (f *fakeGames) List() []games.Game {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]games.Game, 0, len(f.games))
	for _, game := range f.games {
		out = append(out, game)
	}
	return out
}

type testAPI struct {
	handler http.Handler
	game    games.Game
	store   *dialogue.Store
	broker  *events.Broker
}

func newTestAPI(t *testing.T) testAPI {
	t.Helper()
	install := t.TempDir()
	if err := os.MkdirAll(filepath.Join(install, "game"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(install, "renpy"), 0o755); err != nil {
		t.Fatal(err)
	}
	game := games.Game{AppID: "111", Name: "Game", InstallPath: install, Engine: "renpy"}
	fake := &fakeGames{games: map[string]games.Game{"111": game}}
	store := dialogue.NewStore(1000, time.Now)
	broker := events.NewBroker(4)
	return testAPI{handler: New(Dependencies{Games: fake, Hooks: hook.Manager{}, Store: store, Broker: broker, Logger: log.New(io.Discard, "", 0)}), game: game, store: store, broker: broker}
}

func TestGamesReturnsDetectedGamesAndRefreshes(t *testing.T) {
	api := newTestAPI(t)
	response := httptest.NewRecorder()
	api.handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/games?refresh=1", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"appId":"111"`) {
		t.Fatalf("response = %d %s", response.Code, response.Body)
	}
}

func TestInstallHookRejectsUnknownAndTraversalIDs(t *testing.T) {
	api := newTestAPI(t)
	for _, path := range []string{"/api/games/999/hook", "/api/games/..%2Fetc%2Fpasswd/hook"} {
		response := httptest.NewRecorder()
		api.handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, nil))
		if response.Code != http.StatusNotFound && response.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d", path, response.Code)
		}
		if !strings.Contains(response.Header().Get("Content-Type"), "application/json") {
			t.Fatalf("%s content type = %q", path, response.Header().Get("Content-Type"))
		}
	}
}

func TestHookConflictUsesConsistentJSONError(t *testing.T) {
	api := newTestAPI(t)
	path := filepath.Join(api.game.InstallPath, "game", "_yomirelay_hook.rpy")
	original := []byte("init python: pass")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	api.handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/games/111/hook", nil))
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"HOOK_FILE_CONFLICT"`) {
		t.Fatalf("response = %d %s", response.Code, response.Body)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(original) {
		t.Fatal("conflict file modified")
	}
}

func TestDialoguesAreFilteredAndClearIsIsolated(t *testing.T) {
	api := newTestAPI(t)
	api.store.Append(dialogue.Dialogue{GameID: "111", Text: "one"})
	api.store.Append(dialogue.Dialogue{GameID: "222", Text: "two"})
	response := httptest.NewRecorder()
	api.handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/dialogues?gameId=111", nil))
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "two") {
		t.Fatalf("response = %d %s", response.Code, response.Body)
	}
	response = httptest.NewRecorder()
	api.handler.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/dialogues?gameId=111", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("clear status = %d", response.Code)
	}
	if len(api.store.List("111")) != 0 || len(api.store.List("222")) != 1 {
		t.Fatal("clear was not isolated")
	}
}

func TestEventsStreamsDialogueToMultipleClients(t *testing.T) {
	api := newTestAPI(t)
	server := httptest.NewServer(api.handler)
	defer server.Close()
	clients := make([]*http.Response, 2)
	errs := make(chan error, 2)
	for i := range clients {
		go func(i int) {
			response, err := http.Get(server.URL + "/api/events")
			if err != nil {
				errs <- err
				return
			}
			clients[i] = response
			errs <- nil
		}(i)
	}
	for range clients {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		for _, response := range clients {
			response.Body.Close()
		}
	}()
	// Wait until both subscriptions have reached the broker by publishing after connection setup.
	api.broker.Publish(dialogue.Dialogue{GameID: "111", Text: "日本語"})
	for _, response := range clients {
		reader := bufio.NewReader(response.Body)
		var block strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			block.WriteString(line)
			if line == "\n" {
				break
			}
		}
		if !strings.Contains(block.String(), "event: dialogue") || !strings.Contains(block.String(), "日本語") {
			t.Fatalf("SSE block = %q", block.String())
		}
	}
}

func TestHealthReturnsOK(t *testing.T) {
	api := newTestAPI(t)
	response := httptest.NewRecorder()
	api.handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	var value map[string]string
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &value) != nil || value["status"] != "ok" {
		t.Fatalf("response = %d %s", response.Code, response.Body)
	}
}
