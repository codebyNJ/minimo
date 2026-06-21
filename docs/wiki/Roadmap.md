# Roadmap

Carried from `draft.md`, refined where research changed the plan. Only Phase 1 has a detailed task-by-task implementation plan so far (`docs/superpowers/plans/`); the rest stay at this level of detail until the phase before them ships.

## Phase 1 — Skeleton + Claude Code provider (in progress)

- `go.mod`, package layout per [Architecture](Architecture.md#folder-structure-phase-1-scope-full-project-shape-shown-for-context)
- `Provider` interface + compiled-in registry
- Claude Code provider: JSONL tail reader, exact token summing from `usage`, `ai-title` → session label, live-PID status check
- Recursive fsnotify watcher with debounce
- `ctx status` — flat CLI listing, no TUI yet

## Phase 2 — Remaining providers (scope locked 2026-06-21: the 4 confirmed-usage-data tools)

Scope decision: Phase 2 is the 3 providers where exact usage data is confirmed on real evidence, not just hoped-for from docs — joining Claude Code (Phase 1). See [Decisions](Decisions.md#main-vs-future-scope-provider-set-2026-06-21).

- **OpenCode provider** (first in line): `modernc.org/sqlite` reader against `opencode.db`'s `session` table — `SELECT tokens_input, tokens_output, tokens_reasoning, tokens_cache_read, tokens_cache_write, cost, model, directory, title, time_updated, time_archived FROM session`. No transcript parsing needed; cheapest provider to build and the richest data of any tool researched (see [Research](Research.md#opencode--ground-truth-verified-by-inspecting-a-real-install-on-this-machine-2026-06-21)). 2s polling, not fsnotify, per [Architecture #6](Architecture.md#6-file-change-detection-must-handle-truncationrewrite-not-just-append).
- **Codex provider**: date-bucketed rollout file discovery (`~/.codex/sessions/YYYY/MM/DD/`), corrected from draft.md's assumed single index file. Usage from the latest `token_count` event's `total_token_usage` (see [Research](Research.md#openai-codex-cli--verified-via-web-search-2026-06-21)) — read the latest event, don't sum, due to the rate-limit re-emission gotcha.
- **Kimi Code CLI provider**: per-session `agents/main/wire.jsonl` under `~/.kimi-code/sessions/<workDirKey>/<sessionId>/`. Usage from the latest `StatusUpdate.TokenUsage`.
- Cross-provider session listing in `ctx status`, now across 4 providers instead of 1.

## Future scope — deferred, not actively planned

Came up during 2026-06-21 research but didn't make the confirmed-usage-data cut, so parked here instead of on a phase. Revisit only with a concrete reason to (e.g. the user starts depending on one of these daily, the way OpenCode and Claude Code already are):

- **Gemini CLI** — real session file checked on this machine had no usage data at all, only typed-text logs (see [Research](Research.md#gemini-cli--verified-via-web-search-2026-06-21)). Needs a more active install to even confirm the data exists before it's worth building.
- **Cursor (`cursor-agent` CLI)** — schema not empirically verified, usage-field presence not checked this round.
- **GitHub Copilot CLI** — storage location known, usage-field presence not checked this round.
- **Aider** — structurally doesn't fit the `Provider` interface (no sessions, no daemon, one flat file per repo); would need a design change before it's even buildable, separate from the usage-data question.

## Phase 3 — TUI

- Bubble Tea model + 4-panel dashboard view
- Lip Gloss styling; distinguish exact vs. estimated token counts visually (see [Architecture](Architecture.md#5-exact-token-counts-where-available-heuristic-only-where-not))
- Keyboard navigation, status indicators, color-coded usage bars

## Phase 4 — Export/Inject

- Portable JSON context format (`Decisions` field removed per [Decisions](Decisions.md))
- `Files[].Content` opt-in only via `--with-content` (see [Architecture](Architecture.md#8-privacy-exported-file-content-is-opt-in-not-automatic))
- Per-agent injection strategies; clipboard fallback

## Phase 5 — Custom provider system

- YAML-based external provider config (as drafted)
- This is also where a real config file format gets introduced for the whole tool — don't build a second one before this

## Phase 6 — Polish & cross-platform

- Windows live-PID check implementation (see open question in [Decisions](Decisions.md))
- macOS kqueue watch-limit handling, Linux inotify max_user_watches warnings
- goreleaser-based distribution
