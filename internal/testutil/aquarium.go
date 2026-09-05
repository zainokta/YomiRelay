package testutil

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func Aquarium(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"Thumbnail.pac", "Script.pac", "System.pac", "Language.pac"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	b := make([]byte, 1024)
	copy(b, "MZ")
	binary.LittleEndian.PutUint32(b[60:], 128)
	copy(b[128:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(b[132:], 0x14c)
	binary.LittleEndian.PutUint16(b[134:], 1)
	binary.LittleEndian.PutUint16(b[148:], 224)
	binary.LittleEndian.PutUint16(b[152:], 0x10b)
	binary.LittleEndian.PutUint32(b[152+92:], 16)
	binary.LittleEndian.PutUint32(b[152+96+6*8:], 0x1000)
	binary.LittleEndian.PutUint32(b[152+96+6*8+4:], 28)
	copy(b[376:], ".text")
	for off, value := range map[int]uint32{
		384: 512, 388: 0x1000, 392: 512, 396: 512,
		412: 0x60000020,
		524: 2, 528: 100, 536: 576,
	} {
		binary.LittleEndian.PutUint32(b[off:], value)
	}
	copy(b[576:], "RSDS")
	copy(b[600:], "NeXAS（マスター用）（Steam）.pdb\x00")
	copy(b[700:], []byte{0x50, 0xe8, 0x11, 0x22, 0x33, 0x44, 0x8b, 0x86, 0xa4, 0x00, 0x00, 0x00, 0x8b, 0x40, 0xfc})
	if err := os.WriteFile(filepath.Join(root, "Aquarium.exe"), b, 0600); err != nil {
		t.Fatal(err)
	}
	return root
}
