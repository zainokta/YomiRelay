package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
	"yomirelay/internal/source/aquarium"
)

func capture(root string) (aquarium.Snapshot, error) {
	result := aquarium.Snapshot{
		Status:     "unverified",
		Message:    "Memory candidates may include old script or backlog copies. They are not live dialogue events.",
		Candidates: []aquarium.Candidate{},
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return result, err
	}
	var process string
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		path := filepath.Join("/proc", entry.Name())
		comm, err := os.ReadFile(filepath.Join(path, "comm"))
		if err != nil || strings.TrimSpace(string(comm)) != "Aquarium.exe" {
			continue
		}
		env, err := os.ReadFile(filepath.Join(path, "environ"))
		if err != nil {
			continue
		}
		if !bytes.Contains(append([]byte{0}, env...), []byte("\x00SteamAppId="+aquarium.AppID+"\x00")) {
			continue
		}
		maps, err := os.ReadFile(filepath.Join(path, "maps"))
		if err != nil {
			return result, err
		}
		expected := strings.ReplaceAll(filepath.Join(root, "Aquarium.exe"), " ", "\\040")
		mapped := false
		for _, line := range strings.Split(string(maps), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 6 && fields[2] == "00000000" && strings.Join(fields[5:], " ") == expected {
				mapped = true
			}
		}
		if !mapped {
			continue
		}
		if process != "" {
			return result, fmt.Errorf("multiple AQUARIUM processes found; close the extra instance")
		}
		result.PID = pid
		process = path
	}
	if process == "" {
		return result, fmt.Errorf("AQUARIUM is not visible; start it through Steam and run YomiRelay outside a restricted process sandbox")
	}

	before, err := os.ReadFile(filepath.Join(process, "stat"))
	if err != nil {
		return result, err
	}
	maps, err := os.ReadFile(filepath.Join(process, "maps"))
	if err != nil {
		return result, err
	}
	buffer := make([]byte, 1<<20)
	seen := map[string]bool{}
	for _, line := range strings.Split(string(maps), "\n") {
		start, end, ok := anonymousRegion(line)
		if !ok {
			continue
		}
		var overlap []byte
		for address := start; address < end; {
			if result.BytesRead >= 512<<20 {
				return result, fmt.Errorf("native preview memory budget exceeded")
			}
			size := uint64(len(buffer))
			if remaining := uint64(512<<20) - result.BytesRead; remaining < size {
				size = remaining
			}
			if end-address < size {
				size = end - address
			}
			local := unix.Iovec{Base: &buffer[0]}
			local.SetLen(int(size))
			n, err := unix.ProcessVMReadv(result.PID, []unix.Iovec{local}, []unix.RemoteIovec{{Base: uintptr(address), Len: int(size)}}, 0)
			if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
				return result, fmt.Errorf("AQUARIUM memory access denied; no game changes were made: %w", err)
			}
			if err != nil || n <= 0 {
				overlap = nil
				address += size
				continue
			}

			result.BytesRead += uint64(n)
			data := append(overlap, buffer[:n]...)
			base := address - uint64(len(overlap))
			for _, candidate := range aquarium.Candidates(data, base) {
				if !seen[candidate.Raw] {
					seen[candidate.Raw] = true
					result.Candidates = append(result.Candidates, candidate)
					if len(result.Candidates) >= 128 {
						break
					}
				}
			}
			if len(result.Candidates) >= 128 {
				break
			}
			tail := len(data) - 8192
			if tail < 0 {
				tail = 0
			}
			overlap = append([]byte(nil), data[tail:]...)
			address += uint64(n)
		}
		if len(result.Candidates) >= 128 {
			result.Message += " Showing the first 128 distinct candidates."
			break
		}
	}

	after, err := os.ReadFile(filepath.Join(process, "stat"))
	if err != nil {
		return result, err
	}
	identity := func(stat []byte) string {
		pos := bytes.LastIndexByte(stat, ')')
		if pos < 0 {
			return ""
		}
		fields := strings.Fields(string(stat[pos+1:]))
		if len(fields) < 20 {
			return ""
		}
		return fields[19]
	}
	if identity(before) == "" || identity(before) != identity(after) {
		return result, fmt.Errorf("AQUARIUM process changed during inspection")
	}
	return result, nil
}

func anonymousRegion(line string) (uint64, uint64, bool) {
	fields := strings.Fields(line)
	if len(fields) != 5 || fields[1] != "rw-p" {
		return 0, 0, false
	}
	bounds := strings.Split(fields[0], "-")
	if len(bounds) != 2 {
		return 0, 0, false
	}
	start, e1 := strconv.ParseUint(bounds[0], 16, 32)
	end, e2 := strconv.ParseUint(bounds[1], 16, 32)
	return start, end, e1 == nil && e2 == nil && start < end
}
