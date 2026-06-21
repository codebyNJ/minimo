# Architecture

This refines `draft.md`'s design after grounding it in real session files (see [Research](Research.md)). It keeps the original's shape — Provider interface, watcher pool, in-memory state store, Bubble Tea TUI — and fixes the parts that were guesses.

## What draft.md got right (kept as-is)

- Provider interface shape (`Detect`, `ListSessions`, `ReadContext`, `Export`, `Inject`)
- Layered package structure (`cmd/`, `internal/ui`, `internal/engine`, `internal/provider`, `internal/watcher`, `internal/export`)
- Typed event bus feeding Bubble Tea via `tea.Msg`
- Debounced fsnotify with polling fallback for SQLite-backed providers
- Zero-AI, local-first, single-binary philosophy

## Gaps found and how they're closed

### 1. Session status must come from the live-PID registry, not file mtimes

draft.md inferred `Active`/`Idle`/`Ended` purely from when a session's file last changed. That's wrong in a specific, common way: an agent can sit silent for tens of seconds while "thinking" — a quiet file doesn't mean the session ended.

Claude Code already publishes the real signal at `~/.claude/sessions/<pid>.json` (see [Research](Research.md)). Fixed design:

```go
type LiveCheck func(pid int) bool // os.FindProcess + signal-0 probe; tasklist-based on Windows

func (p *ClaudeCodeProvider) Status(s SessionInfo) SessionStatus {
    if pidFile, ok := p.liveRegistry[s.ID]; ok && p.isAlive(pidFile.PID) {
        return StatusActive
    }
    if time.Since(s.LastActive) < idleThreshold {
        return StatusIdle
    }
    return StatusEnded
}
```

Codex and Gemini CLI don't have a confirmed equivalent registry yet (unverified — no local install to test against). For those two, fall back to mtime-based inference, documented as a known accuracy gap rather than silently assumed correct.

### 2. Streaming tail reader (the optimization layer draft.md was missing) {#streaming-tail-reader}

draft.md's `ReadContext()` implies re-parsing a session file on every fsnotify event. Two real facts make that wrong:

- A live Claude Code session on this machine is already 41MB.
- Codex's own GitHub issues report rollout files growing to 700MB–2GB ([source](Research.md#openai-codex-cli--verified-via-web-search-2026-06-21)).

Re-parsing megabytes of JSON on every keystroke-adjacent event would make `ctx` itself the slow thing in the room. Fix: track a byte offset per session and only read what's new.

```go
type TailCursor struct {
    Path      string
    Offset    int64 // bytes already consumed
    LastStat  os.FileInfo
}

func (c *TailCursor) ReadNew() ([]byte, error) {
    f, err := os.Open(c.Path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    info, err := f.Stat()
    if err != nil {
        return nil, err
    }
    if info.Size() < c.Offset {
        c.Offset = 0 // file was truncated/rewritten — start over
    }

    if _, err := f.Seek(c.Offset, io.SeekStart); err != nil {
        return nil, err
    }
    buf, err := io.ReadAll(f)
    if err != nil {
        return nil, err
    }
    c.Offset += int64(len(buf))
    return buf, nil
}
```

The provider keeps one `TailCursor` per session ID in memory, parses only the newly returned bytes as JSONL, and folds the result into the existing `SessionSnapshot` instead of rebuilding it from scratch. This is the single most important addition not in draft.md — without it, the tool doesn't scale to real session sizes.

### 3. Recursive watch without an unverifiable dependency {#recursive-watch}

draft.md named `LarsArtmann/go-filewatcher` as a candidate wrapper. It doesn't turn up in a search — don't depend on something we can't verify exists or is maintained. `fsnotify` itself still has no public recursive-watch API in 2026. The fix is to own the ~80 lines ourselves:

```go
func addRecursive(w *fsnotify.Watcher, root string) error {
    return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
        if err != nil || !d.IsDir() {
            return err
        }
        return w.Add(path)
    })
}

// In the event loop, on fsnotify.Create for a directory, call w.Add(newPath)
// so newly created session subdirectories are picked up without a restart.
```

This is less code than vendoring and learning an unverified wrapper library, and it's ours to debug.

### 4. `Decisions` field dropped from v1

draft.md's `Context.Decisions []string` implied summarizing "what was decided" from raw logs — that requires an AI to read and judge intent, which contradicts the "zero AI" pillar of this whole project. **Decision: removed from `SessionContext` for v1.** See [Decisions](Decisions.md) for the record of this call. If a future non-AI heuristic earns its way back in (e.g. git commit messages during the session window), it gets re-added with a named source, not as a vague freeform list.

### 5. Exact token counts where available, heuristic only where not

draft.md proposed one universal `EstimateTokens(text) int` heuristic. Claude Code already gives exact numbers per turn (see [Research](Research.md)), so:

```go
type TokenSource int
const (
    TokenSourceExact TokenSource = iota // summed from provider-native usage data
    TokenSourceEstimated                 // chars/4 fallback
)

type TokenUsage struct {
    Total  int
    Source TokenSource
}
```

The TUI should visually distinguish exact vs. estimated (e.g. a `~` prefix on estimated numbers) so users aren't misled into trusting a guess as precisely as a real count.

### 6. File-change detection must handle truncation/rewrite, not just append

SQLite-backed providers (Cursor) rewrite their backing file rather than append. The `TailCursor.ReadNew()` above already resets to offset 0 when `info.Size() < c.Offset` — call this out explicitly because it's an easy bug to reintroduce if Phase 2's Cursor provider is built without remembering this constraint.

### 7. Minimal config, not zero config

draft.md had no settings story at all. v1 needs just enough to avoid hardcoding values that are obviously per-user preferences, without building a config system prematurely:

- Debounce window (default 500ms)
- Idle threshold (default: no event for 30s while no live PID)
- Which providers are enabled (default: all detected)

For Phase 1, these are constants in `internal/engine/config.go` with documented defaults — no YAML file yet. A file-based config is Phase 5's problem (it already needs one for custom providers); don't build two config systems.

### 8. Privacy: exported file content is opt-in, not automatic

We observed `.credentials.json` sitting directly next to session data on this machine. `SessionContext.Files[].Content` must default to empty/omitted on export; full file content is only included if the user explicitly passes `--with-content` to `ctx export`. This isn't optional hardening — it's the difference between a monitoring tool and an accidental credential leak.

### 9. Graceful shutdown

`main()` must wire `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` and pass that context down to every watcher and the Bubble Tea program, so `Ctrl+C` exits cleanly instead of leaving fsnotify goroutines or open file handles behind.

### 10. CLI argument parsing: stdlib over Cobra for v1

draft.md implied Cobra. For four flat subcommands (`ctx`, `status`, `export`, `inject`) with no nested flags or shell-completion requirement yet, the stdlib `flag` package plus a manual `switch os.Args[1]` dispatch is fewer lines and one fewer dependency — directly serving the "minimal code, zero unnecessary deps" goal. Revisit Cobra only if subcommands grow nested flags or need completion scripts (not needed before Phase 5 at the earliest).

## Folder Structure (Phase 1 scope, full project shape shown for context)

```
ctx/
├── go.mod
├── go.sum
├── LICENSE
├── README.md
├── cmd/
│   └── ctx/
│       └── main.go              # entrypoint: parses os.Args, wires context, calls into engine
├── internal/
│   ├── provider/
│   │   ├── provider.go          # Provider interface + shared types (SessionInfo, SessionContext, ...)
│   │   ├── registry.go          # Register()/All() — compiled-in provider list
│   │   └── claudecode/
│   │       ├── provider.go      # ClaudeCodeProvider: Detect, ListSessions, ReadContext
│   │       ├── jsonl.go         # JSONL line parsing into typed events
│   │       └── tokens.go        # sums usage.* fields into TokenUsage{Source: Exact}
│   ├── watcher/
│   │   └── watcher.go           # recursive fsnotify wrapper + debounce (see Architecture#recursive-watch)
│   └── engine/
│       ├── engine.go            # orchestrator: owns provider registry + store, drives Refresh()
│       ├── store.go             # in-memory StateStore (sessionID -> SessionContext)
│       └── config.go            # debounce default (see Architecture#7)
├── docs/
│   ├── wiki/                    # this wiki
│   └── superpowers/plans/       # implementation plans
└── .gitignore
```

`internal/tokenestimate/` (the chars/4 fallback heuristic) is **not** part of Phase 1 — the only Phase 1 provider (Claude Code) always has exact token counts, so the heuristic would have zero callers. It gets added in Phase 2 alongside whichever provider first lacks exact usage data. Building it now would be dead code.

Phase 2+ adds `internal/provider/codex/`, `internal/provider/cursor/`, `internal/provider/geminicli/`, `internal/tokenestimate/`, `internal/ui/`, `internal/export/` following the same per-provider-own-package pattern — not built now, listed so Phase 1's structure doesn't have to be reshaped later.

## Why no `internal/export/` or `internal/ui/` yet

Phase 1's deliverable is `ctx status` — a flat CLI listing of detected sessions, proving the provider + watcher + store loop works end-to-end without the added surface area of a TUI or export format. Building those before one provider is proven against real data would be designing against guesses. See [Roadmap](Roadmap.md).
