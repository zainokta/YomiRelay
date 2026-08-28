//go:build darwin

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
	return existingRoots([]string{filepath.Join(home, "Library", "Application Support", "Steam")}), nil
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
