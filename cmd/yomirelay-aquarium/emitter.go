package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"yomirelay/internal/source/aquarium"
)

type udpEmitter struct {
	conn     *net.UDPConn
	gameName string
}

type dialoguePacket struct {
	Engine    string `json:"engine"`
	Version   int    `json:"v"`
	GameID    string `json:"gameId"`
	GameName  string `json:"gameName"`
	Speaker   string `json:"speaker,omitempty"`
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}

func newUDPEmitter(address, gameName string) (*udpEmitter, error) {
	resolved, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}
	if resolved.IP == nil || !resolved.IP.IsLoopback() {
		return nil, fmt.Errorf("UDP destination must be loopback: %s", address)
	}
	conn, err := net.DialUDP("udp", nil, resolved)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(gameName) == "" {
		gameName = "AQUARIUM"
	}
	return &udpEmitter{conn: conn, gameName: gameName}, nil
}

func (e *udpEmitter) Close() error { return e.conn.Close() }

func (e *udpEmitter) Emit(line aquarium.Line, observed time.Time) {
	packet := dialoguePacket{Engine: "nexas", Version: 1, GameID: aquarium.AppID, GameName: e.gameName, Speaker: line.Speaker, Text: line.Text, Timestamp: observed.Unix()}
	data, err := json.Marshal(packet)
	if err != nil {
		return
	}
	_, _ = e.conn.Write(data)
}
