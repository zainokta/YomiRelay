//go:build linux

package nexas

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/sys/unix"
)

const renderContextInstanceBit uint64 = 1 << 63

func renderContextKey(pid int, si uint64, imageBase uint64, imageSize uint32) uint64 {
	esi := uint32(si)
	if esi == 0 {
		return 0
	}
	firstWord, err := readProcessUint32(pid, uintptr(esi))
	if err != nil {
		return fallbackRenderContextKey(esi)
	}
	return stableRenderContextKey(esi, firstWord, imageBase, imageSize)
}

func stableRenderContextKey(esi uint32, firstWord uint32, imageBase uint64, imageSize uint32) uint64 {
	if esi == 0 {
		return 0
	}
	imageEnd := imageBase + uint64(imageSize)
	if imageSize != 0 && imageEnd > imageBase {
		value := uint64(firstWord)
		if firstWord != 0 && value >= imageBase && value < imageEnd {
			return value
		}
	}
	return fallbackRenderContextKey(esi)
}

func fallbackRenderContextKey(esi uint32) uint64 {
	return renderContextInstanceBit | uint64(esi)
}

func describeRenderContextKey(key uint64) string {
	if key == 0 {
		return "context=0"
	}
	if key&renderContextInstanceBit != 0 {
		return fmt.Sprintf("instance-esi=0x%x", uint32(key))
	}
	return fmt.Sprintf("class-vtable=0x%x", uint32(key))
}

func readProcessUint32(pid int, address uintptr) (uint32, error) {
	if address == 0 {
		return 0, fmt.Errorf("null process address")
	}
	buffer := make([]byte, 4)
	local := unix.Iovec{Base: &buffer[0]}
	local.SetLen(len(buffer))
	n, err := unix.ProcessVMReadv(pid, []unix.Iovec{local}, []unix.RemoteIovec{{Base: address, Len: len(buffer)}}, 0)
	if err != nil {
		return 0, err
	}
	if n != len(buffer) {
		return 0, fmt.Errorf("short process read: %d", n)
	}
	return binary.LittleEndian.Uint32(buffer), nil
}
