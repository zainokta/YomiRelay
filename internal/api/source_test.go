package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"yomirelay/internal/aquarium"
	"yomirelay/internal/dialogue"
	"yomirelay/internal/games"
)

func TestNativeDiagnosticsStayOutOfDialogueHistory(t *testing.T) {
	api := newTestAPI(t)
	game := games.Game{AppID: aquarium.AppID, Engine: "nexas", SourceStatus: "experimental"}
	api.games.games[game.AppID] = game
	called := 0
	handler := New(Dependencies{Games: api.games, Store: api.store, Broker: api.broker, InspectSource: func(_ context.Context, g games.Game) (aquarium.Snapshot, error) {
		called++
		if g.AppID != game.AppID {
			t.Fatal("wrong app")
		}
		return aquarium.Snapshot{Status: "unverified", Candidates: []aquarium.Candidate{{Raw: "【トーレス】@n「日本語」"}}}, nil
	}})
	for _, tc := range []struct {
		method, id string
		want       int
	}{
		{"GET", "999", 404}, {"GET", "111", 501}, {"POST", aquarium.AppID, 405}, {"GET", aquarium.AppID, 200},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(tc.method, "/api/games/"+tc.id+"/source-debug", nil))
		if response.Code != tc.want {
			t.Fatalf("%s: %d %s", tc.id, response.Code, response.Body)
		}
		if tc.want == 200 && !strings.Contains(response.Body.String(), "unverified") {
			t.Fatal("candidate not labeled")
		}
	}
	if called != 1 || len(api.store.List(game.AppID)) != 0 {
		t.Fatal("diagnostics polluted history or ran for an unrelated app")
	}
}

func TestNativeHookAPIReportsUnavailable(t *testing.T) {
	api := newTestAPI(t)
	api.games.games[aquarium.AppID] = games.Game{AppID: aquarium.AppID, Engine: "nexas"}
	response := httptest.NewRecorder()
	api.handler.ServeHTTP(response, httptest.NewRequest("POST", "/api/games/"+aquarium.AppID+"/hook", nil))
	if response.Code != 501 || !strings.Contains(response.Body.String(), "SOURCE_UNAVAILABLE") {
		t.Fatalf("%d %s", response.Code, response.Body)
	}
}

func TestNativeCandidatePublishUsesReaderPipeline(t *testing.T) {
	api := newTestAPI(t)
	game := games.Game{AppID: aquarium.AppID, Name: "AQUARIUM", Engine: "nexas", SourceStatus: "experimental"}
	api.games.games[game.AppID] = game
	handler := New(Dependencies{Games: api.games, Store: api.store, Broker: api.broker})
	_, stream, cancel := api.broker.Subscribe()
	defer cancel()
	body := `{"raw":"【トーレス】@n@v20002「だって、たった一瞬とは言え、キミと愛し合えたのだから」"}`
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("POST", "/api/games/2515070/source-debug/publish", strings.NewReader(body)))
	if response.Code != 204 {
		t.Fatalf("response = %d %s", response.Code, response.Body)
	}
	items := api.store.List(game.AppID)
	if len(items) != 1 || items[0].Speaker != "トーレス" || items[0].Text != "だって、たった一瞬とは言え、キミと愛し合えたのだから" || items[0].Engine != "nexas" {
		t.Fatalf("history = %#v", items)
	}
	select {
	case got := <-stream:
		if got.GameID != game.AppID || got.GameName != game.Name || got.Engine != "nexas" {
			t.Fatalf("event = %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("published dialogue did not reach SSE broker")
	}
	var decoded dialogue.Dialogue
	if err := json.Unmarshal([]byte(`{"engine":"nexas","gameId":"2515070","gameName":"AQUARIUM","speaker":"トーレス","text":"日本語","timestamp":"2026-01-01T00:00:00Z"}`), &decoded); err != nil || decoded.Engine != "nexas" {
		t.Fatalf("dialogue JSON = %#v, %v", decoded, err)
	}
}

func TestNativeCandidatePublishRejectsMenuAndUnknownGames(t *testing.T) {
	api := newTestAPI(t)
	api.games.games[aquarium.AppID] = games.Game{AppID: aquarium.AppID, Name: "AQUARIUM", Engine: "nexas", SourceStatus: "experimental"}
	handler := New(Dependencies{Games: api.games, Store: api.store, Broker: api.broker})
	for _, tc := range []struct {
		id, body string
	}{
		{"999", `{"raw":"【トーレス】@n「台詞」"}`},
		{aquarium.AppID, `{"raw":"【選択肢】@n＞日本語@n　English@n"}`},
		{aquarium.AppID, `{"raw":"【トーレス】@n「"}`},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("POST", "/api/games/"+tc.id+"/source-debug/publish", strings.NewReader(tc.body)))
		if response.Code != 400 && tc.id == aquarium.AppID || response.Code != 404 && tc.id == "999" {
			t.Fatalf("%s: response = %d %s", tc.id, response.Code, response.Body)
		}
	}
	if len(api.store.List(aquarium.AppID)) != 0 {
		t.Fatal("invalid candidates entered history")
	}
}
