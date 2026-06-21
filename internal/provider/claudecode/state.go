package claudecode

import (
	"time"

	"github.com/codebyNJ/minimo/internal/provider"
)

type sessionState struct {
	id         string
	cwd        string
	label      string
	startedAt  time.Time
	lastActive time.Time
	tokens     int
	cursor     tailCursor
}

func (s *sessionState) applyNew(data []byte) {
	for _, l := range parseLines(data) {
		if ts, ok := parseTimestamp(l.Timestamp); ok {
			if s.startedAt.IsZero() {
				s.startedAt = ts
			}
			s.lastActive = ts
		}
		if l.CWD != "" {
			s.cwd = l.CWD
		}
		if l.Type == "ai-title" && l.AITitle != "" {
			s.label = l.AITitle
		}
		if l.Type == "assistant" {
			u := l.Message.Usage
			s.tokens += u.InputTokens + u.OutputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
		}
	}
}

func (s *sessionState) info(providerName string, status provider.SessionStatus) provider.SessionInfo {
	return provider.SessionInfo{
		ID:         s.id,
		Provider:   providerName,
		CWD:        s.cwd,
		Label:      s.label,
		Status:     status,
		StartedAt:  s.startedAt,
		LastActive: s.lastActive,
	}
}
