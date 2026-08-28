package receiver

import (
	"context"
	"net"
	"testing"
	"time"

	"yomirelay/internal/dialogue"
)

func TestListenReceivesValidPacketAndIgnoresMalformedPacket(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	received := make(chan string, 1)
	listener, err := Listen(ctx, "127.0.0.1:0", func(d dialogue.Dialogue) { received <- d.Text })
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	addr, err := net.ResolveUDPAddr("udp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("not json")); err != nil {
		t.Fatal(err)
	}
	valid := []byte(`{"v":1,"gameId":"111","gameName":"影の物語","text":"日本語","timestamp":1787895000}`)
	if _, err := conn.Write(valid); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-received:
		if got != "日本語" {
			t.Fatalf("received %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("valid packet not received")
	}
}
