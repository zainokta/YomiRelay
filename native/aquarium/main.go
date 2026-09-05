// The AQUARIUM diagnostic helper runs separately from the HTTP backend.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"yomirelay/internal/aquarium"
	"yomirelay/internal/platform"
	"yomirelay/internal/steam"
)

func main() {
	root := flag.String("install", "", "Steam-discovered AQUARIUM installation (auto-detected when omitted)")
	flag.Parse()
	if err := run(*root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(root string) error {
	if root == "" {
		roots, err := platform.NewSteamLocator().FindSteamRoots()
		if err != nil {
			return err
		}
		games, err := steam.Discover(roots)
		if err != nil {
			return err
		}
		for _, game := range games {
			if game.AppID == aquarium.AppID {
				root = game.InstallPath
				break
			}
		}
		if root == "" {
			return fmt.Errorf("AQUARIUM was not found by Steam discovery")
		}
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	build, err := aquarium.Inspect(root)
	if err != nil {
		return err
	}
	if !build.VerifiedBuild {
		return fmt.Errorf("unsupported AQUARIUM executable version: %s", build.SHA256)
	}
	snapshot, err := capture(root)
	if err != nil {
		return err
	}
	snapshot.Build = build
	return json.NewEncoder(os.Stdout).Encode(snapshot)
}
