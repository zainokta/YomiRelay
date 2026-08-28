package events

import (
	"sync"

	"yomirelay/internal/dialogue"
)

type subscription struct {
	mu     sync.Mutex
	ch     chan dialogue.Dialogue
	closed bool
}

func (s *subscription) close() {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		close(s.ch)
	}
	s.mu.Unlock()
}

func (s *subscription) send(value dialogue.Dialogue) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return true
	}
	select {
	case s.ch <- value:
		return true
	default:
		return false
	}
}

type Broker struct {
	mu          sync.Mutex
	nextID      uint64
	subscribers map[uint64]*subscription
	queueSize   int
}

func NewBroker(queueSize int) *Broker {
	if queueSize < 1 {
		queueSize = 1
	}
	return &Broker{queueSize: queueSize, subscribers: make(map[uint64]*subscription)}
}

func (b *Broker) Subscribe() (uint64, <-chan dialogue.Dialogue, func()) {
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	sub := &subscription{ch: make(chan dialogue.Dialogue, b.queueSize)}
	b.subscribers[id] = sub
	b.mu.Unlock()
	cancel := func() { b.remove(id, sub) }
	return id, sub.ch, cancel
}

func (b *Broker) remove(id uint64, sub *subscription) {
	b.mu.Lock()
	if current, ok := b.subscribers[id]; ok && current == sub {
		delete(b.subscribers, id)
	}
	b.mu.Unlock()
	sub.close()
}

func (b *Broker) Publish(value dialogue.Dialogue) {
	b.mu.Lock()
	subs := make([]struct {
		id  uint64
		sub *subscription
	}, 0, len(b.subscribers))
	for id, sub := range b.subscribers {
		subs = append(subs, struct {
			id  uint64
			sub *subscription
		}{id, sub})
	}
	b.mu.Unlock()
	for _, item := range subs {
		if !item.sub.send(value) {
			b.remove(item.id, item.sub)
		}
	}
}
