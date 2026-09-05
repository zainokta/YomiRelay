package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"yomirelay/internal/api"
	"yomirelay/internal/games"
	"yomirelay/internal/source/aquarium"
)

func nativeInspector() api.InspectSourceFunc {
	// One on-demand read-only helper at a time prevents concurrent memory scans.
	busy := make(chan struct{}, 1)
	return func(ctx context.Context, game games.Game) (aquarium.Snapshot, error) {
		select {
		case busy <- struct{}{}:
			defer func() { <-busy }()
		default:
			return aquarium.Snapshot{}, fmt.Errorf("native inspection is already running")
		}

		helper := os.Getenv("YOMIRELAY_AQUARIUM_HELPER")
		if helper == "" {
			executable, err := os.Executable()
			if err != nil {
				return aquarium.Snapshot{}, err
			}
			helper = filepath.Join(filepath.Dir(executable), "yomirelay-aquarium")
		}

		command := exec.CommandContext(ctx, helper, "-install", game.InstallPath)
		var stderr strings.Builder
		command.Stderr = &stderr
		data, err := command.Output()
		if err != nil {
			if ctx.Err() != nil {
				return aquarium.Snapshot{}, fmt.Errorf("native inspection cancelled or timed out")
			}
			if stderr.Len() > 0 {
				return aquarium.Snapshot{}, fmt.Errorf("native inspection: %s", strings.TrimSpace(stderr.String()))
			}
			return aquarium.Snapshot{}, fmt.Errorf("native helper unavailable at %s: %w", helper, err)
		}

		var result aquarium.Snapshot
		if err := json.Unmarshal(data, &result); err != nil {
			return result, fmt.Errorf("invalid native helper response: %w", err)
		}
		return result, nil
	}
}
