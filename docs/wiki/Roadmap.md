# Roadmap

Carried from `draft.md`, refined where research changed the plan. Only Phase 1 has a detailed task-by-task implementation plan so far (`docs/superpowers/plans/`); the rest stay at this level of detail until the phase before them ships.

## Phase 1 — Skeleton + Claude Code provider (in progress)

- `go.mod`, package layout per [Architecture](Architecture.md#folder-structure-phase-1-scope-full-project-shape-shown-for-context)
- `Provider` interface + compiled-in registry
- Claude Code provider: JSONL tail reader, exact token summing from `usage`, `ai-title` → session label, live-PID status check
- Recursive fsnotify watcher with debounce
- `ctx status` — flat CLI listing, no TUI yet

## Phase 2 — Remaining providers

- Codex provider: date-bucketed rollout file discovery (`~/.codex/sessions/YYYY/MM/DD/`), corrected from draft.md's assumed single index file
- Cursor provider: `modernc.org/sqlite` (cgo-free) reader against `cursorDiskKV`, 2s polling instead of fsnotify
- Gemini CLI provider: chat + checkpoint file parser
- Cross-provider session listing in `ctx status`
- **Before this phase starts:** verify Cursor's actual current schema and confirm/deny a Codex live-session registry exists, per [Decisions](Decisions.md#open-questions-not-yet-resolved)

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
