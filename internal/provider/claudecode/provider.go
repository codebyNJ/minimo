package claudecode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/codebyNJ/minimo/internal/provider"
	"github.com/codebyNJ/minimo/internal/tailreader"
)

const idleThreshold = 30 * time.Second

func init() {
	provider.Register(New())
}

type ClaudeCodeProvider struct {
	mu       sync.Mutex
	sessions map[string]*sessionState
}

func New() *ClaudeCodeProvider {
	return &ClaudeCodeProvider{sessions: make(map[string]*sessionState)}
}

func (p *ClaudeCodeProvider) Name() string { return "claude-code" }

func (p *ClaudeCodeProvider) home() string {
	if override, ok := provider.PathOverride(p.Name()); ok {
		return override
	}
	if env := os.Getenv("CLAUDE_CONFIG_DIR"); env != "" {
		return env
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}
	return filepath.Join(homeDir, ".claude")
}

func (p *ClaudeCodeProvider) projectsDir() string { return filepath.Join(p.home(), "projects") }
func (p *ClaudeCodeProvider) liveDir() string      { return filepath.Join(p.home(), "sessions") }

func (p *ClaudeCodeProvider) CheckedPath() string { return p.projectsDir() }

func (p *ClaudeCodeProvider) WatchPaths() []string { return []string{p.projectsDir()} }

func (p *ClaudeCodeProvider) Detect() bool {
	info, err := os.Stat(p.projectsDir())
	return err == nil && info.IsDir()
}

type liveEntry struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
}

func (p *ClaudeCodeProvider) loadLiveRegistry() map[string]liveEntry {
	out := make(map[string]liveEntry)
	entries, err := os.ReadDir(p.liveDir())
	if err != nil {
		return out
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(p.liveDir(), e.Name()))
		if err != nil {
			continue
		}
		var le liveEntry
		if err := json.Unmarshal(data, &le); err != nil {
			continue
		}
		out[le.SessionID] = le
	}
	return out
}

func (p *ClaudeCodeProvider) statusFor(id string, s *sessionState, live map[string]liveEntry) provider.SessionStatus {
	if entry, ok := live[id]; ok && isAlive(entry.PID) {
		return provider.StatusActive
	}
	if !s.lastActive.IsZero() && time.Since(s.lastActive) < idleThreshold {
		return provider.StatusIdle
	}
	return provider.StatusEnded
}

func (p *ClaudeCodeProvider) ListSessions() ([]provider.SessionInfo, error) {
	projectDirs, err := os.ReadDir(p.projectsDir())
	if err != nil {
		return nil, err
	}

	live := p.loadLiveRegistry()

	p.mu.Lock()
	defer p.mu.Unlock()

	var out []provider.SessionInfo
	for _, pd := range projectDirs {
		if !pd.IsDir() {
			continue
		}
		dir := filepath.Join(p.projectsDir(), pd.Name())
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || filepath.Ext(f.Name()) != ".jsonl" {
				continue
			}
			id := strings.TrimSuffix(f.Name(), ".jsonl")
			state, ok := p.sessions[id]
			if !ok {
				state = &sessionState{id: id, cursor: tailreader.Cursor{Path: filepath.Join(dir, f.Name())}}
				p.sessions[id] = state
			}
			out = append(out, state.info(p.Name(), p.statusFor(id, state, live)))
		}
	}
	return out, nil
}

func (p *ClaudeCodeProvider) ReadContext(sessionID string) (*provider.SessionContext, error) {
	p.mu.Lock()
	state, ok := p.sessions[sessionID]
	p.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("claudecode: unknown session %q (call ListSessions first)", sessionID)
	}

	data, err := state.cursor.ReadNew()
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	state.applyNew(data)
	live := p.loadLiveRegistry()
	return &provider.SessionContext{
		Session: state.info(p.Name(), p.statusFor(sessionID, state, live)),
		Tokens:  provider.TokenUsage{Total: state.tokens, Source: provider.TokenSourceExact},
		Files:   state.fileRefs(),
		Context: provider.ContextUsage{
			Tokens: state.contextTokens,
			Known:  state.model != "",
			Limit:  contextLimitFor(state.model),
		},
	}, nil
}
