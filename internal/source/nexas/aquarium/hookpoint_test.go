package aquarium

import (
	"os"
	"path/filepath"
	"testing"

	"yomirelay/internal/testutil"
)

func TestFindHookReturnsUniqueExecutablePattern(t *testing.T) {
	root := testutil.Aquarium(t)
	got, err := FindHook(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.FileOffset != 700 || got.RVA != 0x10bc {
		t.Fatalf("hook = %#v", got)
	}
}

func TestHookSignatureKeepsAquariumObjectOffsetExact(t *testing.T) {
	pattern, mask := HookSignature()
	if len(pattern) < 12 || len(mask) != len(pattern) {
		t.Fatalf("invalid signature lengths: pattern=%d mask=%d", len(pattern), len(mask))
	}
	want := []byte{0xa4, 0x00, 0x00, 0x00}
	for i := range want {
		if pattern[8+i] != want[i] || mask[8+i] != 0xff {
			t.Fatalf("object displacement is not exact: pattern=% x mask=% x", pattern, mask)
		}
	}
}

func TestFindHookRejectsMissingOrAmbiguousPattern(t *testing.T) {
	for _, tc := range []struct {
		name      string
		duplicate bool
		remove    bool
	}{
		{name: "missing", remove: true},
		{name: "ambiguous", duplicate: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := testutil.Aquarium(t)
			path := filepath.Join(root, "Aquarium.exe")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if tc.remove {
				for i := 700; i < 715; i++ {
					data[i] = 0x90
				}
			}
			if tc.duplicate {
				copy(data[760:], data[700:715])
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := FindHook(root); err == nil {
				t.Fatal("invalid hook layout accepted")
			}
		})
	}
}
