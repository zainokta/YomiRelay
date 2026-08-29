package translation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
)

const validSource = "カフェ。"
const validOutput = `{"translation":"The cafe.","segments":[{"text":"カフェ","kana":"かふぇ","meaning":"cafe"},{"text":"。","kana":"","meaning":""}]}`

func TestParseResultAcceptsExactSegments(t *testing.T) {
	got, err := ParseResult(validSource, []byte(validOutput))
	if err != nil {
		t.Fatal(err)
	}
	if got.Translation != "The cafe." || len(got.Segments) != 2 {
		t.Fatalf("result = %#v", got)
	}
}

func TestParseResultRejectsSourceMismatch(t *testing.T) {
	_, err := ParseResult(validSource, []byte(`{"translation":"The cafe.","segments":[{"text":"カフェ","kana":"かふぇ","meaning":"cafe"}]}`))
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
}

func TestParseResultRejectsMalformedAndUnglossedWordOutput(t *testing.T) {
	cases := []string{
		"not json",
		`{"translation":"","segments":[{"text":"カフェ","kana":"かふぇ","meaning":"cafe"}]}`,
		`{"translation":"Cafe.","segments":[]}`,
		`{"translation":"Cafe.","segments":[{"text":"カフェ。","kana":"","meaning":""}]}`,
	}
	for _, data := range cases {
		if _, err := ParseResult(validSource, []byte(data)); !errors.Is(err, ErrUnavailable) {
			t.Errorf("data %q error = %v, want ErrUnavailable", data, err)
		}
	}
}

func TestClientUsesRequestedModelAndCachesByGameAndText(t *testing.T) {
	fake := &fakeAppServer{responses: []string{validOutput}}
	client := newFakeClient(fake)

	if _, err := client.Translate(context.Background(), "111", validSource); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Translate(context.Background(), "111", validSource); err != nil {
		t.Fatal(err)
	}
	if fake.starts != 1 || fake.waits != 1 {
		t.Fatalf("app-server starts = %d, waits = %d, want 1 each", fake.starts, fake.waits)
	}
	if fake.threadParams["model"] != "gpt-5.6-luna" || fake.threadParams["serviceTier"] != "fast" || fake.threadParams["ephemeral"] != true {
		t.Fatalf("thread params = %#v", fake.threadParams)
	}
	if fake.turnParams["serviceTier"] != "fast" || fake.turnParams["effort"] != "low" {
		t.Fatalf("turn params = %#v", fake.turnParams)
	}
}

func TestClientRejectsOversizedSourceAndWrapsCommandFailure(t *testing.T) {
	client := New("codex")
	client.start = func(string) (appServerTransport, error) {
		return nil, errors.New("app-server unavailable")
	}
	if _, err := client.Translate(context.Background(), "111", strings.Repeat("日", MaxTextBytes+1)); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("oversized source error = %v", err)
	}
	if _, err := client.Translate(context.Background(), "111", validSource); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("command error = %v", err)
	}
}

func TestClientPassesCancellationToAppServer(t *testing.T) {
	fake := &fakeAppServer{waitForContext: true}
	client := newFakeClient(fake)
	called := make(chan struct{})
	fake.waitCalled = called
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_, _ = client.Translate(ctx, "111", validSource)
		close(done)
	}()
	<-called
	cancel()
	<-done
	fake.mu.Lock()
	closed := fake.closed
	fake.mu.Unlock()
	if closed != 1 {
		t.Fatalf("app-server close calls = %d, want 1", closed)
	}
}

type fakeAppServer struct {
	mu             sync.Mutex
	starts         int
	waits          int
	closed         int
	responses      []string
	waitForContext bool
	waitCalled     chan struct{}
	threadParams   map[string]any
	turnParams     map[string]any
}

func newFakeClient(fake *fakeAppServer) *Client {
	client := New("codex")
	client.start = func(string) (appServerTransport, error) {
		fake.mu.Lock()
		fake.starts++
		fake.mu.Unlock()
		return fake, nil
	}
	return client
}

func (f *fakeAppServer) call(ctx context.Context, method string, params any, result any) error {
	f.mu.Lock()
	switch method {
	case "initialize":
	case "thread/start":
		data, _ := json.Marshal(params)
		_ = json.Unmarshal(data, &f.threadParams)
		_ = json.Unmarshal([]byte(`{"thread":{"id":"thread-1"}}`), result)
	case "turn/start":
		data, _ := json.Marshal(params)
		_ = json.Unmarshal(data, &f.turnParams)
		_ = json.Unmarshal([]byte(`{"turn":{"id":"turn-1"}}`), result)
	default:
		f.mu.Unlock()
		return errors.New("unexpected method " + method)
	}
	f.mu.Unlock()
	return ctx.Err()
}

func (f *fakeAppServer) notify(string, any) error { return nil }

func (f *fakeAppServer) waitTurn(ctx context.Context, _, _ string) (string, error) {
	f.mu.Lock()
	f.waits++
	if f.waitCalled != nil {
		close(f.waitCalled)
		f.waitCalled = nil
	}
	waitForContext := f.waitForContext
	response := ""
	if len(f.responses) > 0 {
		response = f.responses[0]
		f.responses = f.responses[1:]
	}
	f.mu.Unlock()
	if waitForContext {
		<-ctx.Done()
		return "", ctx.Err()
	}
	return response, nil
}

func (f *fakeAppServer) close() error {
	f.mu.Lock()
	f.closed++
	f.mu.Unlock()
	return nil
}
