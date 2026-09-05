//go:build linux

package nexas

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

func Start(ctx context.Context, game Game, sink Sink, logger *log.Logger) error {
	if logger == nil {
		logger = log.Default()
	}
	p, err := profileFor(game.AppID)
	if err != nil {
		return err
	}
	root, err := filepath.EvalSymlinks(game.InstallPath)
	if err != nil {
		return err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	build, err := p.Inspect(root)
	if err != nil {
		return err
	}
	if !build.Verified {
		return fmt.Errorf("unsupported %s executable version: %s", game.Name, build.Hash)
	}
	hookRVA, err := p.HookRVA(root)
	if err != nil {
		return err
	}
	logger.Printf("[nexas] %s hook ready: sha256=%s rva=0x%x", game.Name, build.Hash, hookRVA)

	waitingLogged := false
	for ctx.Err() == nil {
		process, err := findGameProcess(root, p)
		if errors.Is(err, errGameNotRunning) {
			if !waitingLogged {
				logger.Printf("[nexas] waiting for %s Steam/Proton process", game.Name)
				waitingLogged = true
			}
			if !sleepContext(ctx, 500*time.Millisecond) {
				return nil
			}
			continue
		}
		if err != nil {
			return err
		}
		waitingLogged = false
		address := process.ImageBase + uint64(hookRVA)
		logger.Printf("[nexas] attached execution hook: game=%s pid=%d address=0x%x", game.Name, process.PID, address)
		err = observeProcess(ctx, process.PID, address, p.Normalize, sink)
		if ctx.Err() != nil {
			return nil
		}
		if errors.Is(err, errProcessExited) || os.IsNotExist(err) {
			logger.Printf("[nexas] %s exited; waiting for restart", game.Name)
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func observeProcess(ctx context.Context, pid int, address uint64, normalize func(string) (Line, error), sink Sink) error {
	hook, err := newPerfHook(pid, address)
	if err != nil {
		return err
	}
	defer hook.Close()
	coalescer := newLineCoalescer()
	lastRefresh := time.Now()
	for ctx.Err() == nil {
		samples, err := hook.Poll(100)
		if errors.Is(err, errProcessExited) || os.IsNotExist(err) {
			return errProcessExited
		}
		if err != nil {
			return err
		}
		now := time.Now()
		for _, sample := range samples {
			raw, err := readHookString(pid, sample.AX)
			if err != nil {
				continue
			}
			line, err := normalize(raw)
			if err != nil {
				continue
			}
			for _, ready := range coalescer.Add(line, now) {
				sink(Event{Speaker: ready.Speaker, Text: ready.Text, Timestamp: now})
			}
		}
		for _, ready := range coalescer.FlushDue(now) {
			sink(Event{Speaker: ready.Speaker, Text: ready.Text, Timestamp: now})
		}
		if now.Sub(lastRefresh) >= time.Second {
			if err := hook.RefreshThreads(); err != nil {
				if errors.Is(err, errProcessExited) || os.IsNotExist(err) {
					return errProcessExited
				}
				return err
			}
			lastRefresh = now
		}
	}
	return nil
}

func sleepContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
