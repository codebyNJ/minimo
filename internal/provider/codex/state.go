package codex

import (
	"encoding/json"
	"time"

	"github.com/codebyNJ/minimo/internal/provider"
	"github.com/codebyNJ/minimo/internal/tailreader"
)

type sessionState struct {
	id                  string
	cwd                 string
	model               string
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
	cursor              tailreader.Cursor
}

func (s *sessionState) applyNew(data []byte) {
	for _, l := range parseRolloutLines(data) {
		if ts, ok := provider.ParseTimestamp(l.Timestamp); ok {
			if s.startedAt.IsZero() {
				s.startedAt = ts
			}
			s.lastActive = ts
		}

		switch l.Type {
		case "session_meta":
			var meta sessionMetaPayload
			if err := json.Unmarshal(l.Payload, &meta); err != nil {
				continue
			}
			if meta.CWD != "" {
				s.cwd = meta.CWD
			}
		case "turn_context":
			var tc turnContextPayload
			if err := json.Unmarshal(l.Payload, &tc); err != nil {
				continue
			}
			if tc.CWD != "" {
				s.cwd = tc.CWD
			}
			if tc.Model != "" {
				s.model = tc.Model
			}
		case "event_msg":
			var em eventMsgPayload
			if err := json.Unmarshal(l.Payload, &em); err != nil {
				continue
			}
			if em.Type != "token_count" || em.Info == nil {
				continue
			}
			s.tokens = int(em.Info.TotalTokenUsage.TotalTokens)
			tot := em.Info.TotalTokenUsage
			s.inputTokens = int(tot.InputTokens)
			s.cacheReadTokens = int(tot.CachedInputTokens)
			s.outputTokens = int(tot.OutputTokens + tot.ReasoningOutputTokens)
			// Codex reports no cache-creation category; leave it zero.
			last := em.Info.LastTokenUsage
			s.contextTokens = int(last.InputTokens + last.CachedInputTokens)
			if em.Info.ModelContextWindow != nil {
				s.contextLimit = int(*em.Info.ModelContextWindow)
				s.contextKnown = true
			}
		}
	}
}

func (s *sessionState) info(providerName string, status provider.SessionStatus) provider.SessionInfo {
	return provider.SessionInfo{
		ID:         s.id,
		Provider:   providerName,
		CWD:        s.cwd,
		Model:      s.model,
		Status:     status,
		StartedAt:  s.startedAt,
		LastActive: s.lastActive,
	}
}
