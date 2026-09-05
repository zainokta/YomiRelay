package aquarium

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const previewWarning = "Experimental memory preview. Candidates are not chronological and are never added to Reader history or the translation queue."

type PreviewCandidate struct {
	Address string `json:"address"`
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

type Preview struct {
	Status     string             `json:"status"`
	Message    string             `json:"message"`
	PID        int                `json:"pid,omitempty"`
	Build      Build              `json:"build"`
	BytesRead  uint64             `json:"bytesRead"`
	Candidates []PreviewCandidate `json:"candidates"`
}

// BuildPreview converts raw memory candidates into display-only Reader candidates.
// Invalid or non-dialogue strings are dropped rather than entering the canonical dialogue pipeline.
func BuildPreview(snapshot Snapshot) Preview {
	message := strings.TrimSpace(snapshot.Message)
	if message == "" {
		message = previewWarning
	} else {
		message += " " + previewWarning
	}
	preview := Preview{
		Status:     snapshot.Status,
		Message:    message,
		PID:        snapshot.PID,
		Build:      snapshot.Build,
		BytesRead:  snapshot.BytesRead,
		Candidates: []PreviewCandidate{},
	}
	for _, candidate := range snapshot.Candidates {
		value, err := NormalizeCandidate(candidate)
		if err == nil {
			preview.Candidates = append(preview.Candidates, value)
		}
	}
	return preview
}

// NormalizeCandidate parses one NeXAS speaker-tagged memory string for display-only preview.
// It deliberately does not return dialogue.Dialogue because memory scan order is not story order.
func NormalizeCandidate(candidate Candidate) (PreviewCandidate, error) {
	raw := candidate.Raw
	if !utf8.ValidString(raw) || !strings.HasPrefix(raw, "【") {
		return PreviewCandidate{}, fmt.Errorf("candidate is not valid UTF-8 speaker text")
	}
	closeSpeaker := strings.Index(raw, "】")
	if closeSpeaker <= len("【") {
		return PreviewCandidate{}, fmt.Errorf("candidate speaker is missing")
	}
	speaker := raw[len("【"):closeSpeaker]
	content := raw[closeSpeaker+len("】"):]
	openQuote := strings.Index(content, "「")
	closeQuote := strings.LastIndex(content, "」")
	if openQuote < 0 || closeQuote <= openQuote+len("「") {
		return PreviewCandidate{}, fmt.Errorf("candidate dialogue quote is missing")
	}
	text := content[openQuote+len("「") : closeQuote]
	text = strings.ReplaceAll(text, "@n", "\n")
	if strings.TrimSpace(text) == "" {
		return PreviewCandidate{}, fmt.Errorf("candidate dialogue is empty")
	}
	return PreviewCandidate{Address: candidate.Address, Speaker: speaker, Text: text}, nil
}
