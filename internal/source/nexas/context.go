package nexas

import (
	"sort"
	"strings"
	"time"
)

const (
	maxRenderContexts             = 64
	renderContextPendingLimit     = 6
	renderContextCandidateScore   = 3
	renderContextSelectionScore   = 6
	renderContextMinObservations  = 2
)

type renderContextState struct {
	coalescer    *lineCoalescer
	pending      []Line
	seen         map[string]struct{}
	score        int
	observations int
	lastSeen     time.Time
}

type renderContextFilter struct {
	selected uint64
	contexts map[uint64]*renderContextState
	onSelect func(uint64)
}

func newRenderContextFilter(onSelect func(uint64)) *renderContextFilter {
	return &renderContextFilter{contexts: make(map[uint64]*renderContextState), onSelect: onSelect}
}

func (f *renderContextFilter) Add(contextID uint64, line Line, now time.Time) []Line {
	if contextID == 0 {
		return nil
	}
	if f.selected != 0 && contextID != f.selected {
		return nil
	}
	state := f.stateFor(contextID, now)
	if state == nil {
		return nil
	}
	return f.consume(contextID, state, state.coalescer.Add(line, now))
}

func (f *renderContextFilter) FlushDue(now time.Time) []Line {
	if f.selected != 0 {
		state := f.contexts[f.selected]
		if state == nil {
			return nil
		}
		return f.consume(f.selected, state, state.coalescer.FlushDue(now))
	}
	ids := make([]uint64, 0, len(f.contexts))
	for id := range f.contexts {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var result []Line
	for _, id := range ids {
		state := f.contexts[id]
		result = append(result, f.consume(id, state, state.coalescer.FlushDue(now))...)
		if f.selected != 0 {
			break
		}
	}
	return result
}

func (f *renderContextFilter) stateFor(contextID uint64, now time.Time) *renderContextState {
	if state, ok := f.contexts[contextID]; ok {
		state.lastSeen = now
		return state
	}
	if len(f.contexts) >= maxRenderContexts {
		var oldestID uint64
		var oldest time.Time
		for id, state := range f.contexts {
			if oldestID == 0 || state.lastSeen.Before(oldest) {
				oldestID = id
				oldest = state.lastSeen
			}
		}
		if oldestID != 0 {
			delete(f.contexts, oldestID)
		}
	}
	state := &renderContextState{
		coalescer: newLineCoalescer(),
		seen:      make(map[string]struct{}),
		lastSeen:  now,
	}
	f.contexts[contextID] = state
	return state
}

func (f *renderContextFilter) consume(contextID uint64, state *renderContextState, ready []Line) []Line {
	if len(ready) == 0 {
		return nil
	}
	if f.selected != 0 {
		if contextID == f.selected {
			return ready
		}
		return nil
	}

	for _, line := range ready {
		state.pending = append(state.pending, line)
		if len(state.pending) > renderContextPendingLimit {
			copy(state.pending, state.pending[len(state.pending)-renderContextPendingLimit:])
			state.pending = state.pending[:renderContextPendingLimit]
		}

		likelihood := dialogueLikelihood(line)
		if likelihood < renderContextCandidateScore {
			continue
		}
		key := line.Speaker + "\x00" + line.Text
		if _, ok := state.seen[key]; ok {
			continue
		}
		state.seen[key] = struct{}{}
		state.score += likelihood
		state.observations++
		if state.observations < renderContextMinObservations || state.score < renderContextSelectionScore {
			continue
		}

		f.selected = contextID
		pending := append([]Line(nil), state.pending...)
		f.contexts = map[uint64]*renderContextState{contextID: state}
		if f.onSelect != nil {
			f.onSelect(contextID)
		}
		return pending
	}
	return nil
}

func dialogueLikelihood(line Line) int {
	text := strings.TrimSpace(line.Text)
	if text == "" {
		return 0
	}
	length := len([]rune(text))
	punctuated := hasSentencePunctuation(text)
	if line.Speaker == "" && length < 10 && !punctuated {
		return 0
	}

	score := 0
	if line.Speaker != "" {
		score += 4
	}
	switch {
	case length >= 28:
		score += 4
	case length >= 16:
		score += 3
	case length >= 10:
		score += 2
	case length >= 7:
		score++
	}
	if punctuated {
		score += 2
	}
	return score
}

func hasSentencePunctuation(text string) bool {
	return strings.ContainsAny(text, "。！？!?…‥」』")
}
