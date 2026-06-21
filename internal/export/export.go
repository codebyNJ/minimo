package export

import (
	"os"
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
	StartedAt   string         `json:"startedAt"`
	LastActive  string         `json:"lastActive"`
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

func Build(ctx provider.SessionContext, withContent bool) ExportedContext {
	out := ExportedContext{
		Provider:    ctx.Session.Provider,
		SessionID:   ctx.Session.ID,
		CWD:         ctx.Session.CWD,
		Label:       ctx.Session.Label,
		Status:      string(ctx.Session.Status),
		StartedAt:   ctx.Session.StartedAt.Format(time.RFC3339),
		LastActive:  ctx.Session.LastActive.Format(time.RFC3339),
		Tokens:      ctx.Tokens.Total,
		TokenSource: tokenSourceName(ctx.Tokens.Source),
	}
	for _, f := range ctx.Files {
		ef := ExportedFile{Path: f.Path}
		if withContent {
			if data, err := os.ReadFile(f.Path); err == nil {
				ef.Content = string(data)
			}
		}
		out.Files = append(out.Files, ef)
	}
	return out
}
