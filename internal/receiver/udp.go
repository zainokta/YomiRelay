package receiver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"

	"yomirelay/internal/dialogue"
)

type Listener struct {
	conn     *net.UDPConn
	ctx      context.Context
	done     chan struct{}
	err      error
	errMu    sync.Mutex
	closeOne sync.Once
}

func Listen(ctx context.Context, addr string, onDialogue func(dialogue.Dialogue)) (*Listener, error) {
	resolved, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("resolve UDP address: %w", err)
	}
	if resolved.IP == nil || !resolved.IP.IsLoopback() {
		return nil, fmt.Errorf("UDP address must be loopback")
	}
	conn, err := net.ListenUDP("udp", resolved)
	if err != nil {
		return nil, fmt.Errorf("listen for UDP: %w", err)
	}
	listener := &Listener{conn: conn, ctx: ctx, done: make(chan struct{})}
	go listener.readLoop(onDialogue)
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	return listener, nil
}

func (l *Listener) Addr() net.Addr { return l.conn.LocalAddr() }

func (l *Listener) Wait() error {
	<-l.done
	l.errMu.Lock()
	err := l.err
	l.errMu.Unlock()
	return err
}

func (l *Listener) Close() error {
	var err error
	l.closeOne.Do(func() { err = l.conn.Close() })
	return err
}

func (l *Listener) readLoop(onDialogue func(dialogue.Dialogue)) {
	defer close(l.done)
	buffer := make([]byte, MaxDatagramSize+1)
	for {
		n, _, err := l.conn.ReadFromUDP(buffer)
		if err != nil {
			if errors.Is(err, net.ErrClosed) || l.ctx.Err() != nil {
				return
			}
			l.errMu.Lock()
			l.err = err
			l.errMu.Unlock()
			return
		}
		if n > MaxDatagramSize {
			log.Printf("receiver: discarded oversized UDP packet")
			continue
		}
		value, err := ParsePacket(buffer[:n])
		if err != nil {
			log.Printf("receiver: discarded invalid UDP packet: %v", err)
			continue
		}
		if onDialogue != nil {
			onDialogue(value)
		}
	}
}
