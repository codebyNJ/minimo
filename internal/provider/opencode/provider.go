package opencode

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/codebyNJ/minimo/internal/provider"
)

const idleThreshold = 30 * time.Second

func init() {
	provider.Register(New())
}

type OpenCodeProvider struct {
	dbPath string
	db     *sql.DB
}

func New() *OpenCodeProvider {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return &OpenCodeProvider{
		dbPath: filepath.Join(home, ".local", "share", "opencode", "opencode.db"),
	}
}

func (p *OpenCodeProvider) Name() string { return "opencode" }

func (p *OpenCodeProvider) Detect() bool {
	info, err := os.Stat(p.dbPath)
	return err == nil && !info.IsDir()
}

// SQLite's URI filename parser wants forward slashes even on Windows
// (file:C:/Users/... not file:C:\Users\...), so dbPath is normalized here.
func (p *OpenCodeProvider) open() (*sql.DB, error) {
	if p.db != nil {
		return p.db, nil
	}
	dsn := "file:" + filepath.ToSlash(p.dbPath) + "?mode=ro"
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
	if time.Since(epochMillis(r.timeUpdated)) < idleThreshold {
		return provider.StatusIdle
	}
	return provider.StatusEnded
}

func (p *OpenCodeProvider) toSessionInfo(r sessionRow) provider.SessionInfo {
	return provider.SessionInfo{
		ID:         r.id,
		Provider:   p.Name(),
		CWD:        r.directory,
		Label:      r.title,
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

func (p *OpenCodeProvider) ReadContext(sessionID string) (*provider.SessionContext, error) {
	db, err := p.open()
	if err != nil {
		return nil, err
	}
	r, err := readSession(db, sessionID)
	if err != nil {
		return nil, err
	}
	total := int(r.tokensInput + r.tokensOutput + r.tokensReasoning + r.tokensCacheRead + r.tokensCacheWrite)
	return &provider.SessionContext{
		Session: p.toSessionInfo(*r),
		Tokens:  provider.TokenUsage{Total: total, Source: provider.TokenSourceExact},
	}, nil
}
