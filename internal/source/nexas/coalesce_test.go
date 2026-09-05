package nexas

import (
	"testing"
	"time"
)

func TestLineCoalescerKeepsLongestProgressiveText(t *testing.T) {
	c := newLineCoalescer()
	now := time.Unix(1, 0)
	for i, text := range []string{"お", "おは", "おはよう"} {
		if got := c.Add(Line{Speaker: "湊あくあ", Text: text}, now.Add(time.Duration(i)*20*time.Millisecond)); len(got) != 0 {
			t.Fatalf("unexpected early output: %#v", got)
		}
	}
	got := c.FlushDue(now.Add(200 * time.Millisecond))
	if len(got) != 1 || got[0].Text != "おはよう" {
		t.Fatalf("got %#v", got)
	}
}

func TestLineCoalescerCollapsesShrinkingSuffixes(t *testing.T) {
	c := newLineCoalescer()
	now := time.Unix(1, 0)
	for i, text := range []string{
		"あなたの恋人になろうとしたばっかりに",
		"なたの恋人になろうとしたばっかりに",
		"たの恋人になろうとしたばっかりに",
		"の恋人になろうとしたばっかりに",
	} {
		if got := c.Add(Line{Text: text}, now.Add(time.Duration(i)*20*time.Millisecond)); len(got) != 0 {
			t.Fatalf("unexpected early output: %#v", got)
		}
	}
	got := c.FlushDue(now.Add(250 * time.Millisecond))
	if len(got) != 1 || got[0].Text != "あなたの恋人になろうとしたばっかりに" {
		t.Fatalf("got %#v", got)
	}
}

func TestLineCoalescerSuppressesLateShrinkingFragment(t *testing.T) {
	c := newLineCoalescer()
	now := time.Unix(1, 0)
	full := Line{Text: "あなたの恋人になろうとしたばっかりに"}
	c.Add(full, now)
	if got := c.FlushDue(now.Add(100 * time.Millisecond)); len(got) != 1 || got[0] != full {
		t.Fatalf("full = %#v", got)
	}
	if got := c.Add(Line{Text: "なたの恋人になろうとしたばっかりに"}, now.Add(500*time.Millisecond)); len(got) != 0 {
		t.Fatalf("fragment output = %#v", got)
	}
	if got := c.FlushDue(now.Add(700 * time.Millisecond)); len(got) != 0 {
		t.Fatalf("fragment flush = %#v", got)
	}
}

func TestLineCoalescerAllowsSameLineLater(t *testing.T) {
	c := newLineCoalescer()
	now := time.Unix(1, 0)
	line := Line{Text: "同じ台詞"}
	c.Add(line, now)
	if got := c.FlushDue(now.Add(100 * time.Millisecond)); len(got) != 1 {
		t.Fatalf("first = %#v", got)
	}
	c.Add(line, now.Add(2*time.Second))
	if got := c.FlushDue(now.Add(2200 * time.Millisecond)); len(got) != 1 {
		t.Fatalf("repeat = %#v", got)
	}
}
