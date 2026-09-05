package main

import (
	"testing"
	"time"

	"yomirelay/internal/source/aquarium"
)

func TestLineCoalescerKeepsLongestProgressiveText(t *testing.T) {
	c := newLineCoalescer()
	now := time.Unix(1, 0)
	for i, text := range []string{"お", "おは", "おはよう"} {
		if got := c.Add(aquarium.Line{Speaker: "湊あくあ", Text: text}, now.Add(time.Duration(i)*20*time.Millisecond)); len(got) != 0 {
			t.Fatalf("unexpected early output: %#v", got)
		}
	}
	got := c.FlushDue(now.Add(200 * time.Millisecond))
	if len(got) != 1 || got[0].Text != "おはよう" {
		t.Fatalf("got %#v", got)
	}
}

func TestLineCoalescerAllowsSameLineLater(t *testing.T) {
	c := newLineCoalescer()
	now := time.Unix(1, 0)
	line := aquarium.Line{Text: "同じ台詞"}
	c.Add(line, now)
	if got := c.FlushDue(now.Add(100 * time.Millisecond)); len(got) != 1 {
		t.Fatalf("first = %#v", got)
	}
	c.Add(line, now.Add(2*time.Second))
	if got := c.FlushDue(now.Add(2200 * time.Millisecond)); len(got) != 1 {
		t.Fatalf("repeat = %#v", got)
	}
}
