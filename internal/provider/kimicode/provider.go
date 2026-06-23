package kimicode

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/codebyNJ/minimo/internal/provider"
	"github.com/codebyNJ/minimo/internal/tailreader"
)

const idleThreshold = 30 * time.Second

func init() {
	provider.Register(New())
}

type trackedSession struct {
	state  sessionState
	cursor tailreader.Cursor
}

type KimiCodeProvider struct {
	mu       sync.Mutex
	sessions map[string]*trackedSession
}

func New() *KimiCodeProvider {
	return &KimiCodeProvider{sessions: make(map[string]*trackedSession)}
}

func (p *KimiCodeProvider) Name() string { return "kimi-code" }

func (p *KimiCodeProvider) home() string {
	if override, ok := provider.PathOverride(p.Name()); ok {
		return override
	}
	if env := os.Getenv("KIMI_CODE_HOME"); env != "" {
		return env
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}
	return filepath.Join(homeDir, ".kimi-code")
}

func (p *KimiCodeProvider) sessionsDir() string { return filepath.Join(p.home(), "sessions") }

func (p *KimiCodeProvider) CheckedPath() string { return p.home() }

func (p *KimiCodeProvider) WatchPaths() []string { return []string{p.sessionsDir()} }

func (p *KimiCodeProvider) Detect() bool {
	info, err := os.Stat(p.sessionsDir())
	return err == nil && info.IsDir()
}

// findWireLogs walks sessions/<workDirKey>/<sessionId>/agents/main/wire.jsonl
// without decoding workDirKey — only the leaf sessionId directory name is
// used as ctx's session ID, since workDirKey's path-encoding scheme isn't
// documented anywhere found.
func (p *KimiCodeProvider) findWireLogs() map[string]string {
	out := make(map[string]string)
	workDirKeys, err := os.ReadDir(p.sessionsDir())
	if err != nil {
		return out
	}
	for _, wdk := range workDirKeys {
		if !wdk.IsDir() {
			continue
		}
		wdkPath := filepath.Join(p.sessionsDir(), wdk.Name())
		sessionDirs, err := os.ReadDir(wdkPath)
		if err != nil {
			continue
		}
		for _, sd := range sessionDirs {
			if !sd.IsDir() {
				continue
			}
			wirePath := filepath.Join(wdkPath, sd.Name(), "agents", "main", "wire.jsonl")
			if info, err := os.Stat(wirePath); err == nil && !info.IsDir() {
				out[sd.Name()] = wirePath
			}
		}
	}
	return out
}

func (p *KimiCodeProvider) statusFor(s *sessionState) provider.SessionStatus {
	if !s.lastActive.IsZero() && time.Since(s.lastActive) < idleThreshold {
		return provider.StatusIdle
	}
	return provider.StatusEnded
}

func (p *KimiCodeProvider) info(id string, s *sessionState) provider.SessionInfo {
	return provider.SessionInfo{
		ID:         id,
		Provider:   p.Name(),
		Status:     p.statusFor(s),
		StartedAt:  s.startedAt,
		LastActive: s.lastActive,
	}
}

func (p *KimiCodeProvider) ListSessions() ([]provider.SessionInfo, error) {
	wireLogs := p.findWireLogs()

	p.mu.Lock()
	defer p.mu.Unlock()

	var out []provider.SessionInfo
	for id, path := range wireLogs {
		ts, ok := p.sessions[id]
		if !ok {
			ts = &trackedSession{cursor: tailreader.Cursor{Path: path}}
			p.sessions[id] = ts
		}
		if ts.state.startedAt.IsZero() {
			if info, err := os.Stat(path); err == nil {
				ts.state.startedAt = info.ModTime()
			}
		}
		out = append(out, p.info(id, &ts.state))
	}
	return out, nil
}

func (p *KimiCodeProvider) ReadContext(sessionID string) (*provider.SessionContext, error) {
	p.mu.Lock()
	ts, ok := p.sessions[sessionID]
	p.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("kimicode: unknown session %q (call ListSessions first)", sessionID)
	}

	data, err := ts.cursor.ReadNew()
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	ts.state.applyNew(parseWireLines(data))
	if info, err := os.Stat(ts.cursor.Path); err == nil {
		ts.state.lastActive = info.ModTime()
	}
	return &provider.SessionContext{
		Session: p.info(sessionID, &ts.state),
		Tokens:  provider.TokenUsage{Total: ts.state.tokens, Source: provider.TokenSourceExact},
		Context: provider.ContextUsage{
			Tokens: ts.state.contextTokens,
			Known:  ts.state.contextKnown,
			Limit:  ts.state.contextLimit,
		},
	}, nil
}
