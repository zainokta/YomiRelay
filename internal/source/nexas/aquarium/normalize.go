package aquarium

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Line struct {
	Speaker string
	Text    string
}

// NormalizeHookText converts the UTF-8 string observed at the verified NeXAS
// hook point into Reader-ready dialogue. Unknown control tags fail closed so a
// new engine tag cannot silently become visible garbage.
func NormalizeHookText(raw string) (Line, error) {
	if !utf8.ValidString(raw) {
		return Line{}, fmt.Errorf("hook text is not valid UTF-8")
	}
	clean, err := stripControlTags(raw)
	if err != nil {
		return Line{}, err
	}
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return Line{}, fmt.Errorf("hook text is empty")
	}

	line := Line{}
	if strings.HasPrefix(clean, "【") {
		end := strings.Index(clean, "】")
		if end <= len("【") {
			return Line{}, fmt.Errorf("speaker tag is malformed")
		}
		line.Speaker = strings.TrimSpace(clean[len("【"):end])
		if line.Speaker == "選択肢" {
			return Line{}, fmt.Errorf("choice text is not dialogue")
		}
		clean = strings.TrimSpace(clean[end+len("】"):])
	}

	if text, ok := quoted(clean, "「", "」"); ok {
		clean = text
	} else if text, ok := quoted(clean, "『", "』"); ok {
		clean = text
	}
	clean = strings.TrimSpace(clean)
	if clean == "" || !containsJapanese(clean) {
		return Line{}, fmt.Errorf("hook text does not contain Japanese dialogue")
	}
	line.Text = clean
	return line, nil
}

func stripControlTags(raw string) (string, error) {
	var out strings.Builder
	out.Grow(len(raw))
	for i := 0; i < len(raw); {
		if raw[i] != '@' {
			out.WriteByte(raw[i])
			i++
			continue
		}
		if i+2 > len(raw) {
			return "", fmt.Errorf("truncated NeXAS control tag")
		}
		switch raw[i+1] {
		case 'n':
			out.WriteByte('\n')
			i += 2
		case 'p', 'k':
			i += 2
		case 'v':
			if i+7 > len(raw) {
				return "", fmt.Errorf("truncated NeXAS voice tag")
			}
			for _, ch := range raw[i+2 : i+7] {
				if ch < '0' || ch > '9' {
					return "", fmt.Errorf("invalid NeXAS voice tag")
				}
			}
			i += 7
		default:
			return "", fmt.Errorf("unsupported NeXAS control tag @%c", raw[i+1])
		}
	}
	return out.String(), nil
}

func quoted(text, open, close string) (string, bool) {
	start := strings.Index(text, open)
	end := strings.LastIndex(text, close)
	if start < 0 || end <= start+len(open) {
		return "", false
	}
	return text[start+len(open) : end], true
}

func containsJapanese(text string) bool {
	for _, r := range text {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana) {
			return true
		}
	}
	return false
}
