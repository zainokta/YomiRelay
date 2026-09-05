package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"yomirelay/internal/games"
	"yomirelay/internal/source/aquarium"
)

func TestNativePreviewStaysOutOfDialogueHistory(t *testing.T) {
	api := newTestAPI(t)
	game := games.Game{AppID: aquarium.AppID, Name: "AQUARIUM", Engine: "nexas", SourceStatus: "experimental"}
	api.games.games[game.AppID] = game
	api.games.games["111"] = games.Game{AppID: "111", Name: "Other", Engine: "renpy", SourceStatus: "available"}
	called := 0
	handler := New(Dependencies{
		Games:  api.games,
		Store:  api.store,
		Broker: api.broker,
		InspectSource: func(_ context.Context, g games.Game) (aquarium.Snapshot, error) {
			called++
			if g.AppID != game.AppID {
				t.Fatal("wrong app")
			}
			return aquarium.Snapshot{
				Status:  "unverified",
				Message: "Memory candidates may include backlog copies.",
				Candidates: []aquarium.Candidate{
					{Address: "0x10", Raw: "【トーレス】@n「日本語」"},
					{Address: "0x20", Raw: "【選択肢】@n＞日本語@n　English@n"},
				},
			}, nil
		},
	})

	for _, tc := range []struct {
		method string
		id     string
		want   int
	}{
		{method: "GET", id: "999", want: 404},
		{method: "GET", id: "111", want: 501},
		{method: "POST", id: aquarium.AppID, want: 405},
		{method: "GET", id: aquarium.AppID, want: 200},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(tc.method, "/api/games/"+tc.id+"/source-preview", nil))
		if response.Code != tc.want {
			t.Fatalf("%s %s: %d %s", tc.method, tc.id, response.Code, response.Body)
		}
		if tc.want == 200 {
			var preview aquarium.Preview
			if err := json.Unmarshal(response.Body.Bytes(), &preview); err != nil {
				t.Fatal(err)
			}
			if len(preview.Candidates) != 1 || preview.Candidates[0].Speaker != "トーレス" || preview.Candidates[0].Text != "日本語" || preview.Candidates[0].Address != "0x10" {
				t.Fatalf("preview = %#v", preview)
			}
			if strings.Contains(response.Body.String(), "\"raw\"") {
				t.Fatal("raw memory candidate leaked into Reader preview API")
			}
		}
	}
	if called != 1 || len(api.store.List(game.AppID)) != 0 {
		t.Fatal("native preview polluted history or ran for an unrelated app")
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("POST", "/api/games/"+aquarium.AppID+"/source-debug/publish", strings.NewReader(`{"raw":"【トーレス】@n「日本語」"}`)))
	if response.Code != 404 || len(api.store.List(game.AppID)) != 0 {
		t.Fatalf("legacy publish route = %d, history = %#v", response.Code, api.store.List(game.AppID))
	}
}

func TestNativePreviewReportsUnavailableWithoutHelper(t *testing.T) {
	api := newTestAPI(t)
	api.games.games[aquarium.AppID] = games.Game{AppID: aquarium.AppID, Engine: "nexas", SourceStatus: "experimental"}
	handler := New(Dependencies{Games: api.games, Store: api.store, Broker: api.broker})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/api/games/"+aquarium.AppID+"/source-preview", nil))
	if response.Code != 503 || !strings.Contains(response.Body.String(), "NATIVE_PREVIEW_UNAVAILABLE") {
		t.Fatalf("response = %d %s", response.Code, response.Body)
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
