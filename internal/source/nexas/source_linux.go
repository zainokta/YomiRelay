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
	pattern, mask := p.Signature()
	if len(pattern) == 0 || len(pattern) != len(mask) {
		return fmt.Errorf("invalid NeXAS hook signature profile for %s", game.Name)
	}
	logger.Printf("[nexas] %s profile ready: sha256=%s image-size=0x%x preferred-rva=0x%x", game.Name, build.Hash, build.ImageSize, p.PreferredHookRVA)

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

		address, err := waitForRuntimeHook(ctx, process, build.ImageSize, pattern, mask, p.PreferredHookRVA, game.Name, logger)
		if ctx.Err() != nil {
			return nil
		}
		if errors.Is(err, errProcessExited) || os.IsNotExist(err) {
			logger.Printf("[nexas] %s exited before the runtime hook resolved; waiting for restart", game.Name)
			continue
		}
		if err != nil {
			return err
		}
		rva := address - process.ImageBase
		logger.Printf("[nexas] resolved runtime hook: game=%s pid=%d rva=0x%x address=0x%x", game.Name, process.PID, rva, address)
		logger.Printf("[nexas] attached execution hook: game=%s pid=%d address=0x%x", game.Name, process.PID, address)
		err = observeProcess(ctx, process.PID, address, p.Normalize, sink, logger)
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

func waitForRuntimeHook(ctx context.Context, process processInfo, imageSize uint32, pattern, mask []byte, preferredRVA uint32, gameName string, logger *log.Logger) (uint64, error) {
	announced := false
	lastStatus := time.Time{}
	for ctx.Err() == nil {
		address, err := resolveRuntimeHook(process.PID, process.ImageBase, imageSize, pattern, mask, preferredRVA)
		if err == nil {
			return address, nil
		}
		if !errors.Is(err, errRuntimeHookNotFound) {
			return 0, err
		}
		now := time.Now()
		if !announced || now.Sub(lastStatus) >= 10*time.Second {
			logger.Printf("[nexas] waiting for %s runtime hook signature in loaded process memory", gameName)
			announced = true
			lastStatus = now
		}
		if _, statErr := os.Stat(fmt.Sprintf("/proc/%d", process.PID)); os.IsNotExist(statErr) {
			return 0, errProcessExited
		}
		if !sleepContext(ctx, 250*time.Millisecond) {
			return 0, ctx.Err()
		}
	}
	return 0, ctx.Err()
}

func observeProcess(ctx context.Context, pid int, address uint64, normalize func(string) (Line, error), sink Sink, logger *log.Logger) error {
	hook, err := newPerfHook(pid, address)
	if err != nil {
		return err
	}
	defer hook.Close()
	if logger == nil {
		logger = log.Default()
	}
	filter := newRenderContextFilter(func(contextID uint64) {
		logger.Printf("[nexas] selected dialogue render context: pid=%d esi=0x%x", pid, uint32(contextID))
	})
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
			contextID := uint64(uint32(sample.SI))
			for _, ready := range filter.Add(contextID, line, now) {
				sink(Event{Speaker: ready.Speaker, Text: ready.Text, Timestamp: now})
			}
		}
		for _, ready := range filter.FlushDue(now) {
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
