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
	Total         int
	Input         int
	Output        int
	CacheRead     int
	CacheCreation int
	Source        TokenSource
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

type CostSource int

const (
	// CostSourceExact must remain the zero value so a provider reporting a
	// native cost without setting Source is never mislabeled estimated.
	CostSourceExact CostSource = iota
	CostSourceEstimated
)

// Cost is a session's dollar figure. Known is false when no exact cost is
// reported AND no estimate could be produced. Source distinguishes a
// provider-reported exact figure from a pricing-catalog estimate.
type Cost struct {
	USD    float64
	Known  bool
	Source CostSource
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

// Watchable is implemented by providers backed by files that change on
// disk (JSONL transcripts), so the fsnotify watcher knows what to watch.
// Providers backed by a database (OpenCode) rely on the poll ticker
// instead and don't implement this.
type Watchable interface {
	WatchPaths() []string
}

// PathReporter is implemented by providers with a single resolvable root
// directory, so the detection-status UI can show which path was actually
// checked. configprovider's generic Provider matches multiple glob
// patterns rather than one root, so it doesn't implement this.
type PathReporter interface {
	CheckedPath() string
}

// PlanInfo is an account-level subscription tier (e.g. "Max", "Pro",
// "Plus"). Tier strings are provider-specific display values, not
// normalized across providers. Known is false when no local plan signal
// was found — never guess a tier.
type PlanInfo struct {
	Tier  string
	Known bool
}

// PlanReporter is implemented by providers that can read a local,
// non-secret account plan-tier signal. Checked via type-assertion in
// engine.ProviderStatuses, same pattern as PathReporter.
type PlanReporter interface {
	Plan() PlanInfo
}
