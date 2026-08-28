package steam

import (
	"os"
	"path/filepath"
	"testing"
)

func mustReadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestParseLibraryFoldersWithNestedObjects(t *testing.T) {
	data := mustReadFixture(t, "libraryfolders.vdf")
	root, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	folders, ok := root.Object("libraryfolders")
	if !ok {
		t.Fatal("libraryfolders object missing")
	}
	first, ok := folders.Object("0")
	if !ok {
		t.Fatal("library 0 missing")
	}
	if got, _ := first.String("path"); got != "/home/test/.local/share/Steam" {
		t.Fatalf("path = %q", got)
	}
	second, _ := folders.Object("1")
	if got, _ := second.String("path"); got != "/mnt/visual-novels" {
		t.Fatalf("path = %q", got)
	}
}

func TestParseManifest(t *testing.T) {
	manifest, err := ParseManifest(mustReadFixture(t, "appmanifest_111.acf"))
	if err != nil {
		t.Fatal(err)
	}
	if manifest != (Manifest{AppID: "111", Name: "Fake Ren'Py", InstallDir: "FakeRenPy"}) {
		t.Fatalf("manifest = %#v", manifest)
	}
}

func TestParseRejectsUnclosedObject(t *testing.T) {
	if _, err := Parse([]byte("\"root\" { \"key\" \"value\"")); err == nil {
		t.Fatal("Parse accepted unclosed object")
	}
}
