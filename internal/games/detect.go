package games

import (
	"os"
	"path/filepath"
)

// IsRenPy detects a Ren'Py installation from its filesystem layout alone.
func IsRenPy(installPath string) bool {
	root := filepath.Clean(installPath)
	gameInfo, err := os.Stat(filepath.Join(root, "game"))
	if err != nil || !gameInfo.IsDir() {
		return false
	}
	if info, err := os.Stat(filepath.Join(root, "renpy")); err == nil && info.IsDir() {
		return true
	}
	for _, pattern := range []string{"*.rpa", "*.rpy"} {
		matches, err := filepath.Glob(filepath.Join(root, "game", pattern))
		if err == nil && len(matches) > 0 {
			return true
		}
	}
	for _, relative := range []string{
		filepath.Join("lib", "py2-linux-x86_64", "renpy"),
		filepath.Join("lib", "py3-linux-x86_64", "renpy"),
		"renpy.exe", "renpy.sh",
	} {
		if _, err := os.Stat(filepath.Join(root, relative)); err == nil {
			return true
		}
	}
	return false
}
