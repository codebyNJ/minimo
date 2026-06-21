package export

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/codebyNJ/minimo/internal/provider"
)

type ExportedFile struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
}

type ExportedContext struct {
	Provider    string         `json:"provider"`
	SessionID   string         `json:"sessionId"`
	CWD         string         `json:"cwd"`
	Label       string         `json:"label"`
	Status      string         `json:"status"`
	StartedAt   string         `json:"startedAt,omitempty"`
	LastActive  string         `json:"lastActive,omitempty"`
	Tokens      int            `json:"tokens"`
	TokenSource string         `json:"tokenSource"`
	Files       []ExportedFile `json:"files,omitempty"`
}

func tokenSourceName(s provider.TokenSource) string {
	if s == provider.TokenSourceExact {
		return "exact"
	}
	return "estimated"
}

func rfc3339OrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// withinDir reports whether path lives under dir. Comparison is
// case-insensitive because the primary platform is Windows (NTFS), where
// the session cwd and a tracked file path routinely differ only in drive
// letter case (e.g. "D:\codes\minimo" vs "d:\codes\minimo\go.mod").
func withinDir(path, dir string) bool {
	if dir == "" {
		return false
	}
	p := strings.ToLower(filepath.Clean(path))
	d := strings.ToLower(filepath.Clean(dir))
	if p == d {
		return true
	}
	return strings.HasPrefix(p, d+string(filepath.Separator))
}

func Build(ctx provider.SessionContext, withContent bool) ExportedContext {
	out := ExportedContext{
		Provider:    ctx.Session.Provider,
		SessionID:   ctx.Session.ID,
		CWD:         ctx.Session.CWD,
		Label:       ctx.Session.Label,
		Status:      string(ctx.Session.Status),
		StartedAt:   rfc3339OrEmpty(ctx.Session.StartedAt),
		LastActive:  rfc3339OrEmpty(ctx.Session.LastActive),
		Tokens:      ctx.Tokens.Total,
		TokenSource: tokenSourceName(ctx.Tokens.Source),
	}
	for _, f := range ctx.Files {
		ef := ExportedFile{Path: f.Path}
		// --with-content only embeds bytes for files inside the session's
		// own working directory. Out-of-project paths the agent happened to
		// read (credentials, other repos) keep their path listed but never
		// have their contents exported — the privacy line this whole flag exists for.
		if withContent && withinDir(f.Path, ctx.Session.CWD) {
			if data, err := os.ReadFile(f.Path); err == nil {
				ef.Content = string(data)
			}
		}
		out.Files = append(out.Files, ef)
	}
	return out
}
