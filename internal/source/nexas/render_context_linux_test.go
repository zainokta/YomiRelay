//go:build linux

package nexas

import "testing"

func TestStableRenderContextKeyGroupsInstancesByVTable(t *testing.T) {
	const (
		base      = uint64(0x400000)
		imageSize = uint32(0x6ec000)
		vtable    = uint32(0x8a1000)
	)
	left := stableRenderContextKey(0x12000000, vtable, base, imageSize)
	right := stableRenderContextKey(0x13000000, vtable, base, imageSize)
	if left != right || left != uint64(vtable) {
		t.Fatalf("keys = 0x%x, 0x%x", left, right)
	}
}

func TestStableRenderContextKeyFallsBackForNonModuleWord(t *testing.T) {
	key := stableRenderContextKey(0x12000000, 0x22000000, 0x400000, 0x6ec000)
	if key&renderContextInstanceBit == 0 || uint32(key) != 0x12000000 {
		t.Fatalf("fallback key = 0x%x", key)
	}
}

func TestDescribeRenderContextKey(t *testing.T) {
	if got := describeRenderContextKey(0x8a1000); got != "class-vtable=0x8a1000" {
		t.Fatalf("class description = %q", got)
	}
	if got := describeRenderContextKey(fallbackRenderContextKey(0x12000000)); got != "instance-esi=0x12000000" {
		t.Fatalf("instance description = %q", got)
	}
}
