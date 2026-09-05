//go:build linux

package nexas

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var errGameNotRunning = errors.New("NeXAS game is not running")
var errProcessExited = errors.New("NeXAS game process exited")

type processInfo struct {
	PID       int
	ImageBase uint64
}

func findGameProcess(root string, p profile) (processInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return processInfo{}, err
	}
	executable := filepath.Join(root, p.Executable)
	var found *processInfo
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		proc := filepath.Join("/proc", entry.Name())
		comm, err := os.ReadFile(filepath.Join(proc, "comm"))
		if err != nil || strings.TrimSpace(string(comm)) != p.Executable {
			continue
		}
		env, err := os.ReadFile(filepath.Join(proc, "environ"))
		if err != nil || !bytes.Contains(append([]byte{0}, env...), []byte("\x00SteamAppId="+p.AppID+"\x00")) {
			continue
		}
		maps, err := os.ReadFile(filepath.Join(proc, "maps"))
		if err != nil {
			continue
		}
		base, ok := imageBaseFromMaps(string(maps), executable)
		if !ok {
			continue
		}
		if found != nil {
			return processInfo{}, fmt.Errorf("multiple %s processes found; close the extra instance", p.Executable)
		}
		value := processInfo{PID: pid, ImageBase: base}
		found = &value
	}
	if found == nil {
		return processInfo{}, errGameNotRunning
	}
	return *found, nil
}

func imageBaseFromMaps(maps, executable string) (uint64, bool) {
	for _, line := range strings.Split(maps, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 || fields[2] != "00000000" {
			continue
		}
		path := unescapeProcPath(strings.Join(fields[5:], " "))
		if filepath.Clean(path) != filepath.Clean(executable) {
			continue
		}
		bounds := strings.SplitN(fields[0], "-", 2)
		if len(bounds) != 2 {
			continue
		}
		base, err := strconv.ParseUint(bounds[0], 16, 64)
		if err == nil {
			return base, true
		}
	}
	return 0, false
}

func unescapeProcPath(value string) string {
	replacer := strings.NewReplacer("\\040", " ", "\\011", "\t", "\\012", "\n", "\\134", "\\")
	return replacer.Replace(value)
}

func listThreads(pid int) ([]int, error) {
	entries, err := os.ReadDir(filepath.Join("/proc", strconv.Itoa(pid), "task"))
	if os.IsNotExist(err) {
		return nil, errProcessExited
	}
	if err != nil {
		return nil, err
	}
	result := make([]int, 0, len(entries))
	for _, entry := range entries {
		id, err := strconv.Atoi(entry.Name())
		if err == nil {
			result = append(result, id)
		}
	}
	if len(result) == 0 {
		return nil, errProcessExited
	}
	return result, nil
}
