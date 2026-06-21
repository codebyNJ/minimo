package claudecode

import (
	"sort"
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
	files      map[string]struct{}
	cursor     tailCursor
}

var fileTools = map[string]bool{"Read": true, "Edit": true, "Write": true}

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
			for _, block := range l.Message.Content {
				if block.Type == "tool_use" && fileTools[block.Name] && block.Input.FilePath != "" {
					if s.files == nil {
						s.files = make(map[string]struct{})
					}
					s.files[block.Input.FilePath] = struct{}{}
				}
			}
		}
	}
}

func (s *sessionState) fileRefs() []provider.FileRef {
	paths := make([]string, 0, len(s.files))
	for p := range s.files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	out := make([]provider.FileRef, 0, len(paths))
	for _, p := range paths {
		out = append(out, provider.FileRef{Path: p})
	}
	return out
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
