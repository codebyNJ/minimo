# ctx Phase 5 — Custom Provider System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce the tool's first real config file (`~/.ctx/config.yaml`, covering debounce/poll-interval/enabled-providers) and a YAML-based custom provider system (`~/.ctx/providers/*.yaml`) so users can monitor arbitrary log-based coding agents without writing Go code, per `draft.md`'s original design.

**Architecture:** A new `internal/config` package loads `~/.ctx/config.yaml` (gracefully defaulting if absent) and is threaded through `engine.New(cfg)`, which now filters the provider registry by `cfg.EnabledProviders`. A new `internal/provider/configprovider` package implements the generic, regex/glob-driven `Provider` interface described in `draft.md`'s `### External Provider Discovery` section: each YAML file under `~/.ctx/providers/` becomes one dynamically-constructed provider where each file matched by `monitor.paths` is exactly one session (session ID = file path), `session_pattern` extracts a label, `token_pattern` matches are summed, and `file_pattern` matches become `Files[]`. Unlike compiled-in providers (self-registered via `init()`), these are loaded and registered explicitly in `main()` since their existence isn't known until the YAML files are read at runtime.

**Tech Stack:** Go 1.23+, `gopkg.in/yaml.v3` (exact version `v3.0.1` — confirmed latest stable via `go list -m -versions gopkg.in/yaml.v3` on 2026-06-21). No test framework — zero `_test.go` files, manual `go build` / `go run` verification only, using a real demo provider YAML + log file created and then removed during Task 3's verification.

## Global Constraints

- Module path: `github.com/codebyNJ/minimo`
- No automated tests, no test files — manual run/verify steps only
- No comments explaining *what* code does — only where non-obvious
- Primary dev/verification platform is Windows (this machine)
- This is the tool's **first and only** config file format — per `docs/wiki/Architecture.md#7-minimal-config-not-zero-config`, don't build a second one. `internal/engine/config.go`'s old hardcoded constants (`DebounceDefault`, `PollIntervalDefault`) are removed and superseded by `internal/config.Default()` to avoid two sources of truth for the same values.
- **Scope decisions confirmed with user, 2026-06-21:**
  - One session per matched file (session ID = file path) — not one session per regex match within a file.
  - `token_pattern` matches are **summed**, not "take latest" — correct for tools that log per-turn deltas (the common case), unlike Codex's unusual cumulative-counter format documented in `docs/wiki/Research.md`.
  - Custom-provider token counts are always `TokenSourceEstimated`, never `TokenSourceExact` — `ctx` cannot verify an arbitrary user-supplied regex captures complete/correct token data the way a structured API response can, so the conservative, honest label applies regardless of how precise the underlying tool's own numbers might be.
  - `draft.md`'s `export:`/`inject:` sections of the custom-provider YAML schema are **not parsed** in this plan — `ctx inject` doesn't exist yet (deferred per the Phase 4 plan) and `ctx export` already works uniformly across any `provider.Provider` without per-provider config, so those fields would be unused dead config if parsed now.
  - Per-provider idle thresholds (`idleThreshold` constants in `claudecode`/`opencode`/this plan's `configprovider`) are **not** wired to config in this plan — compiled-in providers self-register via `init()`, before any config is loaded, so making their thresholds configurable needs a registration-pattern change that's out of scope here. Named explicitly as a gap, not silently skipped.

---

### Task 1: `internal/config` package

**Files:**
- Create: `internal/config/config.go`

**Interfaces:**
- Consumes: nothing from earlier tasks (first task in this plan).
- Produces: `config.Config{DebounceMS, PollIntervalSec int; EnabledProviders []string}`, `config.Default() Config`, `config.DefaultPath() string`, `config.Load(path string) (Config, error)`, `(Config) Debounce() time.Duration`, `(Config) PollInterval() time.Duration`. Task 3 consumes all of these by exact name.

- [ ] **Step 1: Add the yaml.v3 dependency**

Run: `go get gopkg.in/yaml.v3@v3.0.1`

- [ ] **Step 2: Write the config package**

Create `internal/config/config.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DebounceMS       int      `yaml:"debounce_ms"`
	PollIntervalSec  int      `yaml:"poll_interval_seconds"`
	EnabledProviders []string `yaml:"enabled_providers"`
}

func Default() Config {
	return Config{
		DebounceMS:      500,
		PollIntervalSec: 2,
	}
}

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return filepath.Join(home, ".ctx", "config.yaml")
}

func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c Config) Debounce() time.Duration {
	return time.Duration(c.DebounceMS) * time.Millisecond
}

func (c Config) PollInterval() time.Duration {
	return time.Duration(c.PollIntervalSec) * time.Second
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit code 0.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum internal/config/config.go
git commit -m "feat: add yaml-based config package"
```

---

### Task 2: `internal/provider/configprovider` package

**Files:**
- Create: `internal/provider/configprovider/spec.go`
- Create: `internal/provider/configprovider/provider.go`

**Interfaces:**
- Consumes: `provider.Provider`, `provider.SessionInfo`, `provider.SessionContext`, `provider.SessionStatus`, `provider.StatusIdle`, `provider.StatusEnded`, `provider.TokenUsage`, `provider.TokenSourceEstimated`, `provider.FileRef` (Phase 1 + Phase 4).
- Produces: `configprovider.DefaultDir() string`, `configprovider.LoadAll(dir string) []*Provider`, `(*Provider)` implementing `provider.Provider` fully. Task 3 consumes `DefaultDir`, `LoadAll`, and registers each returned `*Provider` via `provider.Register`.

- [ ] **Step 1: Write the YAML schema and loader**

Create `internal/provider/configprovider/spec.go`:

```go
package configprovider

import (
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

type spec struct {
	Name    string `yaml:"name"`
	Version int    `yaml:"version"`
	Monitor struct {
		Paths []string `yaml:"paths"`
		Parse struct {
			Format         string `yaml:"format"`
			SessionPattern string `yaml:"session_pattern"`
			TokenPattern   string `yaml:"token_pattern"`
			FilePattern    string `yaml:"file_pattern"`
		} `yaml:"parse"`
	} `yaml:"monitor"`
}

func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return filepath.Join(home, ".ctx", "providers")
}

func LoadAll(dir string) []*Provider {
	matches, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil
	}
	var out []*Provider
	for _, path := range matches {
		p, err := loadOne(path)
		if err != nil {
			continue
		}
		out = append(out, p)
	}
	return out
}

func loadOne(path string) (*Provider, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s spec
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	p := &Provider{spec: s}
	if s.Monitor.Parse.SessionPattern != "" {
		p.sessionRe, err = regexp.Compile(s.Monitor.Parse.SessionPattern)
		if err != nil {
			return nil, err
		}
	}
	if s.Monitor.Parse.TokenPattern != "" {
		p.tokenRe, err = regexp.Compile(s.Monitor.Parse.TokenPattern)
		if err != nil {
			return nil, err
		}
	}
	if s.Monitor.Parse.FilePattern != "" {
		p.fileRe, err = regexp.Compile(s.Monitor.Parse.FilePattern)
		if err != nil {
			return nil, err
		}
	}
	return p, nil
}
```

- [ ] **Step 2: Write the dynamic provider**

Create `internal/provider/configprovider/provider.go`:

```go
package configprovider

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/codebyNJ/minimo/internal/provider"
)

const idleThreshold = 30 * time.Second

type Provider struct {
	spec      spec
	sessionRe *regexp.Regexp
	tokenRe   *regexp.Regexp
	fileRe    *regexp.Regexp
}

func (p *Provider) Name() string { return p.spec.Name }

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}

func (p *Provider) matchedFiles() []string {
	var out []string
	for _, pattern := range p.spec.Monitor.Paths {
		matches, err := filepath.Glob(expandHome(pattern))
		if err != nil {
			continue
		}
		out = append(out, matches...)
	}
	return out
}

func (p *Provider) Detect() bool {
	return len(p.matchedFiles()) > 0
}

func (p *Provider) statusFor(modTime time.Time) provider.SessionStatus {
	if time.Since(modTime) < idleThreshold {
		return provider.StatusIdle
	}
	return provider.StatusEnded
}

func (p *Provider) sessionInfo(path string) (provider.SessionInfo, []byte, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return provider.SessionInfo{}, nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return provider.SessionInfo{}, nil, err
	}

	label := ""
	if p.sessionRe != nil {
		if m := p.sessionRe.FindStringSubmatch(string(data)); len(m) > 1 {
			label = m[1]
		}
	}

	return provider.SessionInfo{
		ID:         path,
		Provider:   p.Name(),
		CWD:        filepath.Dir(path),
		Label:      label,
		Status:     p.statusFor(fi.ModTime()),
		StartedAt:  fi.ModTime(),
		LastActive: fi.ModTime(),
	}, data, nil
}

func (p *Provider) ListSessions() ([]provider.SessionInfo, error) {
	var out []provider.SessionInfo
	for _, path := range p.matchedFiles() {
		info, _, err := p.sessionInfo(path)
		if err != nil {
			continue
		}
		out = append(out, info)
	}
	return out, nil
}

func (p *Provider) ReadContext(sessionID string) (*provider.SessionContext, error) {
	info, data, err := p.sessionInfo(sessionID)
	if err != nil {
		return nil, err
	}

	tokens := 0
	if p.tokenRe != nil {
		for _, m := range p.tokenRe.FindAllStringSubmatch(string(data), -1) {
			if len(m) > 1 {
				if n, err := strconv.Atoi(m[1]); err == nil {
					tokens += n
				}
			}
		}
	}

	var files []provider.FileRef
	if p.fileRe != nil {
		seen := make(map[string]bool)
		for _, m := range p.fileRe.FindAllStringSubmatch(string(data), -1) {
			if len(m) > 1 && !seen[m[1]] {
				seen[m[1]] = true
				files = append(files, provider.FileRef{Path: m[1]})
			}
		}
	}

	return &provider.SessionContext{
		Session: info,
		Tokens:  provider.TokenUsage{Total: tokens, Source: provider.TokenSourceEstimated},
		Files:   files,
	}, nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit code 0.

- [ ] **Step 4: Commit**

```bash
git add internal/provider/configprovider/spec.go internal/provider/configprovider/provider.go
git commit -m "feat: add yaml-based custom provider system"
```

---

### Task 3: Wire config + custom providers into the engine and CLI

**Files:**
- Modify: `internal/engine/engine.go`
- Delete: `internal/engine/config.go`
- Modify: `cmd/ctx/main.go`

**Interfaces:**
- Consumes: `config.Config`, `config.Load`, `config.DefaultPath`, `(Config).Debounce`, `(Config).PollInterval` (Task 1); `configprovider.DefaultDir`, `configprovider.LoadAll` (Task 2).
- Produces: `engine.New(cfg config.Config) *Engine` (signature change). Nothing downstream depends on this — it's the leaf of this plan.

- [ ] **Step 1: Filter providers by config in the engine**

Modify `internal/engine/engine.go` — change from:

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
```

to:

```go
package engine

import (
	"github.com/codebyNJ/minimo/internal/config"
	"github.com/codebyNJ/minimo/internal/provider"
)

type Engine struct {
	providers []provider.Provider
	Store     *StateStore
}

func New(cfg config.Config) *Engine {
	return &Engine{
		providers: filterEnabled(provider.All(), cfg.EnabledProviders),
		Store:     NewStateStore(),
	}
}

func filterEnabled(all []provider.Provider, enabled []string) []provider.Provider {
	if len(enabled) == 0 {
		return all
	}
	allowed := make(map[string]bool, len(enabled))
	for _, name := range enabled {
		allowed[name] = true
	}
	var out []provider.Provider
	for _, p := range all {
		if allowed[p.Name()] {
			out = append(out, p)
		}
	}
	return out
}
```

(`Refresh()` below this is unchanged.)

- [ ] **Step 2: Delete the superseded constants file**

Run: `git rm internal/engine/config.go`

- [ ] **Step 3: Rewrite the CLI entrypoint to load config and custom providers**

Replace the full contents of `cmd/ctx/main.go` with:

```go
package main

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

	"github.com/codebyNJ/minimo/internal/config"
	"github.com/codebyNJ/minimo/internal/engine"
	"github.com/codebyNJ/minimo/internal/export"
	"github.com/codebyNJ/minimo/internal/provider"
	_ "github.com/codebyNJ/minimo/internal/provider/claudecode"
	"github.com/codebyNJ/minimo/internal/provider/configprovider"
	_ "github.com/codebyNJ/minimo/internal/provider/opencode"
	"github.com/codebyNJ/minimo/internal/watcher"
)

func main() {
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: failed to load config:", err)
		os.Exit(1)
	}
	for _, p := range configprovider.LoadAll(configprovider.DefaultDir()) {
		provider.Register(p)
	}

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: ctx status [--watch] | ctx export <session-id> [--with-content]")
		os.Exit(1)
	}
	switch os.Args[1] {
	case "status":
		runStatus(os.Args[2:], cfg)
	case "export":
		runExport(os.Args[2:], cfg)
	default:
		fmt.Fprintln(os.Stderr, "usage: ctx status [--watch] | ctx export <session-id> [--with-content]")
		os.Exit(1)
	}
}

func runStatus(args []string, cfg config.Config) {
	watch := false
	for _, a := range args {
		if a == "--watch" {
			watch = true
		}
	}

	e := engine.New(cfg)

	if !watch {
		if err := e.Refresh(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		printTable(e)
		return
	}

	runWatch(e, cfg)
}

func runExport(args []string, cfg config.Config) {
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

	e := engine.New(cfg)
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

func runWatch(e *engine.Engine, cfg config.Config) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	projectsDir := filepath.Join(home, ".claude", "projects")

	w, err := watcher.New(cfg.Debounce())
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

	ticker := time.NewTicker(cfg.PollInterval())
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

- [ ] **Step 4: Build the binary**

Run: `go build -o bin/ctx.exe ./cmd/ctx`
Expected: no output, exit code 0.

- [ ] **Step 5: Manually verify the custom provider end-to-end**

Create a real demo custom provider and a real log file for it to parse:

```powershell
New-Item -ItemType Directory -Force "$env:USERPROFILE\.ctx\providers" | Out-Null
New-Item -ItemType Directory -Force "$env:USERPROFILE\.ctx-demo" | Out-Null
Set-Content -Path "$env:USERPROFILE\.ctx\providers\demo-agent.yaml" -Encoding utf8 -Value @'
name: demo-agent
version: 1
monitor:
  paths:
    - "~/.ctx-demo/demo.log"
  parse:
    format: log
    session_pattern: "session: (\S+)"
    token_pattern: "tokens: (\d+)"
    file_pattern: "file: (\S+)"
'@
Set-Content -Path "$env:USERPROFILE\.ctx-demo\demo.log" -Encoding utf8 -Value @'
session: demo-session-1
tokens: 100
file: /tmp/a.txt
tokens: 50
file: /tmp/b.txt
'@
```

Run: `.\bin\ctx.exe status`

Expected: a new row with `PROVIDER` = `demo-agent`, `TOKENS` = `150` (100 + 50, summed per the confirmed aggregation rule), `LABEL` = `demo-session-1` (extracted via `session_pattern`).

- [ ] **Step 6: Manually verify the `enabled_providers` config filter**

```powershell
Set-Content -Path "$env:USERPROFILE\.ctx\config.yaml" -Encoding utf8 -Value @'
enabled_providers:
  - opencode
'@
```

Run: `.\bin\ctx.exe status`

Expected: **only** `opencode` rows are listed — `claude-code` and `demo-agent` rows are gone, proving `cfg.EnabledProviders` actually filters `provider.All()` rather than being parsed-and-ignored.

- [ ] **Step 7: Clean up the demo artifacts and confirm the baseline returns**

```powershell
Remove-Item -Force "$env:USERPROFILE\.ctx\config.yaml"
Remove-Item -Recurse -Force "$env:USERPROFILE\.ctx-demo"
Remove-Item -Force "$env:USERPROFILE\.ctx\providers\demo-agent.yaml"
```

Run: `.\bin\ctx.exe status`

Expected: back to `claude-code` and `opencode` rows only — no `demo-agent`, no filtering — confirming both the config file and the custom provider are correctly optional/absent-tolerant.

- [ ] **Step 8: Commit**

```bash
git add internal/engine/engine.go cmd/ctx/main.go
git commit -m "feat: wire yaml config and custom providers into ctx CLI"
```

---

## Self-Review

**Spec coverage** — `~/.ctx/config.yaml` (debounce/poll-interval/enabled-providers) is the tool's first config file (Task 1), consumed by the engine (Task 3 Step 1) and CLI (Task 3 Step 3). `~/.ctx/providers/*.yaml` custom providers (Task 2) match `draft.md`'s schema for `monitor`/`parse`, minus the intentionally-unparsed `export`/`inject` sections. Confirmed scope decisions (one session per file, sum token matches, always-estimated token source) are all implemented exactly as decided, not reinterpreted.

**Placeholder scan** — no TBD/TODO, no hand-waved error handling; every step has complete code, including the full verification YAML/log fixtures (not "add a test config" hand-waving).

**Type consistency** — `config.Config`, `config.Load`, `(Config).Debounce`/`PollInterval` (Task 1) match exactly what Task 3's `engine.New`/`main.go` consume. `configprovider.DefaultDir`, `configprovider.LoadAll`, `*configprovider.Provider` (Task 2) match exactly what Task 3's `main()` consumes.

---

## What's next

`ctx inject`, OpenCode-side file-tracking, Codex/Kimi Code providers, and per-provider configurable idle thresholds all remain open, unscoped items — named explicitly above rather than silently dropped. Phase 3 (TUI) was skipped entirely in this run; `ctx status` remains a flat CLI table.
