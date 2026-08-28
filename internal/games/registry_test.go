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

func TestRegistryDecoratesActivityWhenReturningGames(t *testing.T) {
	root := t.TempDir()
	if err := makeRenPyFixture(root); err != nil {
		t.Fatal(err)
	}
	lastSeen := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
	active := true
	known := true
	registry := NewRegistry(func() ([]steam.Installation, error) {
		return []steam.Installation{{AppID: "111", Name: "Detected", InstallPath: root}}, nil
	}, nil, func(string) (time.Time, bool, bool) {
		return lastSeen, active, known
	})
	if err := registry.Refresh(); err != nil {
		t.Fatal(err)
	}
	active = false
	lastSeen = lastSeen.Add(30 * time.Second)

	listed := registry.List()
	if len(listed) != 1 || listed[0].Active || listed[0].LastSeen == nil || !listed[0].LastSeen.Equal(lastSeen) {
		t.Fatalf("listed activity = %#v", listed)
	}
	got, ok := registry.Get("111")
	if !ok || got.Active || got.LastSeen == nil || !got.LastSeen.Equal(lastSeen) {
		t.Fatalf("got activity = %#v, ok %v", got, ok)
	}
}

func makeRenPyFixture(root string) error {
	if err := makeGameFixture(root); err != nil {
		return err
	}
	return os.Mkdir(filepath.Join(root, "renpy"), 0o755)
}

func makeGameFixture(root string) error { return os.Mkdir(filepath.Join(root, "game"), 0o755) }
