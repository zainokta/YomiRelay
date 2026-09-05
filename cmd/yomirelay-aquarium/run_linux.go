//go:build linux

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"yomirelay/internal/platform"
	"yomirelay/internal/source/aquarium"
	"yomirelay/internal/steam"
)

func run(ctx context.Context, root, udpAddress, gameName string) error {
	resolvedRoot, err := resolveAquariumRoot(root)
	if err != nil {
		return err
	}
	build, err := aquarium.Inspect(resolvedRoot)
	if err != nil {
		return err
	}
	if !build.VerifiedBuild {
		return fmt.Errorf("unsupported AQUARIUM executable version: %s", build.SHA256)
	}
	hookPoint, err := aquarium.FindHook(resolvedRoot)
	if err != nil {
		return err
	}
	emitter, err := newUDPEmitter(udpAddress, gameName)
	if err != nil {
		return err
	}
	defer emitter.Close()

	fmt.Fprintf(os.Stderr, "AQUARIUM NeXAS hook ready: sha256=%s rva=0x%x file-offset=0x%x\n", build.SHA256, hookPoint.RVA, hookPoint.FileOffset)
	waitingLogged := false
	for ctx.Err() == nil {
		process, err := findAquariumProcess(resolvedRoot)
		if errors.Is(err, errGameNotRunning) {
			if !waitingLogged {
				fmt.Fprintln(os.Stderr, "waiting for AQUARIUM Steam/Proton process")
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
		address := process.ImageBase + uint64(hookPoint.RVA)
		fmt.Fprintf(os.Stderr, "attached NeXAS execution hook: pid=%d address=0x%x\n", process.PID, address)
		err = observeProcess(ctx, process.PID, address, emitter)
		if ctx.Err() != nil {
			return nil
		}
		if errors.Is(err, errProcessExited) {
			fmt.Fprintln(os.Stderr, "AQUARIUM exited; waiting for restart")
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func resolveAquariumRoot(root string) (string, error) {
	if root == "" {
		roots, err := platform.NewSteamLocator().FindSteamRoots()
		if err != nil {
			return "", err
		}
		games, err := steam.Discover(roots)
		if err != nil {
			return "", err
		}
		for _, game := range games {
			if game.AppID == aquarium.AppID {
				root = game.InstallPath
				break
			}
		}
		if root == "" {
			return "", fmt.Errorf("AQUARIUM was not found by Steam discovery")
		}
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	return filepath.Abs(root)
}

func observeProcess(ctx context.Context, pid int, address uint64, emitter *udpEmitter) error {
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
			line, err := aquarium.NormalizeHookText(raw)
			if err != nil {
				continue
			}
			for _, ready := range coalescer.Add(line, now) {
				emitter.Emit(ready, now)
			}
		}
		for _, ready := range coalescer.FlushDue(now) {
			emitter.Emit(ready, now)
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
