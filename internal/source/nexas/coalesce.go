package nexas

import (
	"strings"
	"time"
)

const (
	settleWindow    = 75 * time.Millisecond
	progressWindow  = 300 * time.Millisecond
	duplicateWindow = time.Second
)

type pendingLine struct {
	line     Line
	lastSeen time.Time
}

type lineCoalescer struct {
	pending *pendingLine
	recent  map[string]time.Time
}

func newLineCoalescer() *lineCoalescer {
	return &lineCoalescer{recent: make(map[string]time.Time)}
}

func (c *lineCoalescer) Add(line Line, now time.Time) []Line {
	if c.pending == nil {
		c.pending = &pendingLine{line: line, lastSeen: now}
		return nil
	}
	if c.pending.line.Speaker == line.Speaker && now.Sub(c.pending.lastSeen) <= progressWindow && relatedText(c.pending.line.Text, line.Text) {
		if len([]rune(line.Text)) >= len([]rune(c.pending.line.Text)) {
			c.pending.line = line
		}
		c.pending.lastSeen = now
		return nil
	}
	ready := c.finish(now)
	c.pending = &pendingLine{line: line, lastSeen: now}
	return ready
}

func (c *lineCoalescer) FlushDue(now time.Time) []Line {
	if c.pending == nil || now.Sub(c.pending.lastSeen) < settleWindow {
		return nil
	}
	return c.finish(now)
}

func (c *lineCoalescer) finish(now time.Time) []Line {
	if c.pending == nil {
		return nil
	}
	line := c.pending.line
	c.pending = nil
	key := line.Speaker + "\x00" + line.Text
	if last, ok := c.recent[key]; ok && now.Sub(last) < duplicateWindow {
		return nil
	}
	c.recent[key] = now
	for key, seen := range c.recent {
		if now.Sub(seen) > 10*duplicateWindow {
			delete(c.recent, key)
		}
	}
	return []Line{line}
}

func relatedText(left, right string) bool {
	return left == right || strings.HasPrefix(left, right) || strings.HasPrefix(right, left)
}
