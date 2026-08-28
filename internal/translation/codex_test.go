package translation

import (
	"context"
	"errors"
	"strings"
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
	calls := 0
	var gotArgs []string
	client := New("codex")
	client.run = func(_ context.Context, _ string, args []string, _ string, prompt string) ([]byte, error) {
		calls++
		gotArgs = append([]string(nil), args...)
		if !strings.Contains(prompt, validSource) {
			t.Fatalf("prompt does not contain source: %q", prompt)
		}
		return []byte(validOutput), nil
	}

	if _, err := client.Translate(context.Background(), "111", validSource); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Translate(context.Background(), "111", validSource); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("Codex calls = %d, want 1", calls)
	}
	joined := strings.Join(gotArgs, " ")
	for _, required := range []string{"exec", "--ephemeral", "--sandbox", "read-only", "--model", "gpt-5.6-luna", "--skip-git-repo-check", "--color", "never", "-"} {
		if !strings.Contains(joined, required) {
			t.Errorf("args %v missing %q", gotArgs, required)
		}
	}
}

func TestClientRejectsOversizedSourceAndWrapsCommandFailure(t *testing.T) {
	client := New("codex")
	client.run = func(context.Context, string, []string, string, string) ([]byte, error) {
		return nil, errors.New("exit status 1")
	}
	if _, err := client.Translate(context.Background(), "111", strings.Repeat("日", MaxTextBytes+1)); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("oversized source error = %v", err)
	}
	if _, err := client.Translate(context.Background(), "111", validSource); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("command error = %v", err)
	}
}

func TestClientPassesCancellationToCommand(t *testing.T) {
	client := New("codex")
	called := make(chan struct{})
	client.run = func(ctx context.Context, _ string, _ []string, _ string, _ string) ([]byte, error) {
		close(called)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	go func() { _, _ = client.Translate(ctx, "111", validSource) }()
	<-called
}
