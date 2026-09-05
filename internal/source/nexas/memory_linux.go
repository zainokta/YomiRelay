//go:build linux

package nexas

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

var errRuntimeHookNotFound = errors.New("NeXAS runtime hook signature not found")

type memoryRegion struct {
	Start uint64
	End   uint64
}

func resolveRuntimeHook(pid int, imageBase uint64, imageSize uint32, pattern, mask []byte) (uint64, error) {
	if imageSize == 0 || imageSize > 128<<20 {
		return 0, fmt.Errorf("invalid NeXAS image size 0x%x", imageSize)
	}
	if len(pattern) == 0 || len(pattern) != len(mask) {
		return 0, fmt.Errorf("invalid NeXAS runtime signature")
	}
	imageEnd := imageBase + uint64(imageSize)
	if imageEnd <= imageBase {
		return 0, fmt.Errorf("invalid NeXAS image address range")
	}
	maps, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "maps"))
	if os.IsNotExist(err) {
		return 0, errProcessExited
	}
	if err != nil {
		return 0, err
	}
	regions := executableModuleRegions(string(maps), imageBase, imageEnd)
	if len(regions) == 0 {
		return 0, errRuntimeHookNotFound
	}

	hits := make(map[uint64]struct{})
	for _, region := range regions {
		found, err := scanRuntimeRegion(pid, region, pattern, mask)
		if errors.Is(err, errProcessExited) {
			return 0, err
		}
		if err != nil {
			return 0, err
		}
		for _, address := range found {
			hits[address] = struct{}{}
		}
	}
	if len(hits) == 0 {
		return 0, errRuntimeHookNotFound
	}
	if len(hits) != 1 {
		addresses := make([]uint64, 0, len(hits))
		for address := range hits {
			addresses = append(addresses, address)
		}
		sort.Slice(addresses, func(i, j int) bool { return addresses[i] < addresses[j] })
		parts := make([]string, 0, len(addresses))
		for _, address := range addresses {
			parts = append(parts, fmt.Sprintf("rva=0x%x", address-imageBase))
		}
		return 0, fmt.Errorf("NeXAS runtime hook signature is ambiguous: %d matches (%s)", len(addresses), strings.Join(parts, ", "))
	}
	for address := range hits {
		return address, nil
	}
	return 0, errRuntimeHookNotFound
}

func executableModuleRegions(maps string, imageBase, imageEnd uint64) []memoryRegion {
	var regions []memoryRegion
	for _, line := range strings.Split(maps, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || len(fields[1]) < 3 || fields[1][0] != 'r' || fields[1][2] != 'x' {
			continue
		}
		bounds := strings.SplitN(fields[0], "-", 2)
		if len(bounds) != 2 {
			continue
		}
		start, err1 := strconv.ParseUint(bounds[0], 16, 64)
		end, err2 := strconv.ParseUint(bounds[1], 16, 64)
		if err1 != nil || err2 != nil || start >= end || end <= imageBase || start >= imageEnd {
			continue
		}
		if start < imageBase {
			start = imageBase
		}
		if end > imageEnd {
			end = imageEnd
		}
		if start < end {
			regions = append(regions, memoryRegion{Start: start, End: end})
		}
	}
	return regions
}

func scanRuntimeRegion(pid int, region memoryRegion, pattern, mask []byte) ([]uint64, error) {
	const chunkSize = 1 << 20
	var hits []uint64
	var carry []byte
	for cursor := region.Start; cursor < region.End; {
		want := chunkSize
		if remaining := region.End - cursor; remaining < uint64(want) {
			want = int(remaining)
		}
		buffer := make([]byte, want)
		local := unix.Iovec{Base: &buffer[0]}
		local.SetLen(want)
		n, err := unix.ProcessVMReadv(pid, []unix.Iovec{local}, []unix.RemoteIovec{{Base: uintptr(cursor), Len: want}}, 0)
		if errors.Is(err, unix.ESRCH) {
			return nil, errProcessExited
		}
		if err != nil {
			if errors.Is(err, unix.EFAULT) || errors.Is(err, unix.EIO) {
				carry = nil
				cursor += uint64(want)
				continue
			}
			return nil, err
		}
		if n <= 0 {
			carry = nil
			cursor += uint64(want)
			continue
		}

		combined := make([]byte, 0, len(carry)+n)
		combined = append(combined, carry...)
		combined = append(combined, buffer[:n]...)
		combinedBase := cursor - uint64(len(carry))
		for _, offset := range findMaskedPatternOffsets(combined, pattern, mask) {
			hits = append(hits, combinedBase+uint64(offset))
		}
		keep := len(pattern) - 1
		if keep > len(combined) {
			keep = len(combined)
		}
		carry = append(carry[:0], combined[len(combined)-keep:]...)

		cursor += uint64(n)
		if n < want {
			carry = nil
			cursor += uint64(want - n)
		}
	}
	return hits, nil
}

func findMaskedPatternOffsets(data, pattern, mask []byte) []int {
	if len(pattern) == 0 || len(pattern) != len(mask) || len(data) < len(pattern) {
		return nil
	}
	var result []int
	for start := 0; start+len(pattern) <= len(data); start++ {
		matched := true
		for i := range pattern {
			if mask[i] != 0 && data[start+i] != pattern[i] {
				matched = false
				break
			}
		}
		if matched {
			result = append(result, start)
		}
	}
	return result
}
