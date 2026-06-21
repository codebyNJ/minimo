# ctx Phase 2 — OpenCode Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an OpenCode provider to `ctx` that reads real session/usage data directly from OpenCode's SQLite database, so `ctx status` (and `--watch`) lists OpenCode sessions alongside Claude Code sessions with exact token/cost numbers — no transcript parsing needed.

**Architecture:** A new `internal/provider/opencode` package implements the existing `provider.Provider` interface (already proven by the Claude Code provider) against `~/.local/share/opencode/opencode.db`'s `session` table, which already stores aggregated `tokens_*`/`cost` columns per session — confirmed by ground-truth inspection of a real install on this machine (see `docs/wiki/Research.md`). Because OpenCode has no live-PID registry (confirmed absent, same doc), liveness falls back to the same mtime-based inference pattern already used elsewhere. Because the database is SQLite/WAL (rewrites, not appends), this provider polls on a timer instead of using the byte-offset tail reader built for Claude Code's JSONL — which means `cmd/ctx/main.go`'s `--watch` loop needs a second trigger source (a ticker) alongside the existing fsnotify channel.

**Tech Stack:** Go 1.23+, `modernc.org/sqlite` (cgo-free SQLite driver, exact version `v1.53.0` — confirmed latest stable via `go list -m -versions modernc.org/sqlite` on 2026-06-21). No test framework — zero `_test.go` files, manual `go build` / `go run` verification only, consistent with Phase 1.

## Global Constraints

- Module path: `github.com/codebyNJ/minimo`
- Go version floor: 1.23
- No automated tests, no test files, no testing frameworks — manual run/verify steps only
- No comments explaining *what* code does — only where a non-obvious constraint needs flagging (e.g. the Windows SQLite URI path-separator gotcha in Task 2)
- Primary dev/verification platform is Windows (this machine)
- Dependencies are added with exact versions via `go get`, never hand-edited into `go.mod`
- OpenCode DB path is `~/.local/share/opencode/opencode.db` on **all** OSes, including Windows — confirmed by inspecting a real file at `C:\Users\sst\.local\share\opencode\opencode.db` on this machine (see `docs/wiki/Research.md#opencode--ground-truth-verified-by-inspecting-a-real-install-on-this-machine-2026-06-21`); no XDG override branching needed for v1
- `session` table columns used (real schema, confirmed): `id, directory, title, cost, tokens_input, tokens_output, tokens_reasoning, tokens_cache_read, tokens_cache_write, time_created, time_updated, time_archived` — `time_created`/`time_updated`/`time_archived` are epoch milliseconds (OpenCode is a TypeScript/Bun project; `Date.now()` semantics)
- OpenCode has no live-PID registry (confirmed absent) — `Status()` can only return `StatusIdle` or `StatusEnded`, never `StatusActive`, same mtime-fallback pattern as documented in `docs/wiki/Architecture.md#1-session-status-must-come-from-the-live-pid-registry-not-file-mtimes`
- Scope: only the OpenCode provider. Codex and Kimi Code (the other two Phase 2 providers per `docs/wiki/Roadmap.md`) are explicitly deferred to a later plan.

---

### Task 1: SQLite query layer

**Files:**
- Create: `internal/provider/opencode/queries.go`

**Interfaces:**
- Consumes: nothing from earlier tasks (this is the first task in this plan).
- Produces: `sessionRow{id, directory, title string; cost float64; tokensInput, tokensOutput, tokensReasoning, tokensCacheRead, tokensCacheWrite, timeCreated, timeUpdated int64; timeArchived sql.NullInt64}`, `listSessions(db *sql.DB) ([]sessionRow, error)`, `readSession(db *sql.DB, id string) (*sessionRow, error)`, `epochMillis(ms int64) time.Time`. Task 2's provider consumes all of these by exact name.

- [ ] **Step 1: Write the query layer**

Create `internal/provider/opencode/queries.go`:

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

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit code 0. (`database/sql` is stdlib, so this compiles even before the driver is added in Task 2.)

- [ ] **Step 3: Commit**

```bash
git add internal/provider/opencode/queries.go
git commit -m "feat: add opencode sqlite query layer"
```

---

### Task 2: OpenCode provider — Detect, ListSessions, ReadContext

**Files:**
- Create: `internal/provider/opencode/provider.go`

**Interfaces:**
- Consumes: `provider.Register`, `provider.Provider`, `provider.SessionInfo`, `provider.SessionContext`, `provider.SessionStatus`, `provider.StatusIdle`, `provider.StatusEnded`, `provider.TokenUsage`, `provider.TokenSourceExact` (from `internal/provider`, Phase 1 Task 1); `sessionRow`, `listSessions`, `readSession`, `epochMillis` (Task 1).
- Produces: `opencode.New() *OpenCodeProvider`, `(*OpenCodeProvider)` implementing `provider.Provider` fully. Registered automatically via `init()`. Task 3 blank-imports this package to trigger registration.

- [ ] **Step 1: Add the modernc.org/sqlite dependency**

Run: `go get modernc.org/sqlite@v1.53.0`

- [ ] **Step 2: Write the provider**

Create `internal/provider/opencode/provider.go`:

```go
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
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit code 0.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum internal/provider/opencode/provider.go
git commit -m "feat: implement opencode provider"
```

---

### Task 3: Wire OpenCode into the CLI, add poll-ticker to watch mode

**Files:**
- Modify: `internal/engine/config.go`
- Modify: `cmd/ctx/main.go`

**Interfaces:**
- Consumes: `opencode.New` (via blank import for `init()` registration, Task 2); `engine.PollIntervalDefault` (new, this task).
- Produces: `engine.PollIntervalDefault time.Duration`. Nothing downstream depends on this — it's the leaf of this plan.

- [ ] **Step 1: Add the poll interval default**

Modify `internal/engine/config.go` — current full contents:

```go
package engine

import "time"

const DebounceDefault = 500 * time.Millisecond
```

New full contents:

```go
package engine

import "time"

const DebounceDefault = 500 * time.Millisecond
const PollIntervalDefault = 2 * time.Second
```

- [ ] **Step 2: Blank-import the opencode provider**

In `cmd/ctx/main.go`, change the import block from:

```go
import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/codebyNJ/minimo/internal/engine"
	_ "github.com/codebyNJ/minimo/internal/provider/claudecode"
	"github.com/codebyNJ/minimo/internal/watcher"
)
```

to:

```go
import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/codebyNJ/minimo/internal/engine"
	_ "github.com/codebyNJ/minimo/internal/provider/claudecode"
	_ "github.com/codebyNJ/minimo/internal/provider/opencode"
	"github.com/codebyNJ/minimo/internal/watcher"
)
```

- [ ] **Step 3: Add a poll ticker to watch mode**

OpenCode is SQLite-backed, not fsnotify-watched (per `docs/wiki/Architecture.md#6-file-change-detection-must-handle-truncationrewrite-not-just-append`), so `--watch` mode needs a second event source. In `cmd/ctx/main.go`, change `runWatch` from:

```go
	go w.Run(ctx)

	refresh := func() {
		if err := e.Refresh(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		fmt.Print("\033[H\033[2J")
		printTable(e)
	}

	refresh()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.Events:
			refresh()
		}
	}
}
```

to:

```go
	go w.Run(ctx)

	ticker := time.NewTicker(engine.PollIntervalDefault)
	defer ticker.Stop()

	refresh := func() {
		if err := e.Refresh(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		fmt.Print("\033[H\033[2J")
		printTable(e)
	}

	refresh()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.Events:
			refresh()
		case <-ticker.C:
			refresh()
		}
	}
}
```

- [ ] **Step 4: Build the binary**

Run: `go build -o bin/ctx.exe ./cmd/ctx`
Expected: no output, exit code 0.

- [ ] **Step 5: Manually verify against this machine's real OpenCode data**

Run: `./bin/ctx.exe status`

Expected: the table now includes rows with `PROVIDER` = `opencode`, alongside the existing `claude-code` rows. At least one `opencode` row should have a positive `TOKENS` integer and a non-empty `LABEL` (a real session title from `opencode.db`'s `title` column). `STATUS` for `opencode` rows should be `idle` or `ended` — never `active` (no live-PID registry exists for OpenCode).

- [ ] **Step 6: Manually verify the poll ticker fires independently of fsnotify**

Run: `./bin/ctx.exe status --watch`, let it run for about 10 seconds, then press `Ctrl+C`.

Expected: the table reprints roughly every 2 seconds even if no Claude Code `.jsonl` file changes during that window — proof the ticker branch is firing, not just the fsnotify branch. `Ctrl+C` exits immediately (same graceful-shutdown guarantee as Phase 1).

- [ ] **Step 7: Commit**

```bash
git add internal/engine/config.go cmd/ctx/main.go
git commit -m "feat: wire opencode provider into ctx status, add poll ticker to watch mode"
```

---

## Self-Review

**Spec coverage** — OpenCode provider implementing the full `Provider` interface (Tasks 1-2), wired into both one-shot and `--watch` CLI modes (Task 3). Liveness correctly limited to `Idle`/`Ended` per the confirmed absence of a live-PID registry. Exact token/cost data read directly from already-aggregated columns, no parsing. Codex and Kimi Code intentionally out of scope for this plan.

**Placeholder scan** — no TBD/TODO, no hand-waved error handling, no "similar to Task N" shortcuts; every step has complete code.

**Type consistency** — `sessionRow`, `listSessions`, `readSession`, `epochMillis` (Task 1) are spelled identically in Task 2's `provider.go`. `OpenCodeProvider`, `New()`, `Name()`, `Detect()`, `ListSessions()`, `ReadContext()` match the `provider.Provider` interface signatures exactly as defined in Phase 1.

---

## What's next

Codex and Kimi Code providers (the rest of Phase 2 per `docs/wiki/Roadmap.md`) remain unscoped into a plan — written up only when picked back up. After this plan, work continues directly into Phase 4 (Export/Inject) and Phase 5 (Custom provider system), each as their own plan document, per explicit instruction.
