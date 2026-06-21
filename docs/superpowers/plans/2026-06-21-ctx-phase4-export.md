# ctx Phase 4 — Export Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `ctx export <session-id> [--with-content]`, producing portable JSON for a session's context, with file paths touched during the session and content gated behind the explicit `--with-content` flag per the project's privacy rule.

**Architecture:** `provider.SessionContext` gains a minimal `Files []FileRef` field (paths only — no `Content` at the provider level, by design: content-reading happens at export time, not collection time, so sensitive bytes are never held longer than necessary). Only the Claude Code provider populates it for now, parsed from real `tool_use` blocks (`Read`/`Edit`/`Write`) already present in its JSONL — confirmed by inspecting this very project's own session file (ground truth, not assumed from docs). OpenCode's provider returns an empty `Files` slice; populating it would require parsing message/part content blobs, which is out of scope for this minimal pass. A new `internal/export` package builds the portable JSON struct and only reads file content from disk when `withContent` is true. `cmd/ctx/main.go` gets a new `export` subcommand.

**Tech Stack:** Go 1.23+ (stdlib only — `encoding/json`, `os`, `sort`, `time`). No test framework — zero `_test.go` files, manual `go build` / `go run` verification only.

## Global Constraints

- Module path: `github.com/codebyNJ/minimo`
- No automated tests, no test files — manual run/verify steps only
- No comments explaining *what* code does — only where non-obvious
- Primary dev/verification platform is Windows (this machine)
- **Scope decision (confirmed with user, 2026-06-21):** add a minimal `Files []FileRef{Path}` field now as a prerequisite for export, rather than waiting for Phase 3's TUI (which was skipped). `ctx inject` is explicitly **out of scope** for this round — deferred entirely pending a dedicated future discussion about how context transfer should work; no command, no stub, nothing built for it here.
- Real `tool_use` block shape, confirmed by grepping this project's own live session file (`C:\Users\sst\.claude\projects\d--codes-minimo\5181c116-b48e-4b38-8101-2423b538ef5b.jsonl`) on 2026-06-21: `{"type":"tool_use","id":"...","name":"Write","input":{"file_path":"..."}}`, sitting inside `message.content[]` on `assistant`-type lines. Same shape confirmed for `Read` and `Edit`.

---

### Task 1: `Files[]` field on `SessionContext`, populated by the Claude Code provider

**Files:**
- Modify: `internal/provider/provider.go`
- Modify: `internal/provider/claudecode/jsonl.go`
- Modify: `internal/provider/claudecode/state.go`
- Modify: `internal/provider/claudecode/provider.go`

**Interfaces:**
- Consumes: existing `provider.SessionContext`, `sessionState`, `parseLines` (Phase 1).
- Produces: `provider.FileRef{Path string}`, `provider.SessionContext.Files []FileRef` (new field), `(*sessionState) fileRefs() []provider.FileRef`. Task 2's `export.Build` consumes `provider.SessionContext.Files` and `provider.FileRef.Path` by exact name.

- [ ] **Step 1: Add `FileRef` and the `Files` field**

Modify `internal/provider/provider.go` — add this type and field:

```go
type FileRef struct {
	Path string
}
```

Change `SessionContext` from:

```go
type SessionContext struct {
	Session SessionInfo
	Tokens  TokenUsage
}
```

to:

```go
type SessionContext struct {
	Session SessionInfo
	Tokens  TokenUsage
	Files   []FileRef
}
```

- [ ] **Step 2: Parse `tool_use` blocks in the JSONL lines**

Modify `internal/provider/claudecode/jsonl.go` — change the `jsonlLine.Message` struct from:

```go
	Message   struct {
		Usage struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
```

to:

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

- [ ] **Step 3: Accumulate deduped file paths in `sessionState`**

Modify `internal/provider/claudecode/state.go` — current full contents:

```go
package claudecode

import (
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
	cursor     tailCursor
}

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
		}
	}
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

- [ ] **Step 4: Populate `Files` in `ReadContext`**

Modify `internal/provider/claudecode/provider.go` — change the end of `ReadContext` from:

```go
	return &provider.SessionContext{
		Session: state.info(p.Name(), p.statusFor(sessionID, state, live)),
		Tokens:  provider.TokenUsage{Total: state.tokens, Source: provider.TokenSourceExact},
	}, nil
```

to:

```go
	return &provider.SessionContext{
		Session: state.info(p.Name(), p.statusFor(sessionID, state, live)),
		Tokens:  provider.TokenUsage{Total: state.tokens, Source: provider.TokenSourceExact},
		Files:   state.fileRefs(),
	}, nil
```

- [ ] **Step 5: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit code 0. (The OpenCode provider's `ReadContext` still compiles unchanged — `Files` is left as its Go zero value, `nil`, since that struct literal doesn't set it.)

- [ ] **Step 6: Commit**

```bash
git add internal/provider/provider.go internal/provider/claudecode/jsonl.go internal/provider/claudecode/state.go internal/provider/claudecode/provider.go
git commit -m "feat: track touched files in claude code provider"
```

---

### Task 2: `internal/export` package

**Files:**
- Create: `internal/export/export.go`

**Interfaces:**
- Consumes: `provider.SessionContext`, `provider.FileRef`, `provider.TokenSource`, `provider.TokenSourceExact` (Task 1, Phase 1).
- Produces: `export.ExportedFile{Path, Content string}`, `export.ExportedContext{...}`, `export.Build(ctx provider.SessionContext, withContent bool) ExportedContext`. Task 3's CLI command consumes `export.Build` by exact name.

- [ ] **Step 1: Write the export builder**

Create `internal/export/export.go`:

```go
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
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit code 0.

- [ ] **Step 3: Commit**

```bash
git add internal/export/export.go
git commit -m "feat: add export package with opt-in file content"
```

---

### Task 3: `ctx export <session-id> [--with-content]` CLI command

**Files:**
- Modify: `internal/engine/store.go`
- Modify: `cmd/ctx/main.go`

**Interfaces:**
- Consumes: `export.Build` (Task 2); `engine.New`, `(*Engine).Refresh`, `(*Engine).Store` (Phase 1); new `(*StateStore).Get` (this task).
- Produces: `(*StateStore).Get(sessionID string) (provider.SessionContext, bool)`, the `export` CLI subcommand. Nothing downstream depends on this — it's the leaf of this plan.

- [ ] **Step 1: Add `StateStore.Get`**

Modify `internal/engine/store.go` — add this method after `Put`:

```go
func (s *StateStore) Get(sessionID string) (provider.SessionContext, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.items[sessionID]
	return c, ok
}
```

- [ ] **Step 2: Wire the `export` subcommand**

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
	"time"

	"github.com/codebyNJ/minimo/internal/engine"
	_ "github.com/codebyNJ/minimo/internal/provider/claudecode"
	_ "github.com/codebyNJ/minimo/internal/provider/opencode"
	"github.com/codebyNJ/minimo/internal/watcher"
)
```

to:

```go
import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/codebyNJ/minimo/internal/engine"
	"github.com/codebyNJ/minimo/internal/export"
	_ "github.com/codebyNJ/minimo/internal/provider/claudecode"
	_ "github.com/codebyNJ/minimo/internal/provider/opencode"
	"github.com/codebyNJ/minimo/internal/watcher"
)
```

Change `main()` from:

```go
func main() {
	if len(os.Args) < 2 || os.Args[1] != "status" {
		fmt.Fprintln(os.Stderr, "usage: ctx status [--watch]")
		os.Exit(1)
	}
	runStatus(os.Args[2:])
}
```

to:

```go
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: ctx status [--watch] | ctx export <session-id> [--with-content]")
		os.Exit(1)
	}
	switch os.Args[1] {
	case "status":
		runStatus(os.Args[2:])
	case "export":
		runExport(os.Args[2:])
	default:
		fmt.Fprintln(os.Stderr, "usage: ctx status [--watch] | ctx export <session-id> [--with-content]")
		os.Exit(1)
	}
}

func runExport(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: ctx export <session-id> [--with-content]")
		os.Exit(1)
	}
	sessionID := args[0]
	withContent := false
	for _, a := range args[1:] {
		if a == "--with-content" {
			withContent = true
		}
	}

	e := engine.New()
	if err := e.Refresh(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	ctx, ok := e.Store.Get(sessionID)
	if !ok {
		fmt.Fprintf(os.Stderr, "error: unknown session %q\n", sessionID)
		os.Exit(1)
	}

	out := export.Build(ctx, withContent)
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}
```

- [ ] **Step 3: Build the binary**

Run: `go build -o bin/ctx.exe ./cmd/ctx`
Expected: no output, exit code 0.

- [ ] **Step 4: Manually verify export without `--with-content`**

Run: `./bin/ctx.exe export 5181c116-b48e-4b38-8101-2423b538ef5b` (this project's own live Claude Code session — guaranteed to have real `Read`/`Write`/`Edit` tool calls).

Expected: JSON printed to stdout with `provider: "claude-code"`, a positive `tokens`, `tokenSource: "exact"`, and a non-empty `files` array of real paths from this project (e.g. entries ending in `provider.go`, `Roadmap.md`, etc.) — each with `path` set and **no** `content` key present (omitted, not empty string, thanks to `omitempty`).

- [ ] **Step 5: Manually verify `--with-content` actually includes content**

Run: `./bin/ctx.exe export 5181c116-b48e-4b38-8101-2423b538ef5b --with-content`

Expected: same structure, but every `files[]` entry that still exists on disk now has a non-empty `content` field containing that file's real current text. Diff this output against Step 4's to confirm the only difference is the presence of `content` — proof the privacy gate is real, not cosmetic.

- [ ] **Step 6: Commit**

```bash
git add internal/engine/store.go cmd/ctx/main.go
git commit -m "feat: add ctx export CLI command"
```

---

## Self-Review

**Spec coverage** — `Files[]` tracking added as a minimal prerequisite (Task 1), portable JSON export format (Task 2), `--with-content` opt-in privacy gate (Task 2 + 3), CLI command (Task 3). `ctx inject` intentionally has zero code in this plan, per explicit user decision to defer it pending future discussion — not a gap, a scope boundary.

**Placeholder scan** — no TBD/TODO, no hand-waved error handling; every step has complete code.

**Type consistency** — `provider.FileRef`, `SessionContext.Files`, `sessionState.fileRefs()`, `export.ExportedFile`, `export.ExportedContext`, `export.Build`, `StateStore.Get` are spelled identically everywhere they're defined (Tasks 1-3) and consumed (Tasks 2-3).

---

## What's next

Phase 5 (Custom provider system) is planned and executed next, per explicit instruction. `ctx inject` and OpenCode-side file-tracking remain open, unscoped items for a future plan.
