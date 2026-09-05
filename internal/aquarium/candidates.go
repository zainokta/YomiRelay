package aquarium

import (
	"bytes"
	"fmt"
	"unicode/utf8"
)

type Candidate struct {
	Address string `json:"address"`
	Raw     string `json:"raw"`
}

type Snapshot struct {
	Status     string      `json:"status"`
	Message    string      `json:"message"`
	PID        int         `json:"pid,omitempty"`
	Build      Build       `json:"build"`
	BytesRead  uint64      `json:"bytesRead"`
	Candidates []Candidate `json:"candidates"`
}

// Candidates finds bounded, terminated NeXAS speaker-tagged strings for diagnostics.
// These may be stale script/backlog copies; they are NEVER live Dialogue events.
func Candidates(data []byte, base uint64) []Candidate {
	var found []Candidate
	prefix := []byte("【")
	for offset := 0; offset < len(data); {
		index := bytes.Index(data[offset:], prefix)
		if index < 0 {
			break
		}
		start := offset + index
		offset = start + len(prefix)
		end := start + 8192
		if end > len(data) {
			end = len(data)
		}
		length := bytes.IndexByte(data[start:end], 0)
		if length < 0 {
			continue
		}
		raw := data[start : start+length]
		if !utf8.Valid(raw) || !bytes.Contains(raw, []byte("】@n")) {
			continue
		}
		found = append(found, Candidate{Address: fmt.Sprintf("0x%x", base+uint64(start)), Raw: string(raw)})
		if len(found) == 128 {
			break
		}
	}
	return found
}
