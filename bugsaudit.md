# 🛑 BRUTAL CODE REVIEW: `ctx` (minimo)

> **Audit date:** 2026-06-24
> **Files:** 31 `.go` files | **Size:** ~56 KB | **Tests:** 0
> **Branch:** `feat/cost-ui-cli-docker`

---

## 🔴 Executive Summary

56 KB. 31 Go source files. **Zero tests.**

A terminal dashboard for monitoring AI coding agent sessions — with no automated verification of any kind. The architecture draft (`docs/wiki/`) is ambitious and well-researched. The implementation is a rushed prototype that doesn't live up to it.

The codebase works on a happy path and breaks silently on everything else.

---

## 🔴 BLOCKER: Zero Tests (31 Files, 0 `_test.go`)

Not a single test exists anywhere in the entire project.

| Package | Files | Lines | Testable logic | Tests |
|---|---|---|---|---|
| `internal/format` | 1 | 60 | String formatting, number rounding, truncation | **0** |
| `internal/tailreader` | 1 | 49 | File offset tracking, partial-line safety | **0** |
| `internal/watcher` | 1 | 91 | Debounce timers, event coalescing | **0** |
| `internal/export` | 1 | 86 | Path containment, case-insensitive comparison | **0** |
| `provider/claudecode` | 5 | 432 | JSONL parsing, token aggregation, file tracking | **0** |
| `provider/codex` | 3 | 285 | Rollout JSON parsing, timestamp recovery | **0** |
| `provider/kimicode` | 3 | 274 | Wire JSON with optional payload nesting | **0** |
| `provider/opencode` | 2 | 206 | SQL queries, epoch conversion, JSON model parse | **0** |
| `provider/configprovider` | 2 | 210 | Regex compilation, glob matching, file parsing | **0** |
| `internal/ui` | 5 | 259 | Table rendering, context bars, formatting | **0** |
| `internal/engine` | 2 | 115 | State store, provider orchestration | **0** |
| `cmd/ctx` | 1 | 227 | CLI dispatch, watch loop, table output | **0** |

**Specific functions that are untested and almost certainly have untriggered bugs:**

- `format.FormatCount` — has a 999,500→"1.0M" rounding promotion edge case. Untested.
- `tailreader.Cursor.ReadNew` — truncation recovery (file shrunk → reset offset). Untested.
- `export.withinDir` — case-insensitive path containment with cross-drive scenarios. Untested.
- `claudecode/state.go` — `contextTokens` uses `InputTokens + CacheCreationInputTokens + CacheReadInputTokens` but `applyNew` overwrites on every `assistant` line. Last line wins, not sum. Untested, may be wrong semantics.
- `watcher.handle` — debounce timers map leak (timer fires → sends to channel, but entry stays in map forever). Untested.

---

## 🔴 Architecture: Global Mutable Registry with `init()` Hell

```go
// provider/registry.go
var registry []Provider
var pathOverrides = map[string]string{}

func Register(p Provider) { registry = append(registry, p) }
```

- 4 `init()` functions in `claudecode`, `codex`, `kimicode`, `opencode` register into the global slice.
- Go's `init()` ordering between packages in the same binary is deterministic (lexical by file path) but **not documented as a language contract**. If the order ever changes, the provider list order is non-deterministic.
- **No way to construct providers in isolation** — any test importing these packages triggers all 4 `init()` calls as side effects.
- `pathOverrides` global exists only because `init()` runs before `main()`, so config-loaded paths can't be passed as constructor arguments. This is a workaround for a problem the architecture itself created.

**Fix:** Kill `init()` registration. Pass provider instances explicitly to `Engine`.

---

## 🔴 Systematic Silent Error Swallowing

The dominant error-handling pattern is: "if anything fails, continue silently."

```go
// engine/engine.go:38-54
for _, p := range e.providers {
    if !p.Detect() { continue }
    sessions, err := p.ListSessions()
    if err != nil { continue }        // ◄ entire provider disappears silently
    for _, s := range sessions {
        ctx, err := p.ReadContext(s.ID)
        if err != nil { continue }    // ◄ session disappears silently
        e.Store.Put(s.ID, *ctx)
    }
}
```

**Same pattern everywhere:**

| Location | Silenced error | Impact |
|---|---|---|
| `watcher.go:70` | `_ = w.fsw.Add(ev.Name)` | New directory not watched |
| `claudecode/provider.go:76` | JSON unmarshal error on live registry | PID-based status detection silently broken |
| `claudecode/provider.go:114` | ReadDir on project dir | Session list silently incomplete |
| `codex/provider.go:68` | WalkDir error (`return nil`) | Entire rollout search tree silently truncated |
| `configprovider/spec.go:42` | YAML parse error | Provider config silently dropped |
| `kimicode/provider.go:78` | ReadDir on session dir | Session silently dropped |
| `opencode/queries.go:43` | Row scan error | All remaining sessions silently dropped |
| `main.go:73` | `e.Refresh()` error ignored | TUI silently shows stale data |

A user whose OpenCode DB has a corrupt row will see an empty session list with no error indicator. A user whose `~/.codex/sessions/` has a permission issue will see Codex absent from the dashboard with no explanation.

---

## 🔴 Duplicated Code Across Providers

**`idleThreshold = 30 * time.Second`** — defined identically in 4 files:

| File | Line |
|---|---|
| `internal/provider/claudecode/provider.go` | 16 |
| `internal/provider/codex/provider.go` | 16 |
| `internal/provider/kimicode/provider.go` | 14 |
| `internal/provider/configprovider/provider.go` | 14 |

Changing the threshold requires touching 4 files. If they ever drift, behaviour becomes inconsistent between providers.

**`parseTimestamp`** — identical function (RFC3339 parse, return `(time.Time, bool)`) in:
- `internal/provider/claudecode/jsonl.go:47-53`
- `internal/provider/codex/rollout.go:59-65`

**Provider struct pattern** — three providers (`ClaudeCodeProvider`, `CodexProvider`, `KimiCodeProvider`) share:

```go
type XProvider struct {
    mu       sync.Mutex
    sessions map[string]*sessionState
}
```

Same mutex, same lazy-init, same lock/unlock around `ListSessions`/`ReadContext`. None of this is extracted into a shared type.

---

## 🔴 `configprovider`: Reads Every File Twice Per Refresh

`ListSessions` calls `matchedFiles()` which globs every pattern. `ReadContext` calls `matchedFiles()` again via `sessionInfo()`. For a user with 3 glob patterns matching 20 files each, that's 120 `filepath.Glob` calls plus 60 `os.ReadFile` calls per refresh.

The code acknowledges this in a comment (`provider.go:87-89`) and then... doesn't fix it.

---

## 🔴 `os.Exit` Galore (10 Calls in main.go)

| Line | Context | Problem |
|---|---|---|
| 33 | Config load fail | `os.Exit(1)` — no defer runs |
| 53 | Unknown subcommand | Hard exit |
| 61 | Engine refresh fail | `defer stop()` from `signal.NotifyContext` skipped |
| 86 | TUI program error | Exit without cleanup |
| 103 | Status refresh fail | Exit without cleanup |
| 115 | Export missing session ID | Usage message then exit |
| 128 | Unknown session | Exit after store lookup failure |
| 134 | Export JSON marshaling | Exit mid-export |
| 141 | Export output | Hard exit |
| 160 | WatchLoop error | **defer `stop()` skipped** — signal handler context leaks |

Line 160 is the worst: `runWatch` defers `stop()` on the signal-notify context, but `os.Exit(1)` at line 160 (inside `runWatch`'s error check of `watchLoop`) causes immediate process termination without running deferred functions. The cancellation context for the signal handler is abandoned.

---

## 🔴 `contextLimitFor` — Hardcoded and Stale

```go
var contextWindowSizes = map[string]int{
    "claude-sonnet-4-6": 1_000_000,
    "claude-opus-4-8":   1_000_000,
}
```

Two models. Every other model (Claude Sonnet 4.5, Claude Opus 4, future models) gets `0` — which means `ContextUsage.Known` is true (because it's set by `state.model != ""`), but `Limit` is 0, so the UI renders a raw token count with **no percentage bar**. The user sees *less* information with a newer model than if the default were wrong but present.

The conservative approach ("wrong denominator is worse than no percentage") is defensible, but the result is that most Claude 4.5+ users get degraded display.

---

## 🔴 UI and Display Issues

### Hardcoded Unicode in Context Bar

`bar.go:51`:
```go
bar := style.Render(strings.Repeat("█", filled)) + barEmptyStyle.Render(strings.Repeat("░", barWidth-filled))
```

Renders fine on modern terminals. **Fails on:**
- Windows cmd.exe pre-Windows 10
- Terminals without Unicode support (e.g., some CI output, serial consoles)
- Certain font configurations
- Screen readers and accessibility tools

Should fall back to ASCII (`"#"` / `"-"` or similar) when terminal capability is unknown.

### Hardcoded Column Widths

`rows.go:51-62` defines columns with fixed widths (MODEL: 18, CWD: 24, CONTEXT: 32, LABEL: 30). Values exceeding these widths are silently truncated with `"..."`. The user has no way to view the full value.

### Bubble Tea Model Uses Value Receiver

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { ... }
```

Bubble Tea conventions universally show pointer receivers for models. Value receivers *work* because the modified copy is returned as `tea.Model`, but this is unusual and signals uncertainty about the framework's lifecycle. It also means every `Update()` call copies the entire model struct (including the `table.Model` inside it), which has performance implications as the model grows.

---

## 🔴 OpenCode Provider: SQLite Connection Leak

```go
func (p *OpenCodeProvider) open() (*sql.DB, error) {
    if p.db != nil { return p.db, nil }
    db, err := sql.Open("sqlite", dsn)
    p.db = db
    return db, nil
}
```

The `*sql.DB` is cached in the struct but never closed. There is no `Close()` method on `OpenCodeProvider`. For a read-only connection in a short-lived CLI process this is harmless, but if the engine is re-initialized (future feature), connections leak.

---

## ⚠️ MODERATE Issues

### `watcher.Events` Buffer Size

`make(chan string, 64)` — undocumented capacity. If the consumer is blocked on a slow refresh, 64 more events pile up before the watcher blocks. With debounce timers collapsing rapid events, this is unlikely to be hit, but the number is arbitrary.

### `watcher.handle` Timer Map Leak

```go
func (w *Watcher) handle(ev fsnotify.Event) {
    // ...
    w.timers[path] = time.AfterFunc(w.debounce, func() {
        w.Events <- path
    })
}
```

When the timer fires, the entry remains in `w.timers` forever. Over hours of monitoring, this map grows unbounded. The timer ID is never cleaned up after firing.

### Kimi Code: `findWireLogs` Walks Dir Tree Every Poll

`findWireLogs()` does `os.ReadDir` twice per session directory — once for `workDirKeys`, once for `sessionDirs` — every single `ListSessions` call. No caching, even though the directory structure is stable during process lifetime. For users with hundreds of archived sessions, this is O(n) I/O per poll interval.

### `format.FormatCount` — Rounding Edge Case

```go
if float64(n)/1_000 >= 999.5 {
    return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
}
```

At n=999,500, this correctly promotes to "1.0M". But `float64(n)/1_000` at this boundary loses precision for values near `float64`'s 53-bit mantissa limit. At n=999,501, the float conversion itself introduces error. Untested, unverified.

### README Outdated

The README shows `--watch` flag for `status` and `--with-content` for `export`, but this is undocumented in the help output. The `ctx` binary has no `--help` flag support — running `ctx --help` hits the default branch and launches the TUI.

---

## ✅ What's Actually Good (For Balance)

1. **Clean package boundaries** — `internal/` packages are well-separated with clear responsibilities. The layering in `draft.md` is respected in code.

2. **`tailreader.Cursor`** — Offset-based incremental reading with partial-line protection is exactly right for multi-hundred-MB JSONL files. This is the cleanest piece of code in the project.

3. **Conservative `go.mod`** — Only 6 direct dependencies, all well-chosen. No unnecessary bloat.

4. **`go vet` passes** — Basic Go hygiene is maintained.

5. **`ContextUsage.Known` semantics** — The explicit `Known` guard for token/cost data prevents rendering meaningless zero values. Thoughtful API design.

6. **`export.withinDir` privacy guard** — Refusing to export file contents outside the session's CWD is a genuine security consideration most tools don't make.

7. **`configprovider` YAML provider system** — Extensible by design. A genuine differentiator for custom/niche coding agents.

8. **Comments provide rationale, not narration** — The code comments explain *why* (e.g., case-insensitive path comparison for Windows), not *what*. Rare and valuable.

9. **Debounced watcher** — Correct design for bursty file writes. The per-file debounce timer (as opposed to a single global debounce) is the right granularity.

10. **No framework over-abstraction** — Despite only needing 15 lines of Bubble Tea boilerplate, the project doesn't wrap Bubble Tea in another abstraction layer. Good restraint.

---

## 📊 Raw Metrics

| Metric | Value |
|---|---|
| Go source files | **31** |
| Test files | **0** |
| Test coverage | **0%** |
| `go vet` passes | ✅ Clean |
| `os.Exit(1)` calls | **10** |
| `init()` functions (with side effects) | **4** |
| Global mutable variables | **2** (`registry`, `pathOverrides`) |
| `continue` on error patterns | **10+** |
| `idleThreshold` copies | **4** (identical, 4 files) |
| Models in `contextWindowSizes` | **2** (of ~15+ real Claude models) |
| `time.Duration` → millisecond conversions | **2** (`config.go:49`, `config.go:53`) — both manual, no `time.Millisecond` constant used |
| `filepath.Join` paths constructed | **12+** — none use `os.PathListSeparator` or account for relative paths |

---

## 🎯 Prescription (Ordered by Impact)

1. **Add tests** — Start with `internal/format`, `internal/tailreader`, and `internal/export`. These are pure functions with zero dependencies. Table-driven tests on these would catch regressions immediately and cost almost nothing to write.

2. **Surface errors to the TUI** — At minimum, collect per-provider error strings and render them in the header. Silent failures are worse than no feature — the user can't debug what they can't see.

3. **Extract shared provider plumbing** — Lift `sync.Mutex + map[string]*sessionState` into a `trackedProvider` base. Deduplicate `idleThreshold` and `parseTimestamp`.

4. **Kill `init()` registration** — Replace global `registry` with explicit construction in `main()`. Pass `Engine` its providers as a slice. This is the single change that makes the entire codebase testable.

5. **Update `contextWindowSizes`** — Add current Claude models (4.5 Sonnet, Opus 4, etc.). Consider a `CTX_MODEL_LIMITS` env var or config override for power users.

6. **Fix the watcher timer leak** — Remove map entries after the timer fires or on `Close()`. Over an 8-hour session this could accumulate thousands of stale entries.

7. **Clean up `os.Exit` callers** — Return errors to `main()` and let defers run. At minimum fix the `runWatch` exit that skips `stop()`.

8. **Add ASCII fallback for context bars** — Detect terminal capabilities or add a config flag for block vs ASCII characters.

9. **Cache configprovider's glob results** — Don't read every monitored file twice per 2-second poll interval. Structure the provider so `ReadContext` can reuse data from `ListSessions`.

10. **Add `Close()` to `OpenCodeProvider`** — Close the SQLite connection on engine shutdown, even if the process exits immediately after.
