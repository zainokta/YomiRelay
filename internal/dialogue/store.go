package dialogue

import (
	"sync"
	"time"
)

// Dialogue is the normalized event shared by all backend consumers.
type Dialogue struct {
	Engine    string    `json:"engine,omitempty"`
	GameID    string    `json:"gameId"`
	GameName  string    `json:"gameName"`
	Speaker   string    `json:"speaker,omitempty"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
}

const defaultLimit = 1000

type Store struct {
	mu       sync.RWMutex
	limit    int
	entries  map[string][]Dialogue
	lastSeen map[string]time.Time
	now      func() time.Time
}

func NewStore(limit int, now func() time.Time) *Store {
	if limit <= 0 {
		limit = defaultLimit
	}
	if now == nil {
		now = time.Now
	}
	return &Store{limit: limit, entries: make(map[string][]Dialogue), lastSeen: make(map[string]time.Time), now: now}
}

func (s *Store) Append(d Dialogue) {
	s.mu.Lock()
	items := s.entries[d.GameID]
	items = append(items, d)
	if len(items) > s.limit {
		items = items[len(items)-s.limit:]
	}
	s.entries[d.GameID] = items
	s.lastSeen[d.GameID] = s.now()
	s.mu.Unlock()
}

func (s *Store) List(gameID string) []Dialogue {
	s.mu.RLock()
	source := s.entries[gameID]
	items := make([]Dialogue, len(source))
	copy(items, source)
	s.mu.RUnlock()
	return items
}

func (s *Store) Clear(gameID string) {
	s.mu.Lock()
	delete(s.entries, gameID)
	delete(s.lastSeen, gameID)
	s.mu.Unlock()
}

func (s *Store) Activity(gameID string) (lastSeen time.Time, active bool, known bool) {
	s.mu.RLock()
	lastSeen, known = s.lastSeen[gameID]
	s.mu.RUnlock()
	if !known {
		return time.Time{}, false, false
	}
	return lastSeen, s.now().Sub(lastSeen) < 30*time.Second, true
}
