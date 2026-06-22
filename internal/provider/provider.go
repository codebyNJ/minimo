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
	Model      string
	Status     SessionStatus
	StartedAt  time.Time
	LastActive time.Time
}

type FileRef struct {
	Path string
}

// ContextUsage is the latest single turn's input+cache size — "what's
// actually in the model's context window right now" — distinct from
// TokenUsage.Total, which is a lifetime sum across every turn. Known is
// false for providers that only expose lifetime aggregates (no way to
// isolate the latest turn); Tokens/Limit are meaningless when Known is
// false. Limit is the model's context window size in tokens; 0 means
// unknown, in which case no percentage should ever be displayed.
type ContextUsage struct {
	Tokens int
	Known  bool
	Limit  int
}

// Cost is a provider-reported dollar figure. Known is false for providers
// that don't track cost at all — USD is meaningless when Known is false.
// There is no estimation here: a provider either reports an exact cost or
// reports none.
type Cost struct {
	USD   float64
	Known bool
}

type SessionContext struct {
	Session SessionInfo
	Tokens  TokenUsage
	Files   []FileRef
	Context ContextUsage
	Cost    Cost
}

type Provider interface {
	Name() string
	Detect() bool
	ListSessions() ([]SessionInfo, error)
	ReadContext(sessionID string) (*SessionContext, error)
}
