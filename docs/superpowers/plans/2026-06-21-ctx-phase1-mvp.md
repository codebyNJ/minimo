# ctx Phase 1 MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the smallest end-to-end slice of `ctx` that proves the architecture works against real data: a Claude Code provider that reads real session files on this machine and a `ctx status` CLI command that lists them, with an optional `--watch` mode that live-updates via the recursive fsnotify watcher.

**Architecture:** Provider interface (`internal/provider`) abstracts session discovery/reading. The Claude Code provider (`internal/provider/claudecode`) implements it using a byte-offset tail reader so repeated reads in `--watch` mode never re-parse bytes already seen. An `Engine` (`internal/engine`) asks every detected provider for sessions and stores results in an in-memory `StateStore`. `cmd/ctx` wires it all together behind one CLI command. No TUI, no export/inject, no second provider yet — see `docs/wiki/Roadmap.md` for what comes after.

**Tech Stack:** Go 1.23+, `github.com/fsnotify/fsnotify` (the only third-party dependency in this phase). No test framework — this plan deliberately has zero `_test.go` files; every task ends with a manual `go build` / `go run` verification step instead.

## Global Constraints

- Module path: `github.com/codebyNJ/minimo` (confirmed with user — see `docs/wiki/Decisions.md`)
- Go version floor: 1.23
- No automated tests, no test files, no testing frameworks — manual run/verify steps only (explicit user instruction)
- No comments explaining *what* code does — only where a non-obvious constraint needs flagging
- Primary dev/verification platform is Windows (this machine) — Windows-specific code (live-PID check) must be real, working code, not a stub
- Every provider/engine/watcher task is verified with `go build ./...`; full behavioral verification happens in Task 8 by running `ctx status` against this machine's real `~/.claude` data
- Dependencies are added with exact versions via `go get`, never hand-edited into `go.mod`

---

### Task 1: Module skeleton + provider interface

**Files:**
- Create: `go.mod`
- Create: `internal/provider/provider.go`
- Create: `internal/provider/registry.go`

**Interfaces:**
- Produces: `provider.SessionStatus` (`StatusActive`, `StatusIdle`, `StatusEnded`), `provider.TokenSource` (`TokenSourceExact`, `TokenSourceEstimated`), `provider.TokenUsage{Total int, Source TokenSource}`, `provider.SessionInfo{ID, Provider, CWD, Label string; Status SessionStatus; StartedAt, LastActive time.Time}`, `provider.SessionContext{Session SessionInfo, Tokens TokenUsage}`, `provider.Provider` interface (`Name() string`, `Detect() bool`, `ListSessions() ([]SessionInfo, error)`, `ReadContext(sessionID string) (*SessionContext, error)`), `provider.Register(p Provider)`, `provider.All() []Provider` — every later task depends on these exact names.

- [ ] **Step 1: Create the module**

Run: `go mod init github.com/codebyNJ/minimo`

Then edit `go.mod` so the go directive reads:

```
module github.com/codebyNJ/minimo

go 1.23
```

- [ ] **Step 2: Write the provider types and interface**

Create `internal/provider/provider.go`:

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

type SessionContext struct {
	Session SessionInfo
	Tokens  TokenUsage
}

type Provider interface {
	Name() string
	Detect() bool
	ListSessions() ([]SessionInfo, error)
	ReadContext(sessionID string) (*SessionContext, error)
}
```

- [ ] **Step 3: Write the provider registry**

Create `internal/provider/registry.go`:

```go
package provider

var registry []Provider

func Register(p Provider) {
	registry = append(registry, p)
}

func All() []Provider {
	return registry
}
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit code 0 (there's no `main` package yet, so this only type-checks the `provider` package).

- [ ] **Step 5: Commit**

```bash
git add go.mod internal/provider/provider.go internal/provider/registry.go
git commit -m "feat: add provider interface and registry"
```

---

### Task 2: Recursive fsnotify watcher with debounce

**Files:**
- Create: `internal/watcher/watcher.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `watcher.Watcher{Events chan string}`, `watcher.New(debounce time.Duration) (*Watcher, error)`, `(*Watcher) AddRecursive(root string) error`, `(*Watcher) Run(ctx context.Context)`, `(*Watcher) Close() error`. Task 8 wires `Watcher.Events` into the engine's refresh loop.

- [ ] **Step 1: Add the fsnotify dependency**

Run: `go get github.com/fsnotify/fsnotify@v1.10.1`

- [ ] **Step 2: Write the watcher**

Create `internal/watcher/watcher.go`:

```go
package watcher

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	fsw      *fsnotify.Watcher
	debounce time.Duration

	mu     sync.Mutex
	timers map[string]*time.Timer

	Events chan string
}

func New(debounce time.Duration) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		fsw:      fsw,
		debounce: debounce,
		timers:   make(map[string]*time.Timer),
		Events:   make(chan string, 64),
	}, nil
}

func (w *Watcher) AddRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		return w.fsw.Add(path)
	})
}

func (w *Watcher) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.handle(ev)
		case _, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
		}
	}
}

func (w *Watcher) handle(ev fsnotify.Event) {
	if ev.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
			_ = w.fsw.Add(ev.Name)
			return
		}
	}
	if ev.Op&(fsnotify.Write|fsnotify.Create) == 0 {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if t, ok := w.timers[ev.Name]; ok {
		t.Stop()
	}
	path := ev.Name
	w.timers[path] = time.AfterFunc(w.debounce, func() {
		w.Events <- path
	})
}

func (w *Watcher) Close() error {
	return w.fsw.Close()
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit code 0.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum internal/watcher/watcher.go
git commit -m "feat: add recursive fsnotify watcher with debounce"
```

---

### Task 3: Claude Code JSONL parsing + tail cursor

**Files:**
- Create: `internal/provider/claudecode/jsonl.go`
- Create: `internal/provider/claudecode/tailcursor.go`

**Interfaces:**
- Produces: `jsonlLine{Type, AITitle, Timestamp, CWD string; Message struct{Usage struct{InputTokens, OutputTokens, CacheReadInputTokens, CacheCreationInputTokens int}}}`, `parseLines(data []byte) []jsonlLine`, `parseTimestamp(s string) (time.Time, bool)`, `tailCursor{path string, offset int64}`, `(*tailCursor) readNew() ([]byte, error)`. Task 4's `sessionState.applyNew` consumes `parseLines`/`parseTimestamp`; Task 5/6 consume `tailCursor`.

- [ ] **Step 1: Write the JSONL line parser**

Create `internal/provider/claudecode/jsonl.go`:

```go
package claudecode

import (
	"bufio"
	"bytes"
	"encoding/json"
	"time"
)

type jsonlLine struct {
	Type      string `json:"type"`
	AITitle   string `json:"aiTitle"`
	Timestamp string `json:"timestamp"`
	CWD       string `json:"cwd"`
	Message   struct {
		Usage struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

func parseLines(data []byte) []jsonlLine {
	var lines []jsonlLine
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		var l jsonlLine
		if err := json.Unmarshal(scanner.Bytes(), &l); err != nil {
			continue
		}
		lines = append(lines, l)
	}
	return lines
}

func parseTimestamp(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
```

- [ ] **Step 2: Write the byte-offset tail cursor**

Create `internal/provider/claudecode/tailcursor.go`:

This only returns **complete** lines. If the file is mid-write and the last line has no trailing `\n` yet, that partial tail is left unconsumed so the next read picks up the now-complete line instead of losing it.

```go
package claudecode

import (
	"bytes"
	"io"
	"os"
)

type tailCursor struct {
	path   string
	offset int64
}

func (c *tailCursor) readNew() ([]byte, error) {
	f, err := os.Open(c.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() < c.offset {
		c.offset = 0
	}

	if _, err := f.Seek(c.offset, io.SeekStart); err != nil {
		return nil, err
	}
	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	last := bytes.LastIndexByte(buf, '\n')
	if last < 0 {
		return nil, nil
	}
	complete := buf[:last+1]
	c.offset += int64(len(complete))
	return complete, nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit code 0.

- [ ] **Step 4: Commit**

```bash
git add internal/provider/claudecode/jsonl.go internal/provider/claudecode/tailcursor.go
git commit -m "feat: add claude code jsonl parser and tail cursor"
```

---

### Task 4: Claude Code session state accumulator

**Files:**
- Create: `internal/provider/claudecode/state.go`

**Interfaces:**
- Consumes: `jsonlLine`, `parseLines`, `parseTimestamp` (Task 3), `provider.SessionInfo`, `provider.SessionStatus` (Task 1).
- Produces: `sessionState{id, cwd, label string; startedAt, lastActive time.Time; tokens int; cursor tailCursor}`, `(*sessionState) applyNew(data []byte)`, `(*sessionState) info(providerName string, status provider.SessionStatus) provider.SessionInfo`. Task 5/6 create and read `sessionState` values.

- [ ] **Step 1: Write the accumulator**

Create `internal/provider/claudecode/state.go`:

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

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit code 0.

- [ ] **Step 3: Commit**

```bash
git add internal/provider/claudecode/state.go
git commit -m "feat: add claude code session state accumulator"
```

---

### Task 5: Live-PID check (Windows + Unix)

**Files:**
- Create: `internal/provider/claudecode/isalive_windows.go`
- Create: `internal/provider/claudecode/isalive_unix.go`

**Interfaces:**
- Produces: `isAlive(pid int) bool`, available to the `claudecode` package regardless of `GOOS` via build tags. Task 6 calls this to decide `StatusActive`.

- [ ] **Step 1: Write the Windows implementation**

Create `internal/provider/claudecode/isalive_windows.go`:

```go
//go:build windows

package claudecode

import (
	"os/exec"
	"strconv"
	"strings"
)

func isAlive(pid int) bool {
	out, err := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/NH").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), strconv.Itoa(pid))
}
```

- [ ] **Step 2: Write the Unix implementation**

Create `internal/provider/claudecode/isalive_unix.go`:

```go
//go:build !windows

package claudecode

import (
	"os"
	"syscall"
)

func isAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
```

- [ ] **Step 3: Verify it compiles on this machine (Windows)**

Run: `go build ./...`
Expected: no output, exit code 0. (Only `isalive_windows.go` is compiled on this machine; the build tag excludes `isalive_unix.go` here, and vice versa on Linux/macOS.)

- [ ] **Step 4: Commit**

```bash
git add internal/provider/claudecode/isalive_windows.go internal/provider/claudecode/isalive_unix.go
git commit -m "feat: add cross-platform live-pid check"
```

---

### Task 6: Claude Code provider — Detect, ListSessions, ReadContext

**Files:**
- Create: `internal/provider/claudecode/provider.go`

**Interfaces:**
- Consumes: `provider.Register`, `provider.Provider`, `provider.SessionInfo`, `provider.SessionContext`, `provider.SessionStatus`, `provider.TokenUsage`, `provider.TokenSourceExact` (Task 1); `sessionState`, `tailCursor` (Tasks 3-4); `isAlive` (Task 5).
- Produces: `claudecode.New() *ClaudeCodeProvider`, `(*ClaudeCodeProvider)` implementing `provider.Provider` fully. Registered automatically via `init()`. Task 8's blank-imports this package to trigger registration.

- [ ] **Step 1: Write the provider**

Create `internal/provider/claudecode/provider.go`:

```go
package claudecode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/codebyNJ/minimo/internal/provider"
)

const idleThreshold = 30 * time.Second

func init() {
	provider.Register(New())
}

type ClaudeCodeProvider struct {
	home string

	mu       sync.Mutex
	sessions map[string]*sessionState
}

func New() *ClaudeCodeProvider {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return &ClaudeCodeProvider{
		home:     filepath.Join(home, ".claude"),
		sessions: make(map[string]*sessionState),
	}
}

func (p *ClaudeCodeProvider) Name() string { return "claude-code" }

func (p *ClaudeCodeProvider) projectsDir() string { return filepath.Join(p.home, "projects") }
func (p *ClaudeCodeProvider) liveDir() string      { return filepath.Join(p.home, "sessions") }

func (p *ClaudeCodeProvider) Detect() bool {
	info, err := os.Stat(p.projectsDir())
	return err == nil && info.IsDir()
}

type liveEntry struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
}

func (p *ClaudeCodeProvider) loadLiveRegistry() map[string]liveEntry {
	out := make(map[string]liveEntry)
	entries, err := os.ReadDir(p.liveDir())
	if err != nil {
		return out
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(p.liveDir(), e.Name()))
		if err != nil {
			continue
		}
		var le liveEntry
		if err := json.Unmarshal(data, &le); err != nil {
			continue
		}
		out[le.SessionID] = le
	}
	return out
}

func (p *ClaudeCodeProvider) statusFor(id string, s *sessionState, live map[string]liveEntry) provider.SessionStatus {
	if entry, ok := live[id]; ok && isAlive(entry.PID) {
		return provider.StatusActive
	}
	if !s.lastActive.IsZero() && time.Since(s.lastActive) < idleThreshold {
		return provider.StatusIdle
	}
	return provider.StatusEnded
}

func (p *ClaudeCodeProvider) ListSessions() ([]provider.SessionInfo, error) {
	projectDirs, err := os.ReadDir(p.projectsDir())
	if err != nil {
		return nil, err
	}

	live := p.loadLiveRegistry()

	p.mu.Lock()
	defer p.mu.Unlock()

	var out []provider.SessionInfo
	for _, pd := range projectDirs {
		if !pd.IsDir() {
			continue
		}
		dir := filepath.Join(p.projectsDir(), pd.Name())
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || filepath.Ext(f.Name()) != ".jsonl" {
				continue
			}
			id := strings.TrimSuffix(f.Name(), ".jsonl")
			state, ok := p.sessions[id]
			if !ok {
				state = &sessionState{id: id, cursor: tailCursor{path: filepath.Join(dir, f.Name())}}
				p.sessions[id] = state
			}
			out = append(out, state.info(p.Name(), p.statusFor(id, state, live)))
		}
	}
	return out, nil
}

func (p *ClaudeCodeProvider) ReadContext(sessionID string) (*provider.SessionContext, error) {
	p.mu.Lock()
	state, ok := p.sessions[sessionID]
	p.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("claudecode: unknown session %q (call ListSessions first)", sessionID)
	}

	data, err := state.cursor.readNew()
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	state.applyNew(data)
	live := p.loadLiveRegistry()
	return &provider.SessionContext{
		Session: state.info(p.Name(), p.statusFor(sessionID, state, live)),
		Tokens:  provider.TokenUsage{Total: state.tokens, Source: provider.TokenSourceExact},
	}, nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit code 0.

- [ ] **Step 3: Commit**

```bash
git add internal/provider/claudecode/provider.go
git commit -m "feat: implement claude code provider"
```

---

### Task 7: Engine — config, state store, refresh orchestration

**Files:**
- Create: `internal/engine/config.go`
- Create: `internal/engine/store.go`
- Create: `internal/engine/engine.go`

**Interfaces:**
- Consumes: `provider.Provider`, `provider.All()`, `provider.SessionContext` (Task 1).
- Produces: `engine.DebounceDefault time.Duration`, `engine.StateStore`, `engine.NewStateStore() *StateStore`, `(*StateStore) Put(sessionID string, ctx provider.SessionContext)`, `(*StateStore) All() []provider.SessionContext`, `engine.Engine{Store *StateStore}`, `engine.New() *Engine`, `(*Engine) Refresh() error`. Task 8 calls `engine.New()` and `(*Engine).Refresh()`.

- [ ] **Step 1: Write the config defaults**

Create `internal/engine/config.go`:

```go
package engine

import "time"

const DebounceDefault = 500 * time.Millisecond
```

- [ ] **Step 2: Write the in-memory state store**

Create `internal/engine/store.go`:

```go
package engine

import (
	"sync"

	"github.com/codebyNJ/minimo/internal/provider"
)

type StateStore struct {
	mu    sync.RWMutex
	items map[string]provider.SessionContext
}

func NewStateStore() *StateStore {
	return &StateStore{items: make(map[string]provider.SessionContext)}
}

func (s *StateStore) Put(sessionID string, ctx provider.SessionContext) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[sessionID] = ctx
}

func (s *StateStore) All() []provider.SessionContext {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]provider.SessionContext, 0, len(s.items))
	for _, c := range s.items {
		out = append(out, c)
	}
	return out
}
```

- [ ] **Step 3: Write the orchestrator**

Create `internal/engine/engine.go`:

```go
package engine

import (
	"github.com/codebyNJ/minimo/internal/provider"
)

type Engine struct {
	providers []provider.Provider
	Store     *StateStore
}

func New() *Engine {
	return &Engine{
		providers: provider.All(),
		Store:     NewStateStore(),
	}
}

func (e *Engine) Refresh() error {
	for _, p := range e.providers {
		if !p.Detect() {
			continue
		}
		sessions, err := p.ListSessions()
		if err != nil {
			continue
		}
		for _, s := range sessions {
			ctx, err := p.ReadContext(s.ID)
			if err != nil {
				continue
			}
			e.Store.Put(s.ID, *ctx)
		}
	}
	return nil
}
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit code 0.

- [ ] **Step 5: Commit**

```bash
git add internal/engine/config.go internal/engine/store.go internal/engine/engine.go
git commit -m "feat: add engine orchestrator and in-memory state store"
```

---

### Task 8: CLI entrypoint — `ctx status` and `ctx status --watch`

**Files:**
- Create: `cmd/ctx/main.go`

**Interfaces:**
- Consumes: `engine.New()`, `(*Engine).Refresh()`, `(*Engine).Store.All()`, `engine.DebounceDefault` (Task 7); `watcher.New()`, `(*Watcher).AddRecursive()`, `(*Watcher).Run()`, `(*Watcher).Events`, `(*Watcher).Close()` (Task 2); blank-imports `claudecode` (Task 6) to trigger its `init()` registration.
- Produces: the `ctx` binary. Nothing downstream depends on this — it's the leaf of Phase 1.

- [ ] **Step 1: Write the entrypoint**

Create `cmd/ctx/main.go`. `--watch` mode is event-driven off the Task 2 watcher (not a fixed-interval ticker) — every refresh is triggered by a real, debounced filesystem change, so the watcher built in Task 2 has an actual consumer instead of sitting unused:

```go
package main

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

func main() {
	if len(os.Args) < 2 || os.Args[1] != "status" {
		fmt.Fprintln(os.Stderr, "usage: ctx status [--watch]")
		os.Exit(1)
	}
	runStatus(os.Args[2:])
}

func runStatus(args []string) {
	watch := false
	for _, a := range args {
		if a == "--watch" {
			watch = true
		}
	}

	e := engine.New()

	if !watch {
		if err := e.Refresh(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		printTable(e)
		return
	}

	runWatch(e)
}

func runWatch(e *engine.Engine) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	projectsDir := filepath.Join(home, ".claude", "projects")

	w, err := watcher.New(engine.DebounceDefault)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer w.Close()
	if err := w.AddRecursive(projectsDir); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
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

func printTable(e *engine.Engine) {
	rows := e.Store.All()
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Session.LastActive.After(rows[j].Session.LastActive)
	})

	fmt.Printf("%-12s %-8s %-8s %-10s %-24s %s\n", "PROVIDER", "STATUS", "TOKENS", "LAST", "CWD", "LABEL")
	for _, r := range rows {
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

- [ ] **Step 2: Build the binary**

Run: `go build -o bin/ctx.exe ./cmd/ctx`
Expected: no output, exit code 0, `bin/ctx.exe` exists.

- [ ] **Step 3: Manually verify against this machine's real Claude Code data**

Run: `./bin/ctx.exe status`

Expected: a table with at least one row for `claude-code`, including a row whose `CWD` ends in `minimo` (this project) — this is the live session running this exact plan. Status should read `active` for that row (its PID file exists under `~/.claude/sessions/` and the process is alive), `TOKENS` should be a positive integer, and `LABEL` should be a real session title pulled from an `ai-title` line (not empty, not a placeholder).

- [ ] **Step 4: Manually verify watch mode**

Run: `./bin/ctx.exe status --watch`

Expected: the table prints immediately, then reprints only when a watched session file actually changes — roughly 500ms (the debounce window) after this very session's next turn gets appended to its `.jsonl` file, not on a fixed timer. The `TOKENS` value for the active session increases across refreshes without the command becoming slower (proof the tail cursor is working — it should not re-read megabytes of already-seen file on each refresh). Press `Ctrl+C`: the process exits immediately and returns control to the shell (proof graceful shutdown via `signal.NotifyContext` is wired correctly, and the watcher's goroutine doesn't block exit).

- [ ] **Step 5: Commit**

```bash
git add cmd/ctx/main.go
git commit -m "feat: add ctx status CLI command"
```

---

## Self-Review

**Spec coverage** — every Roadmap Phase 1 bullet has a task: provider interface (Task 1), watcher (Task 2), Claude Code provider including tail reader/exact tokens/ai-title label/live-PID status (Tasks 3-6), engine/store (Task 7), `ctx status` with no TUI (Task 8). `Decisions` field intentionally absent per `docs/wiki/Decisions.md`. `Files`/file-tracking intentionally deferred to Phase 3 (the TUI's "files: N" metric) — not built here since nothing in Phase 1 consumes it yet.

**Placeholder scan** — no TBD/TODO, no "add error handling" hand-waving, no "similar to Task N" shortcuts; every step has complete code.

**Type consistency** — `provider.SessionInfo`, `provider.SessionContext`, `provider.TokenUsage`, `sessionState`, `tailCursor`, `ClaudeCodeProvider`, `engine.StateStore`, `engine.Engine` are spelled identically everywhere they're constructed (Tasks 1-7) and consumed (Tasks 6-8).

---

## What's next

Phase 2 (remaining providers: Codex, Cursor, Gemini CLI) starts only after this MVP is verified against real data per Task 8 — see `docs/wiki/Roadmap.md`. Two open items from `docs/wiki/Decisions.md` need resolving before Phase 2 begins: confirming whether Codex/Gemini CLI have a live-session registry, and re-verifying Cursor's current SQLite schema against a real install.
