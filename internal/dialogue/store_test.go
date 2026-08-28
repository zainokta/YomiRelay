package dialogue

import (
	"fmt"
	"testing"
	"time"
)

func TestStoreBoundsHistoryPerGame(t *testing.T) {
	now := time.Unix(100, 0)
	store := NewStore(1000, func() time.Time { return now })
	for i := 1; i <= 1001; i++ {
		store.Append(Dialogue{GameID: "111", GameName: "A", Text: fmt.Sprintf("%d", i)})
	}
	store.Append(Dialogue{GameID: "222", GameName: "B", Text: "only"})
	got := store.List("111")
	if len(got) != 1000 || got[0].Text != "2" || got[len(got)-1].Text != "1001" {
		t.Fatalf("history bounds = len %d, first %q, last %q", len(got), got[0].Text, got[len(got)-1].Text)
	}
	if other := store.List("222"); len(other) != 1 || other[0].Text != "only" {
		t.Fatalf("other history = %#v", other)
	}
}

func TestStoreClearDoesNotAffectOtherGame(t *testing.T) {
	store := NewStore(10, time.Now)
	store.Append(Dialogue{GameID: "111", Text: "a"})
	store.Append(Dialogue{GameID: "222", Text: "b"})
	store.Clear("111")
	if got := store.List("111"); len(got) != 0 {
		t.Fatalf("cleared history = %#v", got)
	}
	if got := store.List("222"); len(got) != 1 {
		t.Fatalf("other history = %#v", got)
	}
}

func TestStoreActivityExpiresAfterThirtySeconds(t *testing.T) {
	now := time.Unix(1000, 0)
	store := NewStore(10, func() time.Time { return now })
	store.Append(Dialogue{GameID: "111", Text: "a"})
	last, active, known := store.Activity("111")
	if !known || !active || !last.Equal(now) {
		t.Fatalf("activity = %v, %v, %v", last, active, known)
	}
	now = now.Add(30 * time.Second)
	_, active, known = store.Activity("111")
	if !known || active {
		t.Fatalf("expired activity = active %v known %v", active, known)
	}
	_, _, known = store.Activity("missing")
	if known {
		t.Fatal("missing game reported known")
	}
}
