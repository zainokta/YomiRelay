package events

import (
	"testing"
	"time"

	"yomirelay/internal/dialogue"
)

func TestBrokerFansOutInOrder(t *testing.T) {
	broker := NewBroker(2)
	_, first, cancelFirst := broker.Subscribe()
	_, second, cancelSecond := broker.Subscribe()
	defer cancelFirst()
	defer cancelSecond()
	want := dialogue.Dialogue{GameID: "111", Text: "日本語"}
	broker.Publish(want)
	for name, stream := range map[string]<-chan dialogue.Dialogue{"first": first, "second": second} {
		select {
		case got := <-stream:
			if got != want {
				t.Errorf("%s = %#v, want %#v", name, got, want)
			}
		case <-time.After(time.Second):
			t.Errorf("%s timed out", name)
		}
	}
}

func TestBrokerDropsFullSubscriberWithoutBlockingPublisher(t *testing.T) {
	broker := NewBroker(1)
	_, stream, cancel := broker.Subscribe()
	start := time.Now()
	done := make(chan struct{})
	go func() {
		broker.Publish(dialogue.Dialogue{Text: "one"})
		broker.Publish(dialogue.Dialogue{Text: "two"})
		broker.Publish(dialogue.Dialogue{Text: "three"})
		close(done)
	}()
	select {
	case <-done:
		if elapsed := time.Since(start); elapsed >= 100*time.Millisecond {
			t.Fatalf("publication took %v", elapsed)
		}
	case <-time.After(time.Second):
		t.Fatal("publication blocked")
	}
	select {
	case _, ok := <-stream:
		if ok {
			// The first value may remain buffered before overflow closes the stream.
			if _, ok = <-stream; ok {
				t.Fatal("subscriber remained open after overflow")
			}
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber was not closed")
	}
	cancel()
	cancel()
}
