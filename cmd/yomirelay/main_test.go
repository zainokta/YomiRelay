package main

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yomirelay/internal/api"
	"yomirelay/internal/dialogue"
	"yomirelay/internal/events"
)

func TestConfigFromEnvUsesLoopbackDefaults(t *testing.T) {
	config, err := ConfigFromEnv(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if config.HTTPAddr != "127.0.0.1:17321" || config.UDPAddr != "127.0.0.1:17322" {
		t.Fatalf("config = %#v", config)
	}
}

func TestConfigFromEnvRejectsNonLoopbackAddresses(t *testing.T) {
	config, err := ConfigFromEnv(func(key string) string {
		if key == "YOMIRELAY_HTTP_ADDR" {
			return "0.0.0.0:17321"
		}
		return ""
	})
	if err == nil {
		t.Fatalf("config = %#v, want error", config)
	}
}

func TestRootHandlerKeepsAPISeparateFromSPA(t *testing.T) {
	apiHandler := api.New(api.Dependencies{Store: dialogue.NewStore(10, nil), Broker: events.NewBroker(1), Logger: log.New(io.Discard, "", 0)})
	static := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<title>YomiRelay</title>"))
	})
	handler := RootHandler(apiHandler, static)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"ok"`) {
		t.Fatalf("API response = %d %s", response.Code, response.Body)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/reader", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "YomiRelay") {
		t.Fatalf("SPA response = %d %s", response.Code, response.Body)
	}
}
