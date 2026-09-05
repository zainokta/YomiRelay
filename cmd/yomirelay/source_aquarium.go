package main

import (
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"yomirelay/internal/games"
	"yomirelay/internal/source/aquarium"
)

// startNativeSources launches supported native source helpers. The AQUARIUM
// helper waits for the Proton process and uses a kernel-managed execution
// breakpoint; no game file or process memory is patched.
func startNativeSources(ctx context.Context, registry *games.Registry, udpAddress string, logger *log.Logger) {
	game, ok := registry.Get(aquarium.AppID)
	if !ok || game.SourceStatus != "native-auto" {
		return
	}
	helper := os.Getenv("YOMIRELAY_AQUARIUM_HELPER")
	if helper == "" {
		executable, err := os.Executable()
		if err != nil {
			logger.Printf("AQUARIUM native hook helper: resolve executable: %v", err)
			return
		}
		helper = filepath.Join(filepath.Dir(executable), "yomirelay-aquarium")
	}
	command := exec.CommandContext(ctx, helper,
		"-install", game.InstallPath,
		"-udp", udpAddress,
		"-game-name", game.Name,
	)
	writer := &nativeLogWriter{logger: logger}
	command.Stdout = writer
	command.Stderr = writer
	go func() {
		if err := command.Run(); err != nil && ctx.Err() == nil {
			logger.Printf("AQUARIUM native hook helper stopped: %v", err)
		}
	}()
}

type nativeLogWriter struct {
	logger *log.Logger
}

func (w *nativeLogWriter) Write(data []byte) (int, error) {
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			w.logger.Printf("[nexas] %s", line)
		}
	}
	return len(data), nil
}

var _ io.Writer = (*nativeLogWriter)(nil)
