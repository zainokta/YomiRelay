//go:build linux

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	hwBreakpointExecute = 4
	perfRegX86AX        = 0
	perfRingPages       = 8
	maxHookStringBytes  = 8192
)

type perfSample struct {
	ABI uint64
	AX  uint64
}

type perfThreadEvent struct {
	tid     int
	fd      int
	mapping []byte
}

type perfHook struct {
	pid     int
	address uint64
	events  map[int]*perfThreadEvent
}

func newPerfHook(pid int, address uint64) (*perfHook, error) {
	h := &perfHook{pid: pid, address: address, events: make(map[int]*perfThreadEvent)}
	if err := h.RefreshThreads(); err != nil {
		h.Close()
		return nil, err
	}
	if len(h.events) == 0 {
		return nil, fmt.Errorf("no AQUARIUM threads could be hooked")
	}
	return h, nil
}

func (h *perfHook) Close() {
	for tid, event := range h.events {
		event.close()
		delete(h.events, tid)
	}
}

func (h *perfHook) RefreshThreads() error {
	tids, err := listThreads(h.pid)
	if err != nil {
		return err
	}
	live := make(map[int]bool, len(tids))
	for _, tid := range tids {
		live[tid] = true
		if _, ok := h.events[tid]; ok {
			continue
		}
		event, err := openPerfThreadEvent(tid, h.address)
		if errors.Is(err, unix.ESRCH) {
			continue
		}
		if errors.Is(err, unix.EACCES) || errors.Is(err, unix.EPERM) {
			return fmt.Errorf("perf hook permission denied for AQUARIUM thread %d; YomiRelay does not change kernel security settings: %w", tid, err)
		}
		if err != nil {
			return fmt.Errorf("open perf hook for AQUARIUM thread %d: %w", tid, err)
		}
		h.events[tid] = event
	}
	for tid, event := range h.events {
		if !live[tid] {
			event.close()
			delete(h.events, tid)
		}
	}
	return nil
}

func (h *perfHook) Poll(timeoutMS int) ([]perfSample, error) {
	if len(h.events) == 0 {
		if err := h.RefreshThreads(); err != nil {
			return nil, err
		}
	}
	tids := make([]int, 0, len(h.events))
	for tid := range h.events {
		tids = append(tids, tid)
	}
	sort.Ints(tids)
	fds := make([]unix.PollFd, 0, len(tids))
	for _, tid := range tids {
		fds = append(fds, unix.PollFd{Fd: int32(h.events[tid].fd), Events: unix.POLLIN})
	}
	_, err := unix.Poll(fds, timeoutMS)
	if errors.Is(err, unix.EINTR) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var samples []perfSample
	for i, pollfd := range fds {
		event := h.events[tids[i]]
		if pollfd.Revents&(unix.POLLERR|unix.POLLNVAL) != 0 {
			return nil, fmt.Errorf("perf hook failed for AQUARIUM thread %d", event.tid)
		}
		if pollfd.Revents&(unix.POLLIN|unix.POLLHUP) == 0 {
			continue
		}
		got, err := event.drain()
		if err != nil {
			return nil, err
		}
		samples = append(samples, got...)
	}
	return samples, nil
}

func openPerfThreadEvent(tid int, address uint64) (*perfThreadEvent, error) {
	attr := unix.PerfEventAttr{
		Type:             unix.PERF_TYPE_BREAKPOINT,
		Size:             uint32(unsafe.Sizeof(unix.PerfEventAttr{})),
		Sample:           1,
		Sample_type:      unix.PERF_SAMPLE_REGS_USER,
		Bits:             unix.PerfBitDisabled,
		Wakeup:           1,
		Bp_type:          hwBreakpointExecute,
		Ext1:             address,
		Ext2:             uint64(unsafe.Sizeof(uintptr(0))),
		Sample_regs_user: 1 << perfRegX86AX,
	}
	fd, err := unix.PerfEventOpen(&attr, tid, -1, -1, unix.PERF_FLAG_FD_CLOEXEC)
	if err != nil {
		return nil, err
	}
	pageSize := os.Getpagesize()
	mapping, err := unix.Mmap(fd, 0, pageSize*(1+perfRingPages), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(unix.PERF_EVENT_IOC_ENABLE), 0); errno != 0 {
		_ = unix.Munmap(mapping)
		_ = unix.Close(fd)
		return nil, errno
	}
	return &perfThreadEvent{tid: tid, fd: fd, mapping: mapping}, nil
}

func (e *perfThreadEvent) close() {
	if len(e.mapping) != 0 {
		_ = unix.Munmap(e.mapping)
		e.mapping = nil
	}
	if e.fd >= 0 {
		_ = unix.Close(e.fd)
		e.fd = -1
	}
}

func (e *perfThreadEvent) drain() ([]perfSample, error) {
	if len(e.mapping) < os.Getpagesize() {
		return nil, fmt.Errorf("perf ring mapping is invalid")
	}
	page := (*unix.PerfEventMmapPage)(unsafe.Pointer(&e.mapping[0]))
	head := atomic.LoadUint64(&page.Data_head)
	tail := atomic.LoadUint64(&page.Data_tail)
	dataOffset := page.Data_offset
	dataSize := page.Data_size
	if dataOffset == 0 {
		dataOffset = uint64(os.Getpagesize())
	}
	if dataSize == 0 {
		dataSize = uint64(len(e.mapping)) - dataOffset
	}
	if dataSize == 0 || dataOffset+dataSize > uint64(len(e.mapping)) {
		return nil, fmt.Errorf("perf ring metadata is invalid")
	}
	if head-tail > dataSize {
		tail = head - dataSize
	}
	data := e.mapping[dataOffset : dataOffset+dataSize]
	var result []perfSample
	for tail < head {
		header := ringCopy(data, tail, 8)
		if len(header) != 8 {
			break
		}
		size := uint64(binary.LittleEndian.Uint16(header[6:8]))
		if size < 8 || size > dataSize || tail+size > head {
			atomic.StoreUint64(&page.Data_tail, head)
			return result, fmt.Errorf("invalid perf record size %d", size)
		}
		record := ringCopy(data, tail, int(size))
		tail += size
		if binary.LittleEndian.Uint32(record[:4]) != unix.PERF_RECORD_SAMPLE {
			continue
		}
		sample, err := decodePerfSample(record)
		if err != nil {
			continue
		}
		result = append(result, sample)
	}
	atomic.StoreUint64(&page.Data_tail, tail)
	return result, nil
}

func ringCopy(data []byte, absolute uint64, size int) []byte {
	if size <= 0 || len(data) == 0 {
		return nil
	}
	start := int(absolute % uint64(len(data)))
	result := make([]byte, size)
	first := size
	if available := len(data) - start; first > available {
		first = available
	}
	copy(result, data[start:start+first])
	if first < size {
		copy(result[first:], data[:size-first])
	}
	return result
}

func decodePerfSample(record []byte) (perfSample, error) {
	if len(record) < 24 || binary.LittleEndian.Uint32(record[:4]) != unix.PERF_RECORD_SAMPLE {
		return perfSample{}, fmt.Errorf("not a register sample")
	}
	declared := int(binary.LittleEndian.Uint16(record[6:8]))
	if declared != len(record) {
		return perfSample{}, fmt.Errorf("perf sample size mismatch")
	}
	abi := binary.LittleEndian.Uint64(record[8:16])
	if abi != unix.PERF_SAMPLE_REGS_ABI_32 && abi != unix.PERF_SAMPLE_REGS_ABI_64 {
		return perfSample{}, fmt.Errorf("unsupported register ABI %d", abi)
	}
	return perfSample{ABI: abi, AX: binary.LittleEndian.Uint64(record[16:24])}, nil
}

func readHookString(pid int, ax uint64) (string, error) {
	address := uintptr(uint32(ax))
	if address == 0 {
		return "", fmt.Errorf("NeXAS hook returned a null text pointer")
	}
	buffer := make([]byte, maxHookStringBytes)
	local := unix.Iovec{Base: &buffer[0]}
	local.SetLen(len(buffer))
	n, err := unix.ProcessVMReadv(pid, []unix.Iovec{local}, []unix.RemoteIovec{{Base: address, Len: len(buffer)}}, 0)
	if err != nil {
		return "", err
	}
	if n <= 0 {
		return "", fmt.Errorf("NeXAS hook text pointer was unreadable")
	}
	end := bytes.IndexByte(buffer[:n], 0)
	if end < 0 {
		return "", fmt.Errorf("NeXAS hook string exceeded %d bytes", maxHookStringBytes)
	}
	return string(buffer[:end]), nil
}
