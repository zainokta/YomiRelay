package receiver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"yomirelay/internal/dialogue"
)

const MaxDatagramSize = 8192

type packet struct {
	Version   int64  `json:"v"`
	GameID    string `json:"gameId"`
	GameName  string `json:"gameName"`
	Speaker   string `json:"speaker"`
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}

func ParsePacket(data []byte) (dialogue.Dialogue, error) {
	if len(data) > MaxDatagramSize {
		return dialogue.Dialogue{}, fmt.Errorf("packet exceeds %d bytes", MaxDatagramSize)
	}
	if !utf8.Valid(data) {
		return dialogue.Dialogue{}, fmt.Errorf("packet is not valid UTF-8")
	}
	var value packet
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&value); err != nil {
		return dialogue.Dialogue{}, fmt.Errorf("decode packet: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return dialogue.Dialogue{}, fmt.Errorf("packet contains trailing JSON")
		}
		return dialogue.Dialogue{}, fmt.Errorf("decode trailing packet data: %w", err)
	}
	if value.Version != 1 {
		return dialogue.Dialogue{}, fmt.Errorf("unsupported packet version %d", value.Version)
	}
	if value.GameID == "" || strings.TrimSpace(value.GameID) == "" {
		return dialogue.Dialogue{}, fmt.Errorf("gameId is required")
	}
	if value.GameName == "" || strings.TrimSpace(value.GameName) == "" {
		return dialogue.Dialogue{}, fmt.Errorf("gameName is required")
	}
	if value.Text == "" || strings.TrimSpace(value.Text) == "" {
		return dialogue.Dialogue{}, fmt.Errorf("text is required")
	}
	if value.Timestamp < 1 {
		return dialogue.Dialogue{}, fmt.Errorf("timestamp must be a positive Unix second")
	}
	return dialogue.Dialogue{
		GameID: value.GameID, GameName: value.GameName, Speaker: value.Speaker,
		Text: value.Text, Timestamp: time.Unix(value.Timestamp, 0),
	}, nil
}
