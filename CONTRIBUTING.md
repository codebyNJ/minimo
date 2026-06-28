# Contributing to minimo

Thanks for taking the time. This guide covers everything you need to go from zero to a merged PR.

---

## Table of contents

- [Prerequisites](#prerequisites)
- [Getting started](#getting-started)
- [Project layout](#project-layout)
- [Build and run](#build-and-run)
- [Testing](#testing)
- [Code style](#code-style)
- [Adding a new provider](#adding-a-new-provider)
- [Opening a pull request](#opening-a-pull-request)
- [Commit messages](#commit-messages)

---

## Prerequisites

| Tool | Minimum version |
|---|---|
| Go | 1.22 |
| Git | any recent |

No CGO. No external runtime dependencies. `go build` is the only build tool you need.

---

## Getting started

```bash
git clone https://github.com/codebyNJ/minimo
cd minimo
go build -o minimo ./cmd/ctx   # builds the binary
go test ./...                  # runs the full test suite
```

The binary reads agent session files from disk and needs no API keys or credentials to run.

---

## Project layout

```
cmd/ctx/            entry point — flag parsing, subcommand dispatch, CLI output
internal/
  config/           YAML config loader, default path (~/.minimo/config.yaml)
  engine/           refresh loop, cost estimation, StateStore
  format/           FormatCount / FormatCost / FormatDuration helpers
  logging/          leveled logger (debug / info / error → ~/.minimo/minimo.log)
  pricing/          LiteLLM catalog parse, live fetch, embedded snapshot fallback
  provider/         shared types (SessionContext, TokenUsage, Cost, …) + registry
    claudecode/     provider: ~/.claude JSONL transcripts, subagent attribution
    codex/          provider: ~/.codex JSONL transcripts
    kimicode/       provider: $KIMI_CODE_HOME JSONL transcripts
    opencode/       provider: ~/.local/share/opencode/opencode.db SQLite
    configprovider/ generic JSONL provider driven by config provider_paths
  tailreader/       incremental JSONL cursor (reads only new bytes since last call)
  ui/               bubbletea TUI — model, update, view, panels, stats screen
  usage/            rolling-window aggregation (24h / 7d / 30d per-model stats)
  watcher/          fsnotify wrapper with debounce and recursive watch
```

Key interfaces — everything else plugs into these:

```go
// internal/provider/provider.go

type Provider interface {
    Name() string
    Detect() bool
    ListSessions() ([]SessionInfo, error)
    ReadContext(sessionID string) (*SessionContext, error)
}

// Optional interfaces (detected via type assertion):
type Watchable   interface { WatchPaths() []string }
type PathReporter interface { CheckedPath() string }
type PlanReporter interface { Plan() PlanInfo }
```

---

## Build and run

```bash
# development build
go build -o minimo ./cmd/ctx

# static release build (matches Dockerfile)
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=v0.x.y" -o minimo ./cmd/ctx

# run the TUI
./minimo

# flat table (good for quick debugging without TUI)
./minimo status

# machine-readable output
./minimo status --json | jq '.[0]'

# restrict to one provider during development
./minimo --provider claude-code

# verbose debug log
./minimo --debug   # writes to ~/.minimo/minimo.log
```

---

## Testing

```bash
go test ./...                              # all packages
go test ./internal/ui/...                  # single package
go test -run TestFormatCost ./internal/format/...  # single test
go vet ./...                               # static analysis
```

**Test conventions:**

- Tests live next to the code they test (`foo.go` / `foo_test.go`), same package.
- Write the failing test first, then implement. Every new exported function needs at least one test.
- No mocking the filesystem — tests that need session files use `os.MkdirTemp` fixtures or the real `tailreader` on temp files.
- Table-driven tests with `map[input]want` are preferred for pure functions.
- The `ui` package tests render output as strings; assert on `strings.Contains`, not exact equality, so cosmetic changes don't break them.

---

## Code style

- `gofmt` is the law. Run it before committing (most editors do this automatically).
- No comments explaining *what* the code does — well-named identifiers do that. Only comment the *why*: a hidden constraint, a non-obvious invariant, a workaround for a specific bug.
- No docstring blocks. One short line maximum.
- No error handling for conditions that cannot happen. Trust package guarantees.
- Validate only at system boundaries (user input, external files, provider APIs). Never inside pure helpers.
- YAGNI — do not add parameters, options, or abstraction layers for hypothetical future needs.
- When a function appears in three places identically, extract it. Two is fine.

---

## Adding a new provider

A provider is a single package under `internal/provider/<name>/` that registers itself via `init()`.

**Step 1 — Create the package**

```
internal/provider/myprovider/
  provider.go     # implements provider.Provider, calls provider.Register in init()
  state.go        # session parsing / incremental state (if JSONL-backed)
  provider_test.go
```

**Step 2 — Implement the interface**

```go
package myprovider

import "github.com/codebyNJ/minimo/internal/provider"

func init() { provider.Register(New()) }

type MyProvider struct{}

func New() *MyProvider { return &MyProvider{} }

func (p *MyProvider) Name() string { return "my-provider" }

func (p *MyProvider) Detect() bool {
    // Return true when this provider's session data exists on disk.
    // Must be cheap — called on every refresh cycle.
    _, err := os.Stat(p.dataPath())
    return err == nil
}

func (p *MyProvider) ListSessions() ([]provider.SessionInfo, error) { ... }

func (p *MyProvider) ReadContext(id string) (*provider.SessionContext, error) { ... }
```

**Step 3 — Wire the import**

Add a blank import in `cmd/ctx/main.go`:

```go
_ "github.com/codebyNJ/minimo/internal/provider/myprovider"
```

**Step 4 — Optional interfaces**

| Interface | Implement when |
|---|---|
| `Watchable` | Provider is file-backed; return the root path(s) to watch with fsnotify |
| `PathReporter` | Provider has a single detectable path; lets the UI show "not found at X" |
| `PlanReporter` | Provider can read account tier (e.g. Pro/Max) from a local non-secret file |

**Step 5 — Token categories**

Populate `TokenUsage` accurately:

```go
provider.TokenUsage{
    Total:         input + output + cacheRead + cacheCreation,
    Input:         input,
    Output:        output,   // include reasoning tokens here
    CacheRead:     cacheRead,
    CacheCreation: cacheCreation,
    Source:        provider.TokenSourceExact,
}
```

Setting `Source: TokenSourceExact` tells the engine not to overwrite the cost with a catalog estimate.

**Step 6 — Tests**

At minimum, test `toTokenUsage` / `toSessionInfo` mappings and that `Detect()` returns false on an empty temp dir.

---

## Opening a pull request

1. **Branch off `main`** — `git checkout -b feat/my-thing`.
2. **Keep scope tight** — one logical change per PR. A refactor and a feature should be separate PRs.
3. **Tests must pass** — `go test ./...` and `go vet ./...` must be green before you open the PR.
4. **No generated files** — do not commit `minimo.exe`, `bin/`, or any build artifact.
5. **Describe the why** in the PR body, not just the what. The diff shows what changed; the description should explain why it was the right change.

---

## Commit messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <short summary>

[optional body — the why, not the what]
```

Common types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`

Examples:

```
feat(provider): add myprovider support for XYZ agent sessions
fix(ui): context bar overflows on terminals narrower than 80 cols
refactor(engine): deduplicate idle threshold constant across providers
docs: update contributing guide with provider step-by-step
```

- Summary line: 72 characters max, imperative mood ("add" not "adds"), no trailing period.
- Body: wrap at 80 characters, explain motivation and constraints, not mechanics.
