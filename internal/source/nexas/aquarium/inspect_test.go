package aquarium

import (
	"os"
	"path/filepath"
	"testing"

	"yomirelay/internal/testutil"
)

func TestInspectRequiresCorroboratingPEAndFiles(t *testing.T) {
	for _, tc := range []struct {
		name, remove string
		corrupt      bool
		want         bool
	}{
		{name: "NeXAS PE with archives", want: true},
		{name: "missing thumbnail", remove: "Thumbnail.pac"},
		{name: "missing script", remove: "Script.pac"},
		{name: "PAC files alone", corrupt: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := testutil.Aquarium(t)
			if tc.remove != "" {
				if err := os.Remove(filepath.Join(root, tc.remove)); err != nil {
					t.Fatal(err)
				}
			}
			if tc.corrupt {
				if err := os.WriteFile(filepath.Join(root, "Aquarium.exe"), []byte("NeXAS.pdb"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			info, err := Inspect(root)
			if (err == nil) != tc.want {
				t.Fatalf("Inspect = %+v, %v", info, err)
			}
			if tc.want && (info.Architecture != "x86" || info.SHA256 == "" || info.ImageSize == 0 || info.VerifiedBuild) {
				t.Fatalf("metadata = %+v", info)
			}
		})
	}
}

func TestInspectRejectsUnrelatedPE(t *testing.T) {
	root := testutil.Aquarium(t)
	path := filepath.Join(root, "Aquarium.exe")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	copy(b[600:], make([]byte, 100))
	copy(b[600:], "OtherEngine.pdb\x00")
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(root); err == nil {
		t.Fatal("unrelated executable accepted")
	}
}
