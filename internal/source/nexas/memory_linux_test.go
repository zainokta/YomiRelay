//go:build linux

package nexas

import (
	"strings"
	"testing"
)

func TestFindMaskedPatternOffsetsSupportsWildcards(t *testing.T) {
	pattern := []byte{0x50, 0xe8, 0, 0, 0, 0, 0x8b, 0x86}
	mask := []byte{0xff, 0xff, 0, 0, 0, 0, 0xff, 0xff}
	data := []byte{0x90, 0x50, 0xe8, 1, 2, 3, 4, 0x8b, 0x86, 0x90}
	got := findMaskedPatternOffsets(data, pattern, mask)
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("offsets = %#v", got)
	}
}

func TestFindMaskedPatternOffsetsFindsAmbiguousMatches(t *testing.T) {
	pattern := []byte{0xaa, 0, 0xbb}
	mask := []byte{0xff, 0, 0xff}
	data := []byte{0xaa, 1, 0xbb, 0x90, 0xaa, 2, 0xbb}
	got := findMaskedPatternOffsets(data, pattern, mask)
	if len(got) != 2 || got[0] != 0 || got[1] != 4 {
		t.Fatalf("offsets = %#v", got)
	}
}

func TestSelectRuntimeHookUsesPreferredProfileRVA(t *testing.T) {
	const base = 0x40000000
	hits := map[uint64]struct{}{
		base + 0x279446: {},
		base + 0x27b86e: {},
	}
	got, err := selectRuntimeHook(hits, base, 0x279446)
	if err != nil {
		t.Fatal(err)
	}
	if got != base+0x279446 {
		t.Fatalf("hook = 0x%x", got)
	}
}

func TestSelectRuntimeHookFailsWhenPreferredProfileRVADisappears(t *testing.T) {
	const base = 0x40000000
	hits := map[uint64]struct{}{base + 0x27b86e: {}}
	_, err := selectRuntimeHook(hits, base, 0x279446)
	if err == nil || !strings.Contains(err.Error(), "rva=0x279446") || !strings.Contains(err.Error(), "rva=0x27b86e") {
		t.Fatalf("error = %v", err)
	}
}

func TestExecutableModuleRegionsClampsToImage(t *testing.T) {
	maps := "003ff000-00401000 r-xp 00000000 00:00 0\n" +
		"00401000-00403000 r--p 00001000 00:00 0\n" +
		"00403000-00406000 r-xp 00003000 00:00 0\n" +
		"00500000-00501000 r-xp 00000000 00:00 0\n"
	got := executableModuleRegions(maps, 0x00400000, 0x00405000)
	if len(got) != 2 {
		t.Fatalf("regions = %#v", got)
	}
	if got[0].Start != 0x00400000 || got[0].End != 0x00401000 || got[1].Start != 0x00403000 || got[1].End != 0x00405000 {
		t.Fatalf("regions = %#v", got)
	}
}
