package kimicode

import "time"

type sessionState struct {
	startedAt           time.Time
	lastActive          time.Time
	tokens              int
	inputTokens         int
	outputTokens        int
	cacheReadTokens     int
	cacheCreationTokens int
	contextTokens       int
	contextLimit        int
	contextKnown        bool
}

func (s *sessionState) applyNew(lines []wireLine) {
	for _, l := range lines {
		if !l.usable() {
			continue
		}
		if u := l.usage(); u != nil {
			s.tokens = int(u.total())
			s.inputTokens = int(u.InputOther)
			s.outputTokens = int(u.Output)
			s.cacheReadTokens = int(u.InputCacheRead)
			s.cacheCreationTokens = int(u.InputCacheCreation)
		}
		if ct := l.contextTokens(); ct != nil {
			s.contextTokens = int(*ct)
		}
		if mc := l.maxContextTokens(); mc != nil {
			s.contextLimit = int(*mc)
			s.contextKnown = true
		}
	}
}
