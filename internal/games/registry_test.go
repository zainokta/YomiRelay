package games

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"yomirelay/internal/steam"
)

func TestRegistryRefreshListsOnlyDetectedGames(t *testing.T) {
	trueRoot := t.TempDir()
	if err := makeRenPyFixture(trueRoot); err != nil {
		t.Fatal(err)
	}
	falseRoot := t.TempDir()
	if err := makeGameFixture(falseRoot); err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry(func() ([]steam.Installation, error) {
		return []steam.Installation{
			{AppID: "111", Name: "Detected", InstallPath: trueRoot},
			{AppID: "222", Name: "Not detected", InstallPath: falseRoot},
		}, nil
	}, func(Game) bool { return false }, func(string) (time.Time, bool, bool) {
		return time.Time{}, false, false
	})
	if err := registry.Refresh(); err != nil {
		t.Fatal(err)
	}
	got := registry.List()
	if len(got) != 1 {
		t.Fatalf("got %#v", got)
	}
	want := got[0]
	if want.AppID != "111" || want.Engine != "renpy" || want.HookInstalled || want.Active {
		t.Fatalf("game = %#v", want)
	}
}

func makeRenPyFixture(root string) error {
	if err := makeGameFixture(root); err != nil {
		return err
	}
	return os.Mkdir(filepath.Join(root, "renpy"), 0o755)
}

func makeGameFixture(root string) error { return os.Mkdir(filepath.Join(root, "game"), 0o755) }
