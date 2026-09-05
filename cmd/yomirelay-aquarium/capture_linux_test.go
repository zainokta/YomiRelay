package main

import "testing"

func TestReadableAnonymousRegion(t *testing.T) {
	for _, tc := range []struct {
		line string
		want bool
	}{
		{"01200000-01300000 rw-p 00000000 00:00 0", true},
		{"01200000-01300000 r--p 00000000 00:00 0", false},
		{"01200000-01300000 rw-p 00000000 00:00 1 /game/Script.pac", false},
		{"bad rw-p", false},
		{"01300000-01200000 rw-p 00000000 00:00 0", false},
	} {
		_, _, ok := anonymousRegion(tc.line)
		if ok != tc.want {
			t.Errorf("region %q: %v", tc.line, ok)
		}
	}
}
