package aquarium

import (
	"bytes"
	"debug/pe"
	"fmt"
	"os"
	"path/filepath"
)

// HookPoint is the verified instruction location where AQUARIUM's NeXAS text
// pointer is available in EAX before the engine dispatches the string.
type HookPoint struct {
	FileOffset uint64
	RVA        uint32
}

// This instruction shape was independently validated against AQUARIUM's x86
// text path. Relative call and object-field displacements are intentionally
// wildcarded so the matcher describes instructions rather than absolute addresses.
var hookPattern = []byte{
	0x50,
	0xe8, 0, 0, 0, 0,
	0x8b, 0x86, 0, 0, 0, 0,
	0x8b, 0x40, 0xfc,
}

var hookMask = []byte{
	0xff,
	0xff, 0, 0, 0, 0,
	0xff, 0xff, 0, 0, 0, 0,
	0xff, 0xff, 0xff,
}

func FindHook(root string) (HookPoint, error) {
	path := filepath.Join(root, "Aquarium.exe")
	data, err := os.ReadFile(path)
	if err != nil {
		return HookPoint{}, err
	}
	f, err := pe.NewFile(bytes.NewReader(data))
	if err != nil {
		return HookPoint{}, fmt.Errorf("invalid Aquarium.exe PE: %w", err)
	}
	defer f.Close()
	if _, ok := f.OptionalHeader.(*pe.OptionalHeader32); !ok || f.Machine != pe.IMAGE_FILE_MACHINE_I386 {
		return HookPoint{}, fmt.Errorf("AQUARIUM hook requires an x86 PE")
	}

	var found []HookPoint
	for _, section := range f.Sections {
		if section.Characteristics&pe.IMAGE_SCN_MEM_EXECUTE == 0 || section.Size == 0 {
			continue
		}
		start := uint64(section.Offset)
		end := start + uint64(section.Size)
		if start >= uint64(len(data)) {
			continue
		}
		if end > uint64(len(data)) {
			end = uint64(len(data))
		}
		for i := start; i+uint64(len(hookPattern)) <= end; i++ {
			if matchHookPattern(data[i : i+uint64(len(hookPattern))]) {
				found = append(found, HookPoint{FileOffset: i, RVA: section.VirtualAddress + uint32(i-start)})
			}
		}
	}
	if len(found) == 0 {
		return HookPoint{}, fmt.Errorf("AQUARIUM NeXAS live-hook signature was not found")
	}
	if len(found) != 1 {
		return HookPoint{}, fmt.Errorf("AQUARIUM NeXAS live-hook signature is ambiguous: %d matches", len(found))
	}
	return found[0], nil
}

func matchHookPattern(data []byte) bool {
	if len(data) != len(hookPattern) {
		return false
	}
	for i := range hookPattern {
		if hookMask[i] != 0 && data[i] != hookPattern[i] {
			return false
		}
	}
	return true
}
