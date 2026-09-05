package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"yomirelay/internal/dialogue"
	"yomirelay/internal/events"
	"yomirelay/internal/games"
	"yomirelay/internal/hook"
	"yomirelay/internal/translation"
)

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
	Games      Games
	Hooks      Hooks
	Store      *dialogue.Store
	Broker     *events.Broker
	Translator translation.TranslateFunc
	Logger     *log.Logger
}

type server struct {
	deps Dependencies
}

func New(deps Dependencies) http.Handler {
	if deps.Logger == nil {
		deps.Logger = log.Default()
	}
	return server{deps: deps}
}

func (s server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api")
	switch path {
	case "/health":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method is not allowed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case "/games":
		s.games(w, r)
	case "/dialogues":
		s.dialogues(w, r)
	case "/events":
		s.events(w, r)
	case "/translate":
		s.translate(w, r)
	default:
		s.hook(w, r, path)
	}
}

const maxTranslationRequestBytes = 16 * 1024

type translateRequest struct {
	GameID string `json:"gameId"`
	Text   string `json:"text"`
}

func (s server) translate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method is not allowed")
		return
	}
	if s.deps.Translator == nil {
		writeError(w, http.StatusServiceUnavailable, "TRANSLATION_UNAVAILABLE", "translation is unavailable")
		return
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxTranslationRequestBytes+1))
	decoder.DisallowUnknownFields()
	var input translateRequest
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_TRANSLATION_REQUEST", "request body is invalid")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, http.StatusBadRequest, "INVALID_TRANSLATION_REQUEST", "request body contains trailing data")
		return
	}
	if !validAppID(input.GameID) {
		writeError(w, http.StatusBadRequest, "INVALID_GAME_ID", "gameId must contain only decimal digits")
		return
	}
	if _, ok := s.deps.Games.Get(input.GameID); !ok {
		writeError(w, http.StatusNotFound, "GAME_NOT_FOUND", "game was not found")
		return
	}
	if strings.TrimSpace(input.Text) == "" || len([]byte(input.Text)) > translation.MaxTextBytes {
		writeError(w, http.StatusBadRequest, "INVALID_TRANSLATION_REQUEST", "text is empty or too large")
		return
	}
	result, err := s.deps.Translator(r.Context(), input.GameID, input.Text)
	if err != nil {
		s.deps.Logger.Printf("api: translation unavailable: %v", err)
		writeError(w, http.StatusServiceUnavailable, "TRANSLATION_UNAVAILABLE", "translation is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s server) games(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method is not allowed")
		return
	}
	if r.URL.Query().Get("refresh") == "1" {
		if err := s.deps.Games.Refresh(); err != nil {
			s.internalError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, s.deps.Games.List())
}

func (s server) hook(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) != 3 || parts[0] != "games" || parts[2] != "hook" || !validAppID(parts[1]) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "route was not found")
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method is not allowed")
		return
	}
	game, ok := s.deps.Games.Get(parts[1])
	if !ok {
		writeError(w, http.StatusNotFound, "GAME_NOT_FOUND", "game was not found")
		return
	}
	var err error
	if r.Method == http.MethodPost {
		err = s.deps.Hooks.Install(game)
	} else {
		err = s.deps.Hooks.Remove(game)
	}
	if err != nil {
		s.hookError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s server) dialogues(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method is not allowed")
		return
	}
	gameID := r.URL.Query().Get("gameId")
	if !validAppID(gameID) {
		writeError(w, http.StatusBadRequest, "INVALID_GAME_ID", "gameId must contain only decimal digits")
		return
	}
	if _, ok := s.deps.Games.Get(gameID); !ok {
		writeError(w, http.StatusNotFound, "GAME_NOT_FOUND", "game was not found")
		return
	}
	if r.Method == http.MethodDelete {
		s.deps.Store.Clear(gameID)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, s.deps.Store.List(gameID))
}

func (s server) events(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method is not allowed")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	_, stream, cancel := s.deps.Broker.Subscribe()
	defer cancel()
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "streaming is not supported")
		return
	}
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case value, ok := <-stream:
			if !ok {
				return
			}
			data, err := json.Marshal(value)
			if err != nil {
				s.internalError(w, err)
				return
			}
			_, _ = fmt.Fprintf(w, "event: dialogue\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s server) hookError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, hook.ErrSourceUnavailable):
		writeError(w, http.StatusNotImplemented, "SOURCE_UNAVAILABLE", err.Error())
	case errors.Is(err, hook.ErrFileConflict):
		writeError(w, http.StatusConflict, "HOOK_FILE_CONFLICT", err.Error())
	case errors.Is(err, hook.ErrNotManaged):
		writeError(w, http.StatusConflict, "HOOK_NOT_MANAGED", err.Error())
	case errors.Is(err, hook.ErrUnsafePath):
		writeError(w, http.StatusBadRequest, "HOOK_PATH_UNSAFE", err.Error())
	default:
		s.internalError(w, err)
	}
}

func (s server) internalError(w http.ResponseWriter, err error) {
	s.deps.Logger.Printf("api: %v", err)
	writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "an internal error occurred")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func validAppID(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
