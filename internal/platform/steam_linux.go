//go:build linux

package platform

import (
	"os"
	"path/filepath"
)

type steamLocator struct{}

func newSteamLocator() SteamLocator { return steamLocator{} }

func (steamLocator) FindSteamRoots() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	candidates := []string{
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".steam", "root"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
	}
	return existingRoots(candidates), nil
}

func existingRoots(candidates []string) []string {
	result := make([]string, 0, len(candidates))
	seen := make(map[string]struct{})
	for _, candidate := range candidates {
		clean := filepath.Clean(candidate)
		info, err := os.Stat(clean)
		if err != nil || !info.IsDir() {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		result = append(result, clean)
	}
	return result
}
