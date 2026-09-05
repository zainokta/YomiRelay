package nexas

import (
	"strings"
	"time"
)

const (
	settleWindow     = 75 * time.Millisecond
	progressWindow   = 300 * time.Millisecond
	duplicateWindow  = time.Second
	fragmentWindow   = 2 * time.Second
	recentKeepWindow = 10 * time.Second
)

type pendingLine struct {
	line     Line
	lastSeen time.Time
}

type recentLine struct {
	line Line
	seen time.Time
}

type lineCoalescer struct {
	pending     *pendingLine
	recent      map[string]time.Time
	recentLines []recentLine
}

func newLineCoalescer() *lineCoalescer {
	return &lineCoalescer{recent: make(map[string]time.Time)}
}

func (c *lineCoalescer) Add(line Line, now time.Time) []Line {
	c.prune(now)
	if c.isRecentTrailingFragment(line, now) {
		return nil
	}
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
	if c.isRecentTrailingFragment(line, now) {
		return ready
	}
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
	c.recentLines = append(c.recentLines, recentLine{line: line, seen: now})
	c.prune(now)
	return []Line{line}
}

func (c *lineCoalescer) isRecentTrailingFragment(line Line, now time.Time) bool {
	for i := len(c.recentLines) - 1; i >= 0; i-- {
		recent := c.recentLines[i]
		if now.Sub(recent.seen) > fragmentWindow {
			break
		}
		if recent.line.Speaker != line.Speaker || recent.line.Text == line.Text {
			continue
		}
		if len([]rune(line.Text)) < len([]rune(recent.line.Text)) && strings.HasSuffix(recent.line.Text, line.Text) {
			return true
		}
	}
	return false
}

func (c *lineCoalescer) prune(now time.Time) {
	for key, seen := range c.recent {
		if now.Sub(seen) > recentKeepWindow {
			delete(c.recent, key)
		}
	}
	cut := 0
	for cut < len(c.recentLines) && now.Sub(c.recentLines[cut].seen) > fragmentWindow {
		cut++
	}
	if cut > 0 {
		copy(c.recentLines, c.recentLines[cut:])
		c.recentLines = c.recentLines[:len(c.recentLines)-cut]
	}
}

func relatedText(left, right string) bool {
	return left == right ||
		strings.HasPrefix(left, right) || strings.HasPrefix(right, left) ||
		strings.HasSuffix(left, right) || strings.HasSuffix(right, left)
}
