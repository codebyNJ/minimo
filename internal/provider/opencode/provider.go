package opencode

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/codebyNJ/minimo/internal/provider"
)

func init() {
	provider.Register(New())
}

type OpenCodeProvider struct {
	db *sql.DB
}

func New() *OpenCodeProvider {
	return &OpenCodeProvider{}
}

func (p *OpenCodeProvider) Name() string { return "opencode" }

func (p *OpenCodeProvider) dbPath() string {
	if override, ok := provider.PathOverride(p.Name()); ok {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return filepath.Join(home, ".local", "share", "opencode", "opencode.db")
}

func (p *OpenCodeProvider) CheckedPath() string { return p.dbPath() }

func (p *OpenCodeProvider) Detect() bool {
	info, err := os.Stat(p.dbPath())
	return err == nil && !info.IsDir()
}

// SQLite's URI filename parser wants forward slashes even on Windows
// (file:C:/Users/... not file:C:\Users\...), so dbPath is normalized here.
func (p *OpenCodeProvider) open() (*sql.DB, error) {
	if p.db != nil {
		return p.db, nil
	}
	dsn := "file:" + filepath.ToSlash(p.dbPath()) + "?mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	p.db = db
	return db, nil
}

func (p *OpenCodeProvider) statusFor(r sessionRow) provider.SessionStatus {
	if r.timeArchived.Valid {
		return provider.StatusEnded
	}
	if time.Since(epochMillis(r.timeUpdated)) < provider.IdleThreshold {
		return provider.StatusIdle
	}
	return provider.StatusEnded
}

type modelRef struct {
	ID string `json:"id"`
}

// OpenCode's session.model column stores a JSON object ({"id":...,
// "providerID":...,"variant":...}), not a bare model name — confirmed
// against this machine's real opencode.db on 2026-06-22. Only the id is
// useful for display; fall back to the raw value if it's ever not JSON
// (e.g. an older OpenCode schema), rather than silently showing nothing.
func parseModelName(raw string) string {
	if raw == "" {
		return ""
	}
	var m modelRef
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return raw
	}
	return m.ID
}

func (p *OpenCodeProvider) toSessionInfo(r sessionRow) provider.SessionInfo {
	return provider.SessionInfo{
		ID:         r.id,
		Provider:   p.Name(),
		CWD:        r.directory,
		Label:      r.title,
		Model:      parseModelName(r.model),
		Status:     p.statusFor(r),
		StartedAt:  epochMillis(r.timeCreated),
		LastActive: epochMillis(r.timeUpdated),
	}
}

func (p *OpenCodeProvider) ListSessions() ([]provider.SessionInfo, error) {
	db, err := p.open()
	if err != nil {
		return nil, err
	}
	rows, err := listSessions(db)
	if err != nil {
		return nil, err
	}
	out := make([]provider.SessionInfo, 0, len(rows))
	for _, r := range rows {
		out = append(out, p.toSessionInfo(r))
	}
	return out, nil
}

func toTokenUsage(r sessionRow) provider.TokenUsage {
	total := int(r.tokensInput + r.tokensOutput + r.tokensReasoning + r.tokensCacheRead + r.tokensCacheWrite)
	return provider.TokenUsage{
		Total:         total,
		Input:         int(r.tokensInput),
		Output:        int(r.tokensOutput + r.tokensReasoning),
		CacheRead:     int(r.tokensCacheRead),
		CacheCreation: int(r.tokensCacheWrite),
		Source:        provider.TokenSourceExact,
	}
}

// rowCost reports OpenCode's stored cost. OpenCode writes 0 both for
// genuinely-free turns and for turns it never priced, so a 0 is reported as
// unknown rather than an exact $0.00 — that lets the engine fall back to a
// catalog estimate (and avoids showing a misleading "$0.0000" next to millions
// of tokens). A positive cost is authoritative and kept exact.
func rowCost(r sessionRow) provider.Cost {
	if r.cost > 0 {
		return provider.Cost{USD: r.cost, Known: true, Source: provider.CostSourceExact}
	}
	return provider.Cost{Known: false}
}

func (p *OpenCodeProvider) ReadContext(sessionID string) (*provider.SessionContext, error) {
	db, err := p.open()
	if err != nil {
		return nil, err
	}
	r, err := readSession(db, sessionID)
	if err != nil {
		return nil, err
	}
	return &provider.SessionContext{
		Session: p.toSessionInfo(*r),
		Tokens:  toTokenUsage(*r),
		Cost:    rowCost(*r),
		// Context is left at its zero value (Known: false) — the session
		// table only has lifetime aggregates, not a latest-turn figure.
	}, nil
}
