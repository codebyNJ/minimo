package claudecode

import (
	"sort"
	"time"

	"github.com/codebyNJ/minimo/internal/provider"
	"github.com/codebyNJ/minimo/internal/tailreader"
)

type sessionState struct {
	id                  string
	cwd                 string
	label               string
	model               string
	startedAt           time.Time
	lastActive          time.Time
	tokens              int
	inputTokens         int
	outputTokens        int
	cacheReadTokens     int
	cacheCreationTokens int
	contextTokens       int
	files               map[string]struct{}
	cursor              tailreader.Cursor
	// subCursors tracks one incremental cursor per subagent transcript
	// (<session>/subagents/*.jsonl) so their token usage is folded into this
	// session's totals without re-reading the whole file every poll.
	subCursors map[string]*tailreader.Cursor
}

var fileTools = map[string]bool{"Read": true, "Edit": true, "Write": true}

func (s *sessionState) applyNew(data []byte) {
	for _, l := range parseLines(data) {
		if ts, ok := provider.ParseTimestamp(l.Timestamp); ok {
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
			s.inputTokens += u.InputTokens
			s.outputTokens += u.OutputTokens
			s.cacheReadTokens += u.CacheReadInputTokens
			s.cacheCreationTokens += u.CacheCreationInputTokens
			s.contextTokens = u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
			if l.Message.Model != "" {
				s.model = l.Message.Model
			}
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

// applySubagentTokens folds a subagent sub-transcript's assistant-turn token
// usage into this session's totals. It intentionally ignores model, cwd,
// label, timestamps, files, and contextTokens — those describe the parent
// conversation; a subagent only adds token (and therefore cost) volume that
// would otherwise be undercounted.
func (s *sessionState) applySubagentTokens(data []byte) {
	for _, l := range parseLines(data) {
		if l.Type != "assistant" {
			continue
		}
		u := l.Message.Usage
		s.tokens += u.InputTokens + u.OutputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
		s.inputTokens += u.InputTokens
		s.outputTokens += u.OutputTokens
		s.cacheReadTokens += u.CacheReadInputTokens
		s.cacheCreationTokens += u.CacheCreationInputTokens
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
		Model:      s.model,
		Status:     status,
		StartedAt:  s.startedAt,
		LastActive: s.lastActive,
	}
}
