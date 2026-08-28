package steam

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverIncludesManifestsFromEveryLibrary(t *testing.T) {
	root := t.TempDir()
	second := filepath.Join(t.TempDir(), "library")
	for _, dir := range []string{root, second} {
		if err := os.MkdirAll(filepath.Join(dir, "steamapps", "common"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, "steamapps", "common", "FakeRenPy"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(second, "steamapps", "common", "NotRenPy"), 0o755); err != nil {
		t.Fatal(err)
	}
	libraryVDF := fmt.Sprintf("\"libraryfolders\" { \"0\" { \"path\" \"%s\" } \"1\" { \"path\" \"%s\" } }\n", vdfPath(root), vdfPath(second))
	if err := os.WriteFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"), []byte(libraryVDF), 0o644); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, filepath.Join(root, "steamapps", "appmanifest_111.acf"), "111", "Fake Ren'Py", "FakeRenPy")
	writeManifest(t, filepath.Join(second, "steamapps", "appmanifest_222.acf"), "222", "Other", "NotRenPy")

	got, err := Discover([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d installations: %#v", len(got), got)
	}
	if got[0] != (Installation{AppID: "111", Name: "Fake Ren'Py", InstallPath: filepath.Join(root, "steamapps", "common", "FakeRenPy")}) {
		t.Fatalf("first installation = %#v", got[0])
	}
	if got[1] != (Installation{AppID: "222", Name: "Other", InstallPath: filepath.Join(second, "steamapps", "common", "NotRenPy")}) {
		t.Fatalf("second installation = %#v", got[1])
	}
}

func TestDiscoverSkipsMalformedManifestAndKeepsOtherGames(t *testing.T) {
	root := t.TempDir()
	steamapps := filepath.Join(root, "steamapps")
	if err := os.MkdirAll(filepath.Join(steamapps, "common", "Valid"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, filepath.Join(steamapps, "appmanifest_111.acf"), "111", "Valid", "Valid")
	if err := os.WriteFile(filepath.Join(steamapps, "appmanifest_222.acf"), []byte("broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Discover([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].AppID != "111" {
		t.Fatalf("got %#v", got)
	}
}

func writeManifest(t *testing.T, path, appID, name, installDir string) {
	t.Helper()
	data := fmt.Sprintf("\"AppState\" { \"appid\" \"%s\" \"name\" \"%s\" \"installdir\" \"%s\" }\n", appID, name, installDir)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func vdfPath(path string) string {
	var out []byte
	for _, ch := range path {
		if ch == '\\' {
			out = append(out, '\\', '\\')
		} else if ch == '"' {
			out = append(out, '\\', '"')
		} else {
			out = append(out, string(ch)...)
		}
	}
	return string(out)
}
