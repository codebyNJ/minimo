# Context-Fullness & Cost Metrics — Design

**Date:** 2026-06-22
**Status:** Approved (presented in chat, approved by proceeding directly to implementation plan)

## Why

A reviewer of [github.com/kenn-io/agentsview](https://github.com/kenn-io/agentsview) (a 40-agent analytics tool) surfaced two things `ctx` was getting wrong on the way to being "btop for AI agents":

1. **We were summing the wrong number.** `TokenUsage.Total` sums every turn's `cache_read_input_tokens`, which inflates into the tens/hundreds of millions on long sessions (78M+ observed on this project's own session). That sum is real and useful as a *lifetime/cost-driver total* — it's what you'd be billed for, conceptually — but it is NOT "how full is the context window right now," which is what a btop-style fullness bar actually needs.
2. **Cost was sitting unused.** OpenCode's SQLite `session` table already has an exact `cost` column (confirmed via ground-truth DB inspection earlier this project) — we query it and throw it away.

Ground-truth checks against this project's own live Claude Code session confirmed: `message.model` is present per-turn (sibling to `usage`), and the latest turn's `input_tokens + cache_creation_input_tokens + cache_read_input_tokens` is a real, single-turn "what's in context right now" figure (369,401 observed at the time of checking — well inside Sonnet 4.6/Opus 4.8's documented 1M-token API context window).

## Scope decisions (locked during brainstorming)

| Decision | Answer |
|---|---|
| Deliverable this round | Data/metrics layer only — not the Phase 3 TUI |
| Provider asymmetry (context fullness) | Best-effort per provider: Claude Code gets a real figure; OpenCode/custom providers mark it explicitly unknown rather than approximate |
| Fullness display | Raw token count always shown; a `/limit` percentage-style figure only for models with a **verified** window size — never guess a denominator |
| Cost scope | Exact only (OpenCode's real `cost` column); no price-estimation table for token-only providers |
| Struct approach | Extend existing `SessionInfo`/`SessionContext` types in place (adding fields), not a parallel metrics type or a generic capability map |
| `ctx export` | Frozen — not touched, not extended with the new fields, in this round |
| `ctx status` table | Updated to actually show the new data (CONTEXT, COST, and MODEL columns) — a field with no consumer is dead data, which this project has a standing rule against (see `internal/tokenestimate` precedent in `Architecture.md`) |

## Data model

```go
// provider.go — additions to existing types
type SessionInfo struct {
    ID, Provider, CWD, Label, Model string  // +Model
    Status     SessionStatus
    StartedAt, LastActive time.Time
}

type ContextUsage struct {
    Tokens int   // latest-turn input+cache_creation+cache_read
    Known  bool  // false = provider can't isolate a single-turn figure
    Limit  int   // model's context window; 0 = unknown, never show a %
}

type Cost struct {
    USD   float64
    Known bool
}

type SessionContext struct {
    Session SessionInfo
    Tokens  TokenUsage    // UNCHANGED semantics — lifetime sum, still valid as a cost-driver total
    Files   []FileRef
    Context ContextUsage  // new
    Cost    Cost          // new
}
```

`TokenUsage.Total` is **not** renamed or recomputed — it was correctly summing what it claims to sum. The table column showing it is renamed `LIFETIME` for clarity, since the ambiguous old `TOKENS` label is exactly what caused this confusion in the first place.

## Per-provider behavior

- **Claude Code:** track the latest assistant turn's `message.model` and `input_tokens + cache_creation_input_tokens + cache_read_input_tokens` (replace, not accumulate — same pattern as how `label` already tracks the latest `ai-title`). `Context.Known = true` always. `Limit` from a small hardcoded table, populated only with models actually observed in real session data on this machine (`claude-sonnet-4-6`, `claude-opus-4-8` — both confirmed via grep against a real session file; sourced from Anthropic's published API context-window docs, since Claude Code is an API-driven CLI, not the lower-limit web-chat product). Any other model string → `Limit: 0` → raw number only, no percentage.
- **OpenCode:** add `model` to the existing SQL `SELECT` (`cost` is already selected, just unused). `Cost.Known = true`, `Cost.USD = row.cost`. `Context.Known = false`, explicitly, with a comment — the session table only has lifetime aggregates, no latest-turn figure, and reading the message/part tables to derive one was already ruled out (privacy + scope) earlier this project.
- **configprovider (custom YAML):** no change. Zero-value defaults already give `Context.Known = false`, `Cost.Known = false`, `Model = ""` — correct by construction.

## Display

```
PROVIDER     STATUS   MODEL              LIFETIME   CONTEXT      COST      LAST       CWD                      LABEL
claude-code  active   claude-sonnet-4-6  78893660   369401/1.0M  -         17:20:23   D:\codes\minimo          ...
opencode     ended    deepseek-v4-fla... 16945836   -            $0.0421   19:18:06   D:/Brainstorm            ...
demo         idle     -                  150        -            -        22:52:31   C:\Users\sst\.ctx-demo   alpha
```

## Out of scope (named explicitly)

- `internal/export` — completely untouched.
- Bars/colors/panels — Phase 3 TUI.
- Cost estimation for token-only providers.
- Context-fullness for OpenCode/custom providers.
- Window-size entries for any model not directly observed in real session data on this machine.
