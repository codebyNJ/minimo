package provider

import "time"

type SessionStatus string

const (
	StatusActive SessionStatus = "active"
	StatusIdle   SessionStatus = "idle"
	StatusEnded  SessionStatus = "ended"
)

type TokenSource int

const (
	TokenSourceExact TokenSource = iota
	TokenSourceEstimated
)

type TokenUsage struct {
	Total  int
	Source TokenSource
}

type SessionInfo struct {
	ID         string
	Provider   string
	CWD        string
	Label      string
	Status     SessionStatus
	StartedAt  time.Time
	LastActive time.Time
}

type FileRef struct {
	Path string
}

type SessionContext struct {
	Session SessionInfo
	Tokens  TokenUsage
	Files   []FileRef
}

type Provider interface {
	Name() string
	Detect() bool
	ListSessions() ([]SessionInfo, error)
	ReadContext(sessionID string) (*SessionContext, error)
}
