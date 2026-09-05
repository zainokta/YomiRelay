//go:build linux

package nexas

import (
	"encoding/binary"
	"testing"

	"golang.org/x/sys/unix"
)

func TestPerfEventAttrIsUserSpaceOnly(t *testing.T) {
	attr := perfEventAttr(0x679446)
	if attr.Bits&unix.PerfBitExcludeKernel == 0 {
		t.Fatal("kernel execution was not excluded from NeXAS perf hook")
	}
	if attr.Bits&unix.PerfBitExcludeHv == 0 {
		t.Fatal("hypervisor execution was not excluded from NeXAS perf hook")
	}
	if attr.Bits&unix.PerfBitExcludeUser != 0 {
		t.Fatal("user execution was accidentally excluded from NeXAS perf hook")
	}
	if attr.Ext1 != 0x679446 || attr.Bp_type != hwBreakpointExecute {
		t.Fatalf("attr = %#v", attr)
	}
}

func TestDecodePerfSampleReadsAX(t *testing.T) {
	record := make([]byte, 24)
	binary.LittleEndian.PutUint32(record[:4], unix.PERF_RECORD_SAMPLE)
	binary.LittleEndian.PutUint16(record[6:8], uint16(len(record)))
	binary.LittleEndian.PutUint64(record[8:16], unix.PERF_SAMPLE_REGS_ABI_32)
	binary.LittleEndian.PutUint64(record[16:24], 0x12345678)
	got, err := decodePerfSample(record)
	if err != nil {
		t.Fatal(err)
	}
	if got.ABI != unix.PERF_SAMPLE_REGS_ABI_32 || got.AX != 0x12345678 {
		t.Fatalf("sample = %#v", got)
	}
}

func TestRingCopyWraps(t *testing.T) {
	data := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	got := ringCopy(data, 6, 5)
	want := []byte{6, 7, 0, 1, 2}
	if string(got) != string(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
