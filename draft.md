---
title: ctx — Multi-Agent Context Hub
created: 2026-06-20
tags: [idea, design, research, prototype]
status: active
---

# ctx — Multi-Agent Context Hub

## Core Concept

A **terminal-native TUI** that monitors live context across all your AI coding agents (Claude Code, Codex, Cursor, Gemini CLI, etc.) in one dashboard. Think `htop` / `lazygit` for AI coding agent context.

## What It Does

```
ctx                    → Open TUI: live dashboard of all agents
ctx status             → CLI: quick status of all monitored agents
ctx export <agent>     → Export context from one agent
ctx inject <agent>     → Inject context into another agent
ctx providers          → List installed providers
ctx provider add       → Add a custom provider
```

### TUI Dashboard

```
┌─────────────────────────────────────────────────────────┐
│  ctx  —  Context Hub                             Ctrl+Q │
├───────────┬───────────┬───────────┬─────────────────────┤
│ Claude    │ Codex     │ Cursor    │ Gemini CLI          │
│ Code      │           │           │                     │
│ ● live    │ ● live    │ ○ idle    │ ● live              │
│ session:  │ session:  │ session:  │ session:            │
│ 45m       │ 12m       │ --        │ 8m                  │
│ files: 23 │ files: 8  │ files: -- │ files: 5            │
│ tokens:   │ tokens:   │           │ tokens:             │
│ 87K       │ 34K       │           │ 12K                 │
│           │           │           │                     │
│ [Export]  │ [Export]  │ [Export]  │ [Export]            │
│ [Inject←] │ [Inject←] │ [Inject←] │ [Inject←]           │
├───────────┴───────────┴───────────┴─────────────────────┤
│ Providers: claude-code ✓ | codex ✓ | cursor ✓ | ...     │
│ Custom: my-ollama-agent ⚡ needs import                   │
└─────────────────────────────────────────────────────────┘
```

## Key Features

1. **Live monitoring** — what each agent is working on, files read, decisions made, token usage
2. **Cross-agent export/inject** — carry context from Claude Code → Codex seamlessly (e.g., when hitting rate limits)
3. **Provider system** — built-in readers for major agents; user-defined providers for anything else
4. **One binary, zero AI** — pure monitoring utility, no API keys, no models
5. **Local-first** — all data stays on your machine

## Differentiation

| Dimension | Existing tools | ctx |
|---|---|---|
| Interface | CLI / Web / MCP daemon | **Native TUI** |
| Focus | Sessions OR code graphs OR context files | **Live context monitoring** |
| Agent scope | Single agent | **Multi-agent** |
| Cross-agent | No | **Export/Inject** between agents |
| Extensibility | Fixed | **Provider system** |
| Philosophy | AI-powered | **Utility tool** (zero AI) |

## Competitive Landscape

### CLAUDE.md Generators
- agentmd, context.md, awesome-claude-md, Forge/AnchorMD
- CLI one-shot → generate CLAUDE.md/AGENTS.md
- No TUI, no monitoring, no multi-agent

### Code Knowledge Graphs
- GitNexus, code-graph-mcp
- Parse codebase → graph DB → MCP tools
- Heavy infra, code structure focus, not context monitoring

### Session/Workspace Managers
- Myrlin Workbook, ClaudeCodeUI, Opcode
- Browser-based dashboard for sessions
- Web GUI, not terminal-native, session-focused not context-focused

### Context Pruning
- cozempic, claude-notify
- Background daemon for session health
- Narrow focus, no TUI, no cross-agent

### Knowledge Bases
- anchormd, recuerd0
- Markdown knowledge bases for agents
- CLI or SaaS, no TUI, no monitoring

## Provider System (Draft)

Each provider knows how to:
1. Detect the agent (is it installed?)
2. Read its session/context data (files, format, location)
3. Monitor live state (what's happening right now)
4. Export context (serialize to portable format)
5. Inject context (deserialize and inject into agent)

```go
type Provider interface {
    Name() string
    Detect() bool                    // Is this agent installed?
    Sessions() ([]Session, error)    // Active sessions
    SessionContext(id string) (Context, error)  // Context for a session
    Export(sessionID string) ([]byte, error)
    Inject(ctx []byte) error
}

type Session struct {
    ID        string
    Agent     string
    StartedAt time.Time
    Status    string  // active, idle, paused
    FilesRead int
    TokensUsed int
}

type Context struct {
    Files     []string
    Decisions []string
    Env       map[string]string
    Session   Session
}
```

## Research Needed

- [ ] What session data does each agent expose? (JSONL files, API, log files)
- [ ] Claude Code: `~/.claude/projects/<id>/` — what's the JSONL format?
- [ ] Codex: `~/.codex/` — session storage format?
- [ ] Cursor: workspace session data location?
- [ ] Gemini CLI: session context format?
- [ ] Feasibility of live monitoring (file watching, polling, hooks)
- [ ] Context export format design (portable across agents)
- [ ] Provider SDK design (how easy can we make it to add a new agent?)
- [ ] Token counting for context window estimation

## Next Steps

- [x] Validate need (Reddit research complete)
- [x] Competitive audit (mapped 12+ existing tools)
- [x] Deep research on agent internals (Claude Code, Codex, Cursor, Gemini CLI)
- [x] MCP feasibility assessment (dead end — logging deprecated in favor of OTEL)
- [x] File watcher research (fsnotify v1.10.1 confirmed, recursive watch needed)
- [x] Design provider architecture (detailed below)
- [ ] Build TUI prototype

---

## Architecture Overview

### Component Diagram

```
┌──────────────────────────────────────────────────────────────┐
│                     TUI (Bubble Tea)                          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐   │
│  │ Claude   │ │ Codex    │ │ Cursor   │ │ Gemini CLI   │   │
│  │ Code     │ │          │ │          │ │              │   │
│  │ Panel    │ │ Panel    │ │ Panel    │ │ Panel        │   │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └──────┬───────┘   │
│       │            │            │              │            │
│       └────────────┴────────────┴──────────────┘            │
│                            │                                 │
│                    Event Bus (chan tea.Msg)                  │
│                            │                                 │
│                      Model.Update()                           │
└────────────────────────────┼─────────────────────────────────┘
                             │
              ┌──────────────┴──────────────┐
              │        Engine / Orch.        │
              │  - Provider registry          │
              │  - Watcher pool               │
              │  - State store (in-memory)    │
              │  - Export/Inject coordinator  │
              └────┬──────┬──────┬────────────┘
                   │      │      │
     ┌─────────────┘      │      └─────────────┐
     │                    │                    │
┌────▼────┐        ┌──────▼──────┐      ┌──────▼──────┐
│Claude   │        │ Codex       │      │ Cursor       │
│Provider │        │ Provider    │      │ Provider     │
│ (JSONL) │        │ (JSONL+SQL) │      │ (SQLite)     │
└────┬────┘        └──────┬──────┘      └──────┬──────┘
     │                    │                    │
┌────▼────────────────────▼────────────────────▼──────────┐
│                 File System Layer                         │
│  ~/.claude/projects/  ~/.codex/sessions/  state.vscdb   │
│  ~/.gemini/tmp/       fsnotify watchers                 │
└─────────────────────────────────────────────────────────┘
```

### Layering

```
┌────────────────────────┐
│   cmd/        CLI/TUI  │  Entrypoints
├────────────────────────┤
│   internal/ui/  TUI    │  Bubble Tea models, views, Lip Gloss
├────────────────────────┤
│   internal/engine/     │  Orchestrator, state store, event bus
├────────────────────────┤
│   internal/provider/   │  Provider interfaces + built-in impls
├────────────────────────┤
│   internal/watcher/    │  fsnotify wrapper, polling fallback
├────────────────────────┤
│   internal/export/     │  Portable format + cross-agent injection
└────────────────────────┘
```

---

## Event System

All state changes flow through a typed event bus. The TUI subscribes via Bubble Tea's `tea.Msg` interface.

```go
type EventType int

const (
    SessionDiscovered EventType = iota
    SessionUpdated               // File changed, re-read
    SessionEnded
    FileRead
    TokenChanged
    ProviderError
    ExportRequested
    InjectRequested
)

type Event struct {
    Type      EventType
    Provider  string
    SessionID string
    Timestamp time.Time
    Data      interface{}  // typed payload per EventType
}
```

### Flow

```
Agent writes file ──→ fsnotify event
                         │
                    Watcher goroutine
                      (debounce: 500ms)
                         │
                    Read session data via provider
                         │
                    Emit Event{SessionUpdated, ...}
                         │
                    ┌────┴────┐
                    │         │
                EventBus  StateStore (updated)
                    │
               Model.Update()
                    │
               View renders
```

### State Store

In-memory, refreshed from disk on each update. No persistent database.

```go
type StateStore struct {
    mu        sync.RWMutex
    sessions  map[string]*SessionSnapshot  // sessionID → latest state
    providers map[string]ProviderStatus    // provider name → status
}

type SessionSnapshot struct {
    ID         string
    Provider   string
    StartedAt  time.Time
    LastActive time.Time
    Status     SessionStatus  // Active, Idle, Ended
    Files      int
    Tokens     int
    Decisions  int
}
```

---

## Provider SDK (Detailed)

### Core Interface

```go
// Provider is the interface each agent implementor must satisfy.
type Provider interface {
    // Meta
    Name() string                    // e.g. "claude-code"
    Version() string                 // provider version

    // Lifecycle
    Detect(ctx context.Context) bool // Is this agent installed?
    Init(ctx context.Context) error  // One-time setup

    // Session Discovery
    ListSessions(ctx context.Context) ([]SessionInfo, error)

    // Context Reading
    ReadContext(ctx context.Context, sessionID string) (*SessionContext, error)

    // Cross-agent transport
    Export(ctx context.Context, sessionID string) ([]byte, error)
    Inject(ctx context.Context, data []byte) error
}

type SessionInfo struct {
    ID        string        `json:"id"`
    StartedAt time.Time     `json:"started_at"`
    Status    SessionStatus `json:"status"`
    Label     string        `json:"label,omitempty"`
}

type SessionContext struct {
    Session   SessionInfo          `json:"session"`
    Files     []FileEntry          `json:"files"`
    Decisions []string             `json:"decisions"`
    Tokens    int                  `json:"tokens"`
    Metadata  map[string]any       `json:"metadata,omitempty"`
}

type FileEntry struct {
    Path    string `json:"path"`
    Content string `json:"content,omitempty"`
    Size    int    `json:"size"`
}

type SessionStatus string
const (
    StatusActive  SessionStatus = "active"
    StatusIdle    SessionStatus = "idle"
    StatusEnded   SessionStatus = "ended"
)
```

### Built-in Providers

| Provider | Data Source | Parse Strategy | Monitoring |
|----------|------------|----------------|------------|
| `claude-code` | `~/.claude/projects/<hash>/<session>.jsonl` | Stream JSONL, extract last N messages, count tokens, track files read | Watch `.jsonl` files via fsnotify |
| `codex` | `~/.codex/session_index.jsonl` + `~/.codex/sessions/<id>/` | Parse index for session list, then individual session files | Watch session dirs |
| `cursor` | `state.vscdb` (SQLite) | Query `cursorDiskKV` table for `composerData:`, `bubbleId:`, `checkpointId:`, `messageRequestContext:` keys | Poll SQLite (fsnotify on `.vscdb` unreliable — SQLite uses temp files) |
| `gemini-cli` | `~/.gemini/tmp/<hash>/chats/` | Read chat files, extract messages and token usage | Watch `chats/` directory |

### Provider Lifecycle

```
Startup:
  1. Engine.Init() scans for all registered providers
  2. For each provider: provider.Detect()
  3. If detected: provider.Init(), register in provider map
  4. Provider.ListSessions() → seed state store
  5. Start watchers for detected providers

Runtime:
  6. Watcher detects file change → Provider.ReadContext()
  7. State store updated → Event emitted → TUI re-renders

Shutdown:
  8. Producers close → EventBus drains → TUI exits cleanly
```

### External Provider Discovery

For v1, providers are compiled-in. For v1.x, add config-file-based custom providers:

```yaml
# ~/.ctx/providers/my-agent.yaml
name: my-ollama-agent
version: 1
type: config

monitor:
  paths:
    - "~/.my-agent/logs/*.log"
  parse:
    format: jsonl
    session_pattern: "session_id: ([a-z0-9]+)"
    token_pattern: "tokens: (\\d+)"
    file_pattern: "file: (.+)"

export:
  format: json
  output: "~/.my-agent/export.json"

inject:
  method: clipboard  # or "file" with path, or "command"
  command: "my-agent import-context"
```

### Provider Registration Pattern

```go
// Internal registry (not exported — providers are compiled in for v1)
var registry []Provider

func Register(p Provider) {
    registry = append(registry, p)
}

// At package init time in each provider file:
func init() {
    Register(&ClaudeCodeProvider{})
}
```

---

## File Watcher Strategy

### Design Decision: Hybrid (fsnotify + Polling Fallback)

**Primary: fsnotify** for low-latency updates on local filesystems.
**Fallback: Polling** for environments where fsnotify doesn't work (NFS, Docker mounts, WSL cross-fs).

| Backend | OS | Used For |
|---------|----|----------|
| `inotify` | Linux | Primary |
| `kqueue` | macOS, BSD | Primary (watch limit aware) |
| `ReadDirectoryChangesW` | Windows | Primary |
| Poll (1s ticker) | All | Fallback only |

### Implementation

```go
type SessionWatcher struct {
    provider  Provider
    basePaths []string
    fsnotify  *fsnotify.Watcher
    poll      *time.Ticker
    debounce  *time.Timer
    events    chan Event    // outbound to EventBus
    done      chan struct{}
}

func (w *SessionWatcher) Start(ctx context.Context) error {
    // 1. Walk all basePaths to discover session dirs
    // 2. Add each dir to fsnotify watcher
    // 3. Start polling fallback goroutine (if enabled)
    // 4. On fsnotify event: start debounce timer
    // 5. On debounce fire: re-read session data via provider
    // 6. Emit Event to channel
}
```

### Key Decisions

1. **Watch directories, not files** — editors use atomic writes (write temp → rename). A dir-level watch catches the Create/Write on the final file.

2. **Recursive watching** — fsnotify doesn't recurse. On `Init()`, walk the entire agent data dir tree and add each directory. On new `Create` events for directories, add them too.

3. **Debouncing** — Agent logs update rapidly (every message, sometimes every token). A 500ms debounce window groups bursts into single state snapshots. Counter-example: debounce too high and the TUI feels sluggish; 500ms is a good baseline.

4. **Cursor special case** — SQLite `state.vscdb` creates temp files during writes (`state.vscdb-wal`, `state.vscdb-shm`). fsnotify works poorly here. **Poll every 2s** instead, read DB on change.

5. **Watch limit awareness** — On macOS (kqueue), track number of watched fds. If approaching system limit (`kern.maxfilesperproc`), fall back to polling for that provider. Warn user.

### Try: `LarsArtmann/go-filewatcher`

This higher-level wrapper does recursive watching, debouncing, and filtering out of the box. Worth trying in prototyping — if it introduces complexity or bugs, drop down to raw fsnotify.

---

## Token Counting Approach

**No API calls, no models — pure estimation.**

### Strategy

```go
func EstimateTokens(text string) int {
    // Simple heuristic: ~4 characters per token for code
    // More accurate than 1:1 char/token for code-heavy contexts
    return len([]rune(text)) / 4
}
```

For v1, a single heuristic works well enough. The exact token count doesn't matter — what matters is the **relative fullness** of the context window so users can see when they're approaching limits.

### Display

- `tokens: 87K / 100K` — estimated / context window limit
- Color coding: green (<60%), yellow (60-85%), red (>85%)
- Per-provider context window sizes baked into the provider:
  - Claude Code: ~100K tokens
  - Codex: ~200K tokens (GPT-4o)
  - Cursor: depends on underlying model
  - Gemini CLI: ~1M tokens

### v2 Improvement

Swap in a language-aware tokenizer (e.g., `github.com/pkoukk/tiktoken-go` or a simple BPE) for better accuracy. But this adds a dependency and complexity — defer until the heuristic proves inadequate.

---

## Export/Inject Format (Portable Context)

### Specification v1

```json
{
  "format_version": "1",
  "ctx_version": "0.1.0",

  "source": {
    "provider": "claude-code",
    "project": "my-project",
    "exported_at": "2026-06-20T12:00:00Z"
  },

  "session": {
    "id": "sess_abc123",
    "duration_seconds": 2700,
    "tokens_used": 87000
  },

  "context": {
    "summary": "Working on user authentication module",

    "files": [
      {
        "path": "src/auth/login.go",
        "content": "package auth\n\nfunc Login(...",
        "size": 2048,
        "mime": "text/x-go"
      }
    ],

    "decisions": [
      "Using bcrypt for password hashing",
      "JWT tokens with 24h expiry stored in httpOnly cookies"
    ],

    "environment": {
      "go_version": "1.23.0",
      "os": "darwin",
      "arch": "arm64"
    }
  }
}
```

### Injection Strategies per Agent

| Agent | Injection Method | Notes |
|-------|-----------------|-------|
| Claude Code | Append to session `.jsonl` via custom tool message | Works if session is active; else start new session with context |
| Codex | Write to `~/.codex/sessions/<id>/` in expected format | Inject as system message or conversation primer |
| Cursor | Write via `cursorDiskKV` update to SQLite | Tricky — Cursor may overwrite on restart; document as "best effort" |
| Gemini CLI | Prepend to chat file in expected format | Similar to Claude Code approach |
| Fallback | Write to clipboard (`internal/clipboard`) | User manually pastes into agent of choice |
| Fallback | Write to `ctx-context.md` in project root | Agent can read it if configured in CLAUDE.md |

### Export/Inject CLI Flow

```
ctx export claude-code          → prints portable JSON to stdout
ctx export claude-code -o ctx.json  → writes to file
ctx inject codex ctx.json       → reads file, injects into Codex via provider
ctx inject codex -              → reads from stdin (piped)
```

---

## Data Flow (End-to-End)

```
Time ────────────────────────────────────────────────────────────────►

Agent Activity               ctx TUI
─────────────────           ────────
Claude Code writes
to .jsonl file
       │
       ▼
fsnotify event
(WRITE on /sessions/*.jsonl)
       │
       ▼
Debounce (500ms)
       │
       ▼
ClaudeCodeProvider
.ReadContext()
  → Parse last N lines of JSONL
  → Extract files, decisions, tokens
       │
       ▼
StateStore.Update(sessionID, snapshot)
       │
       ▼
EventBus <- Event{SessionUpdated, ...}
       │
       ▼
Model.Update() receives tea.Msg
  → Updates model state
       │
       ▼
Model.View() re-renders
  → Panel shows updated metrics
       │
       ▼
User sees live update
```

---

## Implementation Phases

### Phase 1: Project Skeleton + Claude Code Provider
- `go mod init`, directory structure (`cmd/`, `internal/`)
- Cobra CLI scaffolding (`ctx`, `ctx status`, `ctx export`, `ctx inject`)
- Provider interface + Claude Code provider (JSONL parser)
- fsnotify file watcher for `~/.claude/projects/`
- `ctx status` CLI command — quick non-TUI session listing
- Token estimation heuristic

### Phase 2: Remaining Providers
- Codex provider (session index + per-session JSONL)
- Cursor provider (SQLite reader with polling)
- Gemini CLI provider (chat file parser)
- Cross-provider session listing in `ctx status`

### Phase 3: TUI
- Bubble Tea model + view for 4-panel dashboard
- Lip Gloss styling (color, layout, borders)
- Keyboard navigation (tab between panels, select session, etc.)
- Live updates from event bus
- Status indicators (● live, ○ idle, ◌ ended)
- Color-coded token usage bars

### Phase 4: Export/Inject
- Portable context format serialization/deserialization
- `ctx export <agent>` command
- `ctx inject <agent> <file>` command
- Per-agent injection strategies
- Clipboard fallback

### Phase 5: Custom Provider System
- Config-file-based custom providers (YAML)
- Provider validation
- `ctx provider add <path>` command
- `ctx providers` list with status

### Phase 6: Polish & Cross-Platform
- Windows terminal compatibility testing
- macOS kqueue watch limit handling
- Linux inotify max_user_watches warnings
- Error state display in TUI
- Help system (keybindings)
- Performance benchmarking with real agent workloads
