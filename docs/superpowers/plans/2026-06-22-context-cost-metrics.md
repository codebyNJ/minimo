# Context-Fullness & Cost Metrics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a real "current context fullness" metric (distinct from the existing lifetime token sum) and surface exact cost where providers have it, then make both visible in `ctx status`.

**Architecture:** Extend `provider.SessionInfo`/`SessionContext` in place with `Model`, `ContextUsage`, and `Cost` fields (Approach A from `docs/superpowers/specs/2026-06-22-context-cost-metrics-design.md`). Claude Code tracks the latest assistant turn's model + input/cache tokens (replace-not-accumulate, mirroring the existing `label` tracking). OpenCode adds `model` to its existing `SELECT` and finally surfaces the `cost` column it already queries. `configprovider` needs no changes — its zero-value defaults are already correct. `cmd/ctx/main.go`'s flat table gains MODEL/CONTEXT/COST columns and renames the ambiguous `TOKENS` header to `LIFETIME`.

**Tech Stack:** Go 1.25 (existing floor), stdlib only — no new dependencies. No test framework — zero `_test.go` files, manual `go build` / `go run` verification against real session data on this machine, consistent with every prior phase of this project.

## Global Constraints

- No automated tests, no test files — manual run/verify steps only
- No comments explaining *what* code does — only where non-obvious (the context-window table's sourcing/scoping rationale, the `ContextUsage`/`Cost` `Known`-flag semantics)
- Primary dev/verification platform is Windows (this machine)
- `internal/export` is **not** modified anywhere in this plan — frozen per the design decision
- Window-size table entries are added **only** for models directly observed in real session data on this machine via `grep` — never guessed. Confirmed so far: `claude-sonnet-4-6`, `claude-opus-4-8` (both 1,000,000 tokens per Anthropic's published API context-window docs — the API tier applies because Claude Code is an API-driven CLI, not the lower-limit `claude.ai` web-chat product)
- `TokenUsage.Total`'s existing summing behavior is **not** changed — it remains a correct lifetime/cost-driver sum; only its table label changes (`TOKENS` → `LIFETIME`) for clarity
- Real Claude Code session id for manual verification on this machine: `5181c116-b48e-4b38-8101-2423b538ef5b` (this project's own live session)

---

### Task 1: Extend shared provider types

**Files:**
- Modify: `internal/provider/provider.go`

**Interfaces:**
- Consumes: nothing new (extends existing `SessionInfo`, `SessionContext`).
- Produces: `provider.SessionInfo.Model string` (new field), `provider.ContextUsage{Tokens int, Known bool, Limit int}`, `provider.Cost{USD float64, Known bool}`, `provider.SessionContext.Context ContextUsage` and `.Cost Cost` (new fields). Tasks 2-4 consume all of these by exact name.

- [ ] **Step 1: Add the new fields and types**

Modify `internal/provider/provider.go` — current full contents:

```go
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
```

New full contents:

```go
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
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit code 0. (Every existing provider's struct literals are keyed, so the new fields default to their zero values — `Model: ""`, `Context: ContextUsage{}`, `Cost: Cost{}` — without any other file needing to change yet.)

- [ ] **Step 3: Commit**

```bash
git add internal/provider/provider.go
git commit -m "feat: add model, context-fullness, and cost fields to provider types"
```

---

### Task 2: Claude Code — track model and latest-turn context size

**Files:**
- Modify: `internal/provider/claudecode/jsonl.go`
- Modify: `internal/provider/claudecode/state.go`
- Modify: `internal/provider/claudecode/provider.go`
- Create: `internal/provider/claudecode/contextwindow.go`

**Interfaces:**
- Consumes: `provider.SessionInfo`, `provider.ContextUsage`, `provider.SessionContext` (Task 1).
- Produces: `sessionState.model string`, `sessionState.contextTokens int` (new fields, package-private, read directly by `provider.go` in the same package); `contextLimitFor(model string) int`. Task 4 doesn't touch this package directly — it only reads `SessionContext.Context`/`SessionInfo.Model` via the engine, same as it already reads `Tokens`.

- [ ] **Step 1: Add the `model` field to the JSONL message struct**

Modify `internal/provider/claudecode/jsonl.go` — change the `jsonlLine.Message` struct from:

```go
	Message   struct {
		Usage struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
		Content []struct {
			Type  string `json:"type"`
			Name  string `json:"name"`
			Input struct {
				FilePath string `json:"file_path"`
			} `json:"input"`
		} `json:"content"`
	} `json:"message"`
```

to:

```go
	Message   struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
		Content []struct {
			Type  string `json:"type"`
			Name  string `json:"name"`
			Input struct {
				FilePath string `json:"file_path"`
			} `json:"input"`
		} `json:"content"`
	} `json:"message"`
```

- [ ] **Step 2: Track model and latest-turn context size in `sessionState`**

Modify `internal/provider/claudecode/state.go` — current full contents:

```go
package claudecode

import (
	"sort"
	"time"

	"github.com/codebyNJ/minimo/internal/provider"
)

type sessionState struct {
	id         string
	cwd        string
	label      string
	startedAt  time.Time
	lastActive time.Time
	tokens     int
	files      map[string]struct{}
	cursor     tailCursor
}

var fileTools = map[string]bool{"Read": true, "Edit": true, "Write": true}

func (s *sessionState) applyNew(data []byte) {
	for _, l := range parseLines(data) {
		if ts, ok := parseTimestamp(l.Timestamp); ok {
			if s.startedAt.IsZero() {
				s.startedAt = ts
			}
			s.lastActive = ts
		}
		if l.CWD != "" {
			s.cwd = l.CWD
		}
		if l.Type == "ai-title" && l.AITitle != "" {
			s.label = l.AITitle
		}
		if l.Type == "assistant" {
			u := l.Message.Usage
			s.tokens += u.InputTokens + u.OutputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
			for _, block := range l.Message.Content {
				if block.Type == "tool_use" && fileTools[block.Name] && block.Input.FilePath != "" {
					if s.files == nil {
						s.files = make(map[string]struct{})
					}
					s.files[block.Input.FilePath] = struct{}{}
				}
			}
		}
	}
}

func (s *sessionState) fileRefs() []provider.FileRef {
	paths := make([]string, 0, len(s.files))
	for p := range s.files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	out := make([]provider.FileRef, 0, len(paths))
	for _, p := range paths {
		out = append(out, provider.FileRef{Path: p})
	}
	return out
}

func (s *sessionState) info(providerName string, status provider.SessionStatus) provider.SessionInfo {
	return provider.SessionInfo{
		ID:         s.id,
		Provider:   providerName,
		CWD:        s.cwd,
		Label:      s.label,
		Status:     status,
		StartedAt:  s.startedAt,
		LastActive: s.lastActive,
	}
}
```

New full contents:

```go
package claudecode

import (
	"sort"
	"time"

	"github.com/codebyNJ/minimo/internal/provider"
)

type sessionState struct {
	id            string
	cwd           string
	label         string
	model         string
	startedAt     time.Time
	lastActive    time.Time
	tokens        int
	contextTokens int
	files         map[string]struct{}
	cursor        tailCursor
}

var fileTools = map[string]bool{"Read": true, "Edit": true, "Write": true}

func (s *sessionState) applyNew(data []byte) {
	for _, l := range parseLines(data) {
		if ts, ok := parseTimestamp(l.Timestamp); ok {
			if s.startedAt.IsZero() {
				s.startedAt = ts
			}
			s.lastActive = ts
		}
		if l.CWD != "" {
			s.cwd = l.CWD
		}
		if l.Type == "ai-title" && l.AITitle != "" {
			s.label = l.AITitle
		}
		if l.Type == "assistant" {
			u := l.Message.Usage
			s.tokens += u.InputTokens + u.OutputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
			s.contextTokens = u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
			if l.Message.Model != "" {
				s.model = l.Message.Model
			}
			for _, block := range l.Message.Content {
				if block.Type == "tool_use" && fileTools[block.Name] && block.Input.FilePath != "" {
					if s.files == nil {
						s.files = make(map[string]struct{})
					}
					s.files[block.Input.FilePath] = struct{}{}
				}
			}
		}
	}
}

func (s *sessionState) fileRefs() []provider.FileRef {
	paths := make([]string, 0, len(s.files))
	for p := range s.files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	out := make([]provider.FileRef, 0, len(paths))
	for _, p := range paths {
		out = append(out, provider.FileRef{Path: p})
	}
	return out
}

func (s *sessionState) info(providerName string, status provider.SessionStatus) provider.SessionInfo {
	return provider.SessionInfo{
		ID:         s.id,
		Provider:   providerName,
		CWD:        s.cwd,
		Label:      s.label,
		Model:      s.model,
		Status:     status,
		StartedAt:  s.startedAt,
		LastActive: s.lastActive,
	}
}
```

Note: `contextTokens` is **replaced**, not accumulated (`s.contextTokens = ...`, not `+=`) — it represents the latest turn's figure, not a running sum. Contrast with `s.tokens` just above it, which still accumulates (`+=`) — that's the unchanged lifetime sum.

- [ ] **Step 3: Add the context-window lookup table**

Create `internal/provider/claudecode/contextwindow.go`:

```go
package claudecode

// Verified 2026-06-22 against Anthropic's published API context-window
// docs. Claude Code is an API-driven CLI, so the API tier's window size
// applies — not the lower limit some platforms impose on the claude.ai
// web-chat product. Only models actually observed in real session data on
// this machine are listed; anything else returns 0 (unknown), which the
// CLI renders as a raw token count with no percentage rather than risk a
// wrong denominator.
var contextWindowSizes = map[string]int{
	"claude-sonnet-4-6": 1_000_000,
	"claude-opus-4-8":   1_000_000,
}

func contextLimitFor(model string) int {
	return contextWindowSizes[model]
}
```

- [ ] **Step 4: Populate `Context` in `ReadContext`**

Modify `internal/provider/claudecode/provider.go` — change the end of `ReadContext` from:

```go
	return &provider.SessionContext{
		Session: state.info(p.Name(), p.statusFor(sessionID, state, live)),
		Tokens:  provider.TokenUsage{Total: state.tokens, Source: provider.TokenSourceExact},
		Files:   state.fileRefs(),
	}, nil
```

to:

```go
	return &provider.SessionContext{
		Session: state.info(p.Name(), p.statusFor(sessionID, state, live)),
		Tokens:  provider.TokenUsage{Total: state.tokens, Source: provider.TokenSourceExact},
		Files:   state.fileRefs(),
		Context: provider.ContextUsage{
			Tokens: state.contextTokens,
			Known:  true,
			Limit:  contextLimitFor(state.model),
		},
	}, nil
```

- [ ] **Step 5: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit code 0.

- [ ] **Step 6: Commit**

```bash
git add internal/provider/claudecode/jsonl.go internal/provider/claudecode/state.go internal/provider/claudecode/provider.go internal/provider/claudecode/contextwindow.go
git commit -m "feat: track latest-turn model and context size in claude code provider"
```

---

### Task 3: OpenCode — surface model and exact cost

**Files:**
- Modify: `internal/provider/opencode/queries.go`
- Modify: `internal/provider/opencode/provider.go`

**Interfaces:**
- Consumes: `provider.Cost`, `provider.SessionContext`, `provider.SessionInfo` (Task 1).
- Produces: `sessionRow.model string` (new field). Task 4 doesn't touch this package directly — same as Task 2, only the engine/CLI read the resulting `SessionContext` fields.

- [ ] **Step 1: Add `model` to the SQL query and row struct**

Modify `internal/provider/opencode/queries.go` — current full contents:

```go
package opencode

import (
	"database/sql"
	"time"
)

type sessionRow struct {
	id               string
	directory        string
	title            string
	cost             float64
	tokensInput      int64
	tokensOutput     int64
	tokensReasoning  int64
	tokensCacheRead  int64
	tokensCacheWrite int64
	timeCreated      int64
	timeUpdated      int64
	timeArchived     sql.NullInt64
}

const sessionColumns = `id, directory, title, cost, tokens_input, tokens_output, tokens_reasoning, tokens_cache_read, tokens_cache_write, time_created, time_updated, time_archived`

func scanSessionRow(scan func(...any) error) (sessionRow, error) {
	var r sessionRow
	err := scan(&r.id, &r.directory, &r.title, &r.cost, &r.tokensInput, &r.tokensOutput,
		&r.tokensReasoning, &r.tokensCacheRead, &r.tokensCacheWrite,
		&r.timeCreated, &r.timeUpdated, &r.timeArchived)
	return r, err
}

func listSessions(db *sql.DB) ([]sessionRow, error) {
	rows, err := db.Query(`SELECT ` + sessionColumns + ` FROM session`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []sessionRow
	for rows.Next() {
		r, err := scanSessionRow(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func readSession(db *sql.DB, id string) (*sessionRow, error) {
	row := db.QueryRow(`SELECT `+sessionColumns+` FROM session WHERE id = ?`, id)
	r, err := scanSessionRow(row.Scan)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func epochMillis(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}
```

New full contents:

```go
package opencode

import (
	"database/sql"
	"time"
)

type sessionRow struct {
	id               string
	directory        string
	title            string
	model            string
	cost             float64
	tokensInput      int64
	tokensOutput     int64
	tokensReasoning  int64
	tokensCacheRead  int64
	tokensCacheWrite int64
	timeCreated      int64
	timeUpdated      int64
	timeArchived     sql.NullInt64
}

const sessionColumns = `id, directory, title, model, cost, tokens_input, tokens_output, tokens_reasoning, tokens_cache_read, tokens_cache_write, time_created, time_updated, time_archived`

func scanSessionRow(scan func(...any) error) (sessionRow, error) {
	var r sessionRow
	err := scan(&r.id, &r.directory, &r.title, &r.model, &r.cost, &r.tokensInput, &r.tokensOutput,
		&r.tokensReasoning, &r.tokensCacheRead, &r.tokensCacheWrite,
		&r.timeCreated, &r.timeUpdated, &r.timeArchived)
	return r, err
}

func listSessions(db *sql.DB) ([]sessionRow, error) {
	rows, err := db.Query(`SELECT ` + sessionColumns + ` FROM session`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []sessionRow
	for rows.Next() {
		r, err := scanSessionRow(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func readSession(db *sql.DB, id string) (*sessionRow, error) {
	row := db.QueryRow(`SELECT `+sessionColumns+` FROM session WHERE id = ?`, id)
	r, err := scanSessionRow(row.Scan)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func epochMillis(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}
```

- [ ] **Step 2: Populate `Model` and `Cost`**

Modify `internal/provider/opencode/provider.go` — change `toSessionInfo` from:

```go
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
```

to:

```go
func (p *OpenCodeProvider) toSessionInfo(r sessionRow) provider.SessionInfo {
	return provider.SessionInfo{
		ID:         r.id,
		Provider:   p.Name(),
		CWD:        r.directory,
		Label:      r.title,
		Model:      r.model,
		Status:     p.statusFor(r),
		StartedAt:  epochMillis(r.timeCreated),
		LastActive: epochMillis(r.timeUpdated),
	}
}
```

Change `ReadContext` from:

```go
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
```

to:

```go
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
		Cost:    provider.Cost{USD: r.cost, Known: true},
		// Context is left at its zero value (Known: false) — the session
		// table only has lifetime aggregates, not a latest-turn figure.
	}, nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit code 0.

- [ ] **Step 4: Commit**

```bash
git add internal/provider/opencode/queries.go internal/provider/opencode/provider.go
git commit -m "feat: surface model and exact cost in opencode provider"
```

---

### Task 4: Display MODEL, CONTEXT, and COST in `ctx status`

**Files:**
- Modify: `cmd/ctx/main.go`

**Interfaces:**
- Consumes: `provider.ContextUsage`, `provider.Cost`, `provider.SessionInfo.Model` (Tasks 1-3, via `e.Store.All()`).
- Produces: `formatCount(n int) string`, `formatContext(c provider.ContextUsage) string`, `formatCost(c provider.Cost) string`, `emptyDash(s string) string`, `truncateRight(s string, n int) string`. Nothing downstream depends on these — this is the leaf of this plan.

- [ ] **Step 1: Update `printTable` and add the formatting helpers**

Modify `cmd/ctx/main.go` — change `printTable` from:

```go
func printTable(e *engine.Engine) {
	rows := e.Store.All()
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Session.LastActive.After(rows[j].Session.LastActive)
	})

	fmt.Printf("%-12s %-8s %-8s %-10s %-24s %s\n", "PROVIDER", "STATUS", "TOKENS", "LAST", "CWD", "LABEL")
	for _, r := range rows {
		if r.Session.LastActive.IsZero() {
			continue
		}
		fmt.Printf("%-12s %-8s %-8d %-10s %-24s %s\n",
			r.Session.Provider,
			r.Session.Status,
			r.Tokens.Total,
			r.Session.LastActive.Format("15:04:05"),
			truncate(r.Session.CWD, 24),
			r.Session.Label,
		)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n+3:]
}
```

to:

```go
func printTable(e *engine.Engine) {
	rows := e.Store.All()
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Session.LastActive.After(rows[j].Session.LastActive)
	})

	fmt.Printf("%-12s %-8s %-18s %-10s %-12s %-9s %-10s %-24s %s\n",
		"PROVIDER", "STATUS", "MODEL", "LIFETIME", "CONTEXT", "COST", "LAST", "CWD", "LABEL")
	for _, r := range rows {
		if r.Session.LastActive.IsZero() {
			continue
		}
		fmt.Printf("%-12s %-8s %-18s %-10d %-12s %-9s %-10s %-24s %s\n",
			r.Session.Provider,
			r.Session.Status,
			emptyDash(truncateRight(r.Session.Model, 18)),
			r.Tokens.Total,
			formatContext(r.Context),
			formatCost(r.Cost),
			r.Session.LastActive.Format("15:04:05"),
			truncate(r.Session.CWD, 24),
			r.Session.Label,
		)
	}
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func formatCount(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.0fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func formatContext(c provider.ContextUsage) string {
	if !c.Known {
		return "-"
	}
	if c.Limit > 0 {
		return fmt.Sprintf("%s/%s", formatCount(c.Tokens), formatCount(c.Limit))
	}
	return formatCount(c.Tokens)
}

func formatCost(c provider.Cost) string {
	if !c.Known {
		return "-"
	}
	return fmt.Sprintf("$%.4f", c.USD)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n+3:]
}

func truncateRight(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
```

(`truncate` keeps the tail — right for paths like CWD, where the deepest folder matters most. `truncateRight` keeps the head and is used for the MODEL column, where the model family/version prefix matters most.)

- [ ] **Step 2: Build the binary**

Run: `go build -o bin/ctx.exe ./cmd/ctx`
Expected: no output, exit code 0.

- [ ] **Step 3: Manually verify against real Claude Code data**

Run: `.\bin\ctx.exe status` and find the row for this project's own session (`CWD` ending in `minimo`).

Expected: `MODEL` shows `claude-sonnet-4-6` or `claude-opus-4-8` (truncated to 18 chars if longer); `LIFETIME` shows the same large cumulative number as before (unchanged behavior, just relabeled); `CONTEXT` shows a real `current/1.0M` figure where `current` is a plausible single-turn size (tens of thousands to low hundreds of thousands — NOT the same enormous number as `LIFETIME`, proving this is genuinely a different, non-accumulating figure); `COST` shows `-` (Claude Code has no cost data).

- [ ] **Step 4: Manually verify against real OpenCode data**

In the same `ctx status` output, find any `opencode` row with nonzero `LIFETIME`.

Expected: `MODEL` shows a real model string (e.g. a `deepseek-*` or similar value, truncated if long); `COST` shows a real `$X.XXXX` figure (not `-`); `CONTEXT` shows `-` (OpenCode's `Context.Known` is explicitly false).

- [ ] **Step 5: Manually verify a custom provider still shows correctly**

Create a real demo custom provider (same fixture pattern as Phase 5's verification):

```powershell
New-Item -ItemType Directory -Force "$env:USERPROFILE\.ctx\providers" | Out-Null
New-Item -ItemType Directory -Force "$env:USERPROFILE\.ctx-demo" | Out-Null
Set-Content "$env:USERPROFILE\.ctx-demo\demo.log" -Encoding utf8 -Value "session: alpha`ntokens: 100"
$y = "name: demo`nversion: 1`nmonitor:`n  paths:`n    - `"~/.ctx-demo/*.log`"`n  parse:`n    session_pattern: 'session: (" + [char]92 + "S+)'`n    token_pattern: 'tokens: (" + [char]92 + "d+)'"
Set-Content "$env:USERPROFILE\.ctx\providers\demo.yaml" -Encoding utf8 -Value $y
```

Run: `.\bin\ctx.exe status`

Expected: the `demo` row shows `MODEL` as `-`, `CONTEXT` as `-`, `COST` as `-` (all three correctly absent for a provider with no concept of any of them — proving `emptyDash`/`formatContext`/`formatCost` all degrade correctly on zero-value input, not just on explicitly-set `Known: false`).

Clean up the fixtures afterward:

```powershell
Remove-Item -Force "$env:USERPROFILE\.ctx\providers\demo.yaml"
Remove-Item -Recurse -Force "$env:USERPROFILE\.ctx-demo"
```

- [ ] **Step 6: Commit**

```bash
git add cmd/ctx/main.go
git commit -m "feat: display model, context-fullness, and cost columns in ctx status"
```

---

## Self-Review

**Spec coverage** — every locked decision from `docs/superpowers/specs/2026-06-22-context-cost-metrics-design.md` has a task: shared types (Task 1), Claude Code latest-turn tracking + verified window table (Task 2), OpenCode model/cost surfacing (Task 3), display with the LIFETIME relabel and three new columns (Task 4). `internal/export` is untouched — no task references it. The MODEL column (not in the original mockup) is called out explicitly in this plan's intro and Global Constraints rather than silently added.

**Placeholder scan** — no TBD/TODO, no hand-waved error handling; every step has complete code, including the full before/after for every modified file.

**Type consistency** — `provider.SessionInfo.Model`, `provider.ContextUsage{Tokens, Known, Limit}`, `provider.Cost{USD, Known}` (Task 1) are spelled identically in Task 2's `state.go`/`provider.go`, Task 3's `provider.go`, and Task 4's `main.go`. `sessionState.model`/`.contextTokens` (Task 2) and `sessionRow.model` (Task 3) are package-private and only read within their own package, consistent with how `sessionState.tokens` already worked.

---

## What's next

Phase 3 (TUI) is the natural next step — it would replace this plan's plain-text MODEL/CONTEXT/COST columns with real bars/colors/panels, using the exact same `SessionContext` fields this plan populates. Codex and Kimi Code providers (deferred earlier) would need their own context-fullness/cost investigation when picked up — neither has been researched for these specific fields yet.
