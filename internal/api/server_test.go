package api

import (
	"bufio"
	"context"
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
	"yomirelay/internal/translation"
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
	games   *fakeGames
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
	return testAPI{handler: New(Dependencies{Games: fake, Hooks: hook.Manager{}, Store: store, Broker: broker, Logger: log.New(io.Discard, "", 0)}), game: game, games: fake, store: store, broker: broker}
}

func newTestAPIWithTranslator(t *testing.T, translator translation.TranslateFunc) testAPI {
	t.Helper()
	api := newTestAPI(t)
	api.handler = New(Dependencies{
		Games:      api.games,
		Hooks:      hook.Manager{},
		Store:      api.store,
		Broker:     api.broker,
		Translator: translator,
		Logger:     log.New(io.Discard, "", 0),
	})
	return api
}

func TestTranslateReturnsInjectedResult(t *testing.T) {
	api := newTestAPIWithTranslator(t, func(_ context.Context, gameID, text string) (translation.Result, error) {
		if gameID != "111" || text != "日本語。" {
			t.Fatalf("translator input = %q, %q", gameID, text)
		}
		return translation.Result{
			Translation: "Japanese.",
			Segments: []translation.Segment{
				{Text: "日本語", Kana: "にほんご", Meaning: "Japanese"},
				{Text: "。"},
			},
		}, nil
	})
	request := httptest.NewRequest(http.MethodPost, "/api/translate", strings.NewReader(`{"gameId":"111","text":"日本語。"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	api.handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"translation":"Japanese."`) {
		t.Fatalf("response = %d %s", response.Code, response.Body)
	}
}

func TestTranslateRejectsInvalidAndUnknownRequests(t *testing.T) {
	api := newTestAPIWithTranslator(t, func(context.Context, string, string) (translation.Result, error) {
		t.Fatal("translator should not be called")
		return translation.Result{}, nil
	})
	cases := []struct {
		body string
		want int
	}{
		{`{"gameId":"bad","text":"日本語。"}`, http.StatusBadRequest},
		{`{"gameId":"999","text":"日本語。"}`, http.StatusNotFound},
		{`{"gameId":"111","text":"   "}`, http.StatusBadRequest},
		{`{"gameId":"111","text":"` + strings.Repeat("日", translation.MaxTextBytes+1) + `"}`, http.StatusBadRequest},
	}
	for _, test := range cases {
		response := httptest.NewRecorder()
		api.handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/translate", strings.NewReader(test.body)))
		if response.Code != test.want {
			t.Errorf("body %q status = %d, want %d", test.body, response.Code, test.want)
		}
	}
}

func TestTranslateMapsUnavailableTo503(t *testing.T) {
	api := newTestAPIWithTranslator(t, func(context.Context, string, string) (translation.Result, error) {
		return translation.Result{}, translation.ErrUnavailable
	})
	response := httptest.NewRecorder()
	api.handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/translate", strings.NewReader(`{"gameId":"111","text":"日本語。"}`)))
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), `"code":"TRANSLATION_UNAVAILABLE"`) {
		t.Fatalf("response = %d %s", response.Code, response.Body)
	}
}

func TestTranslatePassesRequestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	api := newTestAPIWithTranslator(t, func(ctx context.Context, _ string, _ string) (translation.Result, error) {
		if ctx.Err() == nil {
			t.Fatal("translator context was not cancelled")
		}
		return translation.Result{}, ctx.Err()
	})
	request := httptest.NewRequest(http.MethodPost, "/api/translate", strings.NewReader(`{"gameId":"111","text":"日本語。"}`)).WithContext(ctx)
	cancel()
	response := httptest.NewRecorder()

	api.handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("response status = %d", response.Code)
	}
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

func TestDialoguesRejectUnknownDecimalGameIDs(t *testing.T) {
	api := newTestAPI(t)
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(method, "/api/dialogues?gameId=999", nil)
		api.handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"code":"GAME_NOT_FOUND"`) {
			t.Fatalf("%s response = %d %s", method, response.Code, response.Body)
		}
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
