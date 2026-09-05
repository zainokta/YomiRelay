package api

import (
	"context"
	"net/http"
	"time"

	"yomirelay/internal/games"
	"yomirelay/internal/source/aquarium"
)

type InspectSourceFunc func(context.Context, games.Game) (aquarium.Snapshot, error)

func (s server) sourcePreview(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method is not allowed")
		return
	}
	game, ok := s.deps.Games.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "GAME_NOT_FOUND", "game was not found")
		return
	}
	if game.AppID != aquarium.AppID || game.Engine != "nexas" || game.SourceStatus != "experimental" {
		writeError(w, http.StatusNotImplemented, "SOURCE_UNAVAILABLE", "native preview is unavailable for this game or executable version")
		return
	}
	if s.deps.InspectSource == nil {
		writeError(w, http.StatusServiceUnavailable, "NATIVE_PREVIEW_UNAVAILABLE", "native preview helper is unavailable")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	w.Header().Set("Cache-Control", "no-store")
	snapshot, err := s.deps.InspectSource(ctx, game)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "NATIVE_PREVIEW_UNAVAILABLE", err.Error())
		return
	}
	preview := aquarium.BuildPreview(snapshot)
	s.deps.Logger.Printf(
		"native source preview: app=%s pid=%d status=%s raw_candidates=%d preview_candidates=%d",
		game.AppID,
		snapshot.PID,
		snapshot.Status,
		len(snapshot.Candidates),
		len(preview.Candidates),
	)
	writeJSON(w, http.StatusOK, preview)
}
