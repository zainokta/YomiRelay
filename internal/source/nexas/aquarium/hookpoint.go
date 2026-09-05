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

// PreferredHookRVA is the intended NeXAS2 hook site for the exact verified
// AQUARIUM Steam build. The real Proton image exposes two copies of the same
// short instruction signature; the upstream NeXAS2 search selects the first
// match, which is this lower RVA.
const PreferredHookRVA uint32 = 0x279446

// This instruction shape comes from the AQUARIUM NeXAS2 text path used by the
// verified Steam build. Only the relative call displacement is wildcarded.
//
// In particular, the verified instruction is:
//
//   push eax
//   call rel32
//   mov eax,[esi+000000A4]
//   mov eax,[eax-04]
var hookPattern = []byte{
	0x50,
	0xe8, 0, 0, 0, 0,
	0x8b, 0x86, 0xa4, 0x00, 0x00, 0x00,
	0x8b, 0x40, 0xfc,
}

var hookMask = []byte{
	0xff,
	0xff, 0, 0, 0, 0,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff,
}

// HookSignature returns defensive copies of the runtime instruction pattern.
func HookSignature() (pattern, mask []byte) {
	return append([]byte(nil), hookPattern...), append([]byte(nil), hookMask...)
}

// FindHook searches the on-disk PE and is retained as a fixture/diagnostic
// helper. Runtime hooking resolves the loaded process image instead.
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
		return HookPoint{}, fmt.Errorf("AQUARIUM NeXAS live-hook signature was not found in the on-disk PE")
	}
	if len(found) != 1 {
		return HookPoint{}, fmt.Errorf("AQUARIUM NeXAS live-hook signature is ambiguous in the on-disk PE: %d matches", len(found))
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
