package games

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsRenPyRequiresGameDirectoryAndCorroboratingSignal(t *testing.T) {
	cases := []struct {
		name   string
		signal func(string) error
		want   bool
	}{
		{"runtime directory", func(root string) error { return os.Mkdir(filepath.Join(root, "renpy"), 0o755) }, true},
		{"script", func(root string) error {
			return os.WriteFile(filepath.Join(root, "game", "script.rpy"), []byte("label start:"), 0o644)
		}, true},
		{"archive", func(root string) error {
			return os.WriteFile(filepath.Join(root, "game", "archive.rpa"), []byte("archive"), 0o644)
		}, true},
		{"bare game", func(string) error { return nil }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "game"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := tc.signal(root); err != nil {
				t.Fatal(err)
			}
			if got := IsRenPy(root); got != tc.want {
				t.Fatalf("IsRenPy() = %v, want %v", got, tc.want)
			}
		})
	}
	nameOnly := filepath.Join(t.TempDir(), "DefinitelyNotRenPy")
	if err := os.MkdirAll(filepath.Join(nameOnly, "game"), 0o755); err != nil {
		t.Fatal(err)
	}
	if IsRenPy(nameOnly) {
		t.Fatal("directory name was used as a Ren'Py signal")
	}
}
