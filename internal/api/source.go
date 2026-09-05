package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"yomirelay/internal/aquarium"
	"yomirelay/internal/games"
)

type InspectSourceFunc func(context.Context, games.Game) (aquarium.Snapshot, error)

func (s server) sourceDebug(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "method is not allowed")
		return
	}
	game, ok := s.deps.Games.Get(id)
	if !ok {
		writeError(w, 404, "GAME_NOT_FOUND", "game was not found")
		return
	}
	if game.AppID != aquarium.AppID || game.Engine != "nexas" || game.SourceStatus != "experimental" {
		writeError(w, 501, "SOURCE_UNAVAILABLE", "native diagnostics are unavailable for this game or executable version")
		return
	}
	if s.deps.InspectSource == nil {
		writeError(w, 503, "NATIVE_DEBUG_DISABLED", "Restart YomiRelay with YOMIRELAY_NATIVE_DEBUG=1 to enable native diagnostics.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	w.Header().Set("Cache-Control", "no-store")
	result, err := s.deps.InspectSource(ctx, game)
	if err != nil {
		writeError(w, 503, "NATIVE_DEBUG_UNAVAILABLE", err.Error())
		return
	}
	s.deps.Logger.Printf("native source inspected: app=%s pid=%d status=%s candidates=%d", game.AppID, result.PID, result.Status, len(result.Candidates))
	writeJSON(w, 200, result)
}

type publishSourceRequest struct {
	Raw string `json:"raw"`
}

func (s server) publishSource(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method is not allowed")
		return
	}
	game, ok := s.deps.Games.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "GAME_NOT_FOUND", "game was not found")
		return
	}
	if game.AppID != aquarium.AppID || game.Engine != "nexas" || game.SourceStatus != "experimental" {
		writeError(w, http.StatusNotImplemented, "SOURCE_UNAVAILABLE", "native diagnostics are unavailable for this game or executable version")
		return
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 8192+1))
	decoder.DisallowUnknownFields()
	var input publishSourceRequest
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_SOURCE_CANDIDATE", "candidate body is invalid")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, http.StatusBadRequest, "INVALID_SOURCE_CANDIDATE", "candidate body contains trailing data")
		return
	}
	if strings.TrimSpace(input.Raw) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_SOURCE_CANDIDATE", "candidate is empty")
		return
	}
	value, err := aquarium.NormalizeCandidate(input.Raw, time.Now())
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_SOURCE_CANDIDATE", err.Error())
		return
	}
	value.GameID = game.AppID
	value.GameName = game.Name
	s.deps.Store.Append(value)
	s.deps.Broker.Publish(value)
	s.deps.Logger.Printf("native source candidate published: app=%s speaker=%s", game.AppID, value.Speaker)
	w.WriteHeader(http.StatusNoContent)
}
