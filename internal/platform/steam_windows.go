//go:build windows

package platform

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

type steamLocator struct{}

func newSteamLocator() SteamLocator { return steamLocator{} }

func (steamLocator) FindSteamRoots() ([]string, error) {
	candidates := make([]string, 0, 2)
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE)
	if err == nil {
		if path, _, valueErr := key.GetStringValue("SteamPath"); valueErr == nil && path != "" {
			candidates = append(candidates, path)
		}
		_ = key.Close()
	}
	candidates = append(candidates, `C:\Program Files (x86)\Steam`)
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
