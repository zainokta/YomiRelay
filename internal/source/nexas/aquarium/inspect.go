// Package aquarium contains verified AQUARIUM/NeXAS build identity and hook metadata.
package aquarium

import (
	"bytes"
	"crypto/sha256"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

const AppID = "2515070"
const ExecutableSHA256 = "5cb02d21fa02172cbfb3389c6194308c541213d023a88fabc77d1ec34e6871d7"

type Build struct {
	Architecture  string `json:"architecture"`
	SHA256        string `json:"sha256"`
	Size          int64  `json:"size"`
	ImageSize     uint32 `json:"imageSize"`
	PETimestamp   uint32 `json:"peTimestamp"`
	VerifiedBuild bool   `json:"verifiedBuild"`
}

// Inspect requires both the archive layout and a NeXAS CodeView record in a valid x86 PE.
// VerifiedBuild identifies the AQUARIUM executable build that was investigated for the live hook.
func Inspect(root string) (Build, error) {
	for _, name := range []string{"Thumbnail.pac", "Script.pac", "System.pac", "Language.pac"} {
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil || !info.Mode().IsRegular() {
			return Build{}, fmt.Errorf("missing AQUARIUM archive %s", name)
		}
	}
	path := filepath.Join(root, "Aquarium.exe")
	info, err := os.Stat(path)
	if err != nil {
		return Build{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > 64<<20 {
		return Build{}, fmt.Errorf("unsupported AQUARIUM executable size")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Build{}, err
	}
	f, err := pe.NewFile(bytes.NewReader(data))
	if err != nil {
		return Build{}, fmt.Errorf("invalid Aquarium.exe PE: %w", err)
	}
	defer f.Close()
	optional, ok := f.OptionalHeader.(*pe.OptionalHeader32)
	if !ok || f.Machine != pe.IMAGE_FILE_MACHINE_I386 {
		return Build{}, fmt.Errorf("unsupported AQUARIUM PE architecture")
	}
	if optional.SizeOfImage == 0 || optional.SizeOfImage > 128<<20 {
		return Build{}, fmt.Errorf("unsupported AQUARIUM PE image size")
	}
	directory := optional.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_DEBUG]
	var debug []byte
	for _, section := range f.Sections {
		if directory.VirtualAddress < section.VirtualAddress {
			continue
		}
		offset := uint64(directory.VirtualAddress - section.VirtualAddress)
		if offset+uint64(directory.Size) > uint64(section.Size) {
			continue
		}
		start := uint64(section.Offset) + offset
		end := start + uint64(directory.Size)
		if end <= uint64(len(data)) {
			debug = data[start:end]
		}
	}
	nexas := false
	for offset := 0; offset+28 <= len(debug); offset += 28 {
		record := debug[offset : offset+28]
		if binary.LittleEndian.Uint32(record[12:]) != 2 {
			continue
		}
		size := uint64(binary.LittleEndian.Uint32(record[16:]))
		start := uint64(binary.LittleEndian.Uint32(record[24:]))
		if size < 25 || start+size > uint64(len(data)) {
			continue
		}
		cv := data[start : start+size]
		if !bytes.HasPrefix(cv, []byte("RSDS")) {
			continue
		}
		name := bytes.TrimRight(cv[24:], "\x00")
		if bytes.HasPrefix(name, []byte("NeXAS")) && bytes.HasSuffix(name, []byte(".pdb")) {
			nexas = true
		}
	}
	if !nexas {
		return Build{}, fmt.Errorf("AQUARIUM NeXAS binary fingerprint not found")
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	return Build{
		Architecture:  "x86",
		SHA256:        hash,
		Size:          int64(len(data)),
		ImageSize:     optional.SizeOfImage,
		PETimestamp:   f.TimeDateStamp,
		VerifiedBuild: hash == ExecutableSHA256,
	}, nil
}
