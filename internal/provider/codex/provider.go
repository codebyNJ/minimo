package codex

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/codebyNJ/minimo/internal/provider"
	"github.com/codebyNJ/minimo/internal/tailreader"
)

func init() {
	provider.Register(New())
}

type CodexProvider struct {
	mu       sync.Mutex
	sessions map[string]*sessionState
}

func New() *CodexProvider {
	return &CodexProvider{sessions: make(map[string]*sessionState)}
}

func (p *CodexProvider) Name() string { return "codex" }

func (p *CodexProvider) home() string {
	if override, ok := provider.PathOverride(p.Name()); ok {
		return override
	}
	if env := os.Getenv("CODEX_HOME"); env != "" {
		return env
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}
	return filepath.Join(homeDir, ".codex")
}

func (p *CodexProvider) sessionsDir() string { return filepath.Join(p.home(), "sessions") }
func (p *CodexProvider) archivedDir() string { return filepath.Join(p.home(), "archived_sessions") }

func (p *CodexProvider) CheckedPath() string { return p.home() }

func (p *CodexProvider) WatchPaths() []string {
	return []string{p.sessionsDir(), p.archivedDir()}
}

func (p *CodexProvider) Detect() bool {
	info, err := os.Stat(p.sessionsDir())
	return err == nil && info.IsDir()
}

// findRollouts walks both the active and archived trees, since a
// long-time Codex user's history is mostly archived and a provider that
// only scans sessions/ would silently miss it.
func (p *CodexProvider) findRollouts() []string {
	var out []string
	for _, root := range []string{p.sessionsDir(), p.archivedDir()} {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() || !strings.HasPrefix(d.Name(), "rollout-") || filepath.Ext(d.Name()) != ".jsonl" {
				return nil
			}
			out = append(out, path)
			return nil
		})
	}
	return out
}

func sessionIDFromPath(path string) string {
	name := filepath.Base(path)
	name = strings.TrimPrefix(name, "rollout-")
	return strings.TrimSuffix(name, ".jsonl")
}

func (p *CodexProvider) statusFor(s *sessionState) provider.SessionStatus {
	if !s.lastActive.IsZero() && time.Since(s.lastActive) < provider.IdleThreshold {
		return provider.StatusIdle
	}
	return provider.StatusEnded
}

func (p *CodexProvider) ListSessions() ([]provider.SessionInfo, error) {
	paths := p.findRollouts()

	p.mu.Lock()
	defer p.mu.Unlock()

	var out []provider.SessionInfo
	for _, path := range paths {
		id := sessionIDFromPath(path)
		state, ok := p.sessions[id]
		if !ok {
			state = &sessionState{id: id, cursor: tailreader.Cursor{Path: path}}
			p.sessions[id] = state
		}
		out = append(out, state.info(p.Name(), p.statusFor(state)))
	}
	return out, nil
}

func (p *CodexProvider) ReadContext(sessionID string) (*provider.SessionContext, error) {
	p.mu.Lock()
	state, ok := p.sessions[sessionID]
	p.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("codex: unknown session %q (call ListSessions first)", sessionID)
	}

	data, err := state.cursor.ReadNew()
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	state.applyNew(data)
	return &provider.SessionContext{
		Session: state.info(p.Name(), p.statusFor(state)),
		Tokens: provider.TokenUsage{
			Total:         state.tokens,
			Input:         state.inputTokens,
			Output:        state.outputTokens,
			CacheRead:     state.cacheReadTokens,
			CacheCreation: state.cacheCreationTokens,
			Source:        provider.TokenSourceExact,
		},
		Context: provider.ContextUsage{
			Tokens: state.contextTokens,
			Known:  state.contextKnown,
			Limit:  state.contextLimit,
		},
	}, nil
}
