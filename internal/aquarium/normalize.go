package aquarium

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"yomirelay/internal/dialogue"
)

// NormalizeCandidate converts one NeXAS speaker-tagged string for the debug publish path.
// It accepts only the structural form that includes a speaker and Japanese quote pair.
func NormalizeCandidate(raw string, timestamp time.Time) (dialogue.Dialogue, error) {
	if !utf8.ValidString(raw) || !strings.HasPrefix(raw, "【") {
		return dialogue.Dialogue{}, fmt.Errorf("candidate is not valid UTF-8 speaker text")
	}
	closeSpeaker := strings.Index(raw, "】")
	if closeSpeaker <= len("【") {
		return dialogue.Dialogue{}, fmt.Errorf("candidate speaker is missing")
	}
	speaker := raw[len("【"):closeSpeaker]
	content := raw[closeSpeaker+len("】"):]
	openQuote := strings.Index(content, "「")
	closeQuote := strings.LastIndex(content, "」")
	if openQuote < 0 || closeQuote <= openQuote+len("「") {
		return dialogue.Dialogue{}, fmt.Errorf("candidate dialogue quote is missing")
	}
	text := content[openQuote+len("「") : closeQuote]
	text = strings.ReplaceAll(text, "@n", "\n")
	if strings.TrimSpace(text) == "" {
		return dialogue.Dialogue{}, fmt.Errorf("candidate dialogue is empty")
	}
	return dialogue.Dialogue{Engine: "nexas", Speaker: speaker, Text: text, Timestamp: timestamp}, nil
}
