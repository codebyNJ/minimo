# Phase 3 — TUI Design

**Date:** 2026-06-22
**Status:** Approved (presented in chat; user proceeded straight to implementation)

## Why

`ctx status`/`ctx status --watch` work but are a flat, reprinted-text table — not the "btop for AI agents" experience the project set out to build. Phase 3 closes that gap: a real terminal dashboard, built deliberately **cleaner and lighter weight than btop**, not a clone of it.

## Locked decisions (from brainstorming, with the visual companion)

| Decision | Answer | Why |
|---|---|---|
| Layout | **Single scrollable/sortable list** across all providers/sessions, not per-provider panels | draft.md's original 4-panel mockup assumed ~1 session per provider; real usage on this machine is 35+ Claude Code sessions and 47+ OpenCode sessions — fixed panels don't fit |
| Visual density | **Option A** — full-width colored fill bar, heavy/dense styling (chosen from a 3-way visual mockup comparison: full bars / minimal text-only / small inline bar) | User's explicit pick after seeing all three rendered |
| What gets a bar | **Only CONTEXT** (the one metric with a natural ceiling: current/limit). LIFETIME and COST stay plain numbers | Forcing a bar onto a number with no ceiling (a running total) would be decorative, not informative |
| Default scope | **Active/Idle only by default; `h` toggles full history** | Matches btop's actual behavior (dead processes vanish) — a real dev machine's full session history is 80+ rows, which defeats "cleaner/lighter" if always shown |
| Row selection | **Browse-only highlight, no action fires in v1** | Keeps Phase 3 scope tight; export-from-TUI and a detail/drill-down panel are real future features, not built now |
| Entry point | Bare `ctx` (no subcommand) launches the TUI — confirmed from draft.md's original CLI table (`ctx → Open TUI`, `ctx status → CLI`). `ctx status`/`ctx export` are **unchanged**, not replaced | Two genuinely different surfaces for two different use cases (interactive dashboard vs. scriptable flat output) |

## Architecture

The TUI does **not** reimplement refreshing. `cmd/ctx/main.go` already has a correct, twice-proven dual-trigger refresh loop (fsnotify for Claude Code's JSONL, a 2s poll ticker for SQLite providers) inside `runWatch`. Phase 3 extracts that loop into a shared `watchLoop(ctx, cfg, onTrigger func())` helper used by **both** the existing flat `--watch` mode and the new TUI — the TUI's `onTrigger` calls `e.Refresh()` then bridges into Bubble Tea via `tea.Program.Send(ui.RefreshMsg{})` instead of `fmt.Print`.

Color/format logic that both the flat table and the TUI need (`formatCount`, `formatContext`, `formatCost`, etc., currently private to `cmd/ctx/main.go`) is extracted into a new shared `internal/format` package so neither path duplicates it.

```
internal/format/format.go   # extracted, exported: FormatCount, FormatContext, FormatCost, EmptyDash, Truncate, TruncateRight
internal/ui/
├── model.go    # Model struct, New(), Init(), RefreshMsg
├── update.go   # Update(): WindowSizeMsg, RefreshMsg, KeyMsg (q/ctrl+c quit, h toggle history)
├── view.go     # View(): summary header + table.View()
├── bar.go      # renderContextBar (color-thresholded fill bar), statusDot
└── rows.go     # visibleRows (filter+sort), rowsToTableRows, tableColumns, tableStyles
```

**Tech stack** (all verified via `go list -m -versions` and `go doc` against the real installed module — not guessed):
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`
- `github.com/charmbracelet/bubbles@v1.0.0` (for its `table` component — gives scrolling, cursor movement, and key navigation for free, satisfying "browse-only, no action" without hand-rolling pagination math)

**Color thresholds for the CONTEXT bar:** green `<70%`, yellow `70-90%`, red `>90%` of the model's window — standard convention, proposed in the design presentation, not objected to.

## Out of scope (named explicitly, not silently dropped)

- Export-from-TUI, detail/drill-down panel on row selection
- Per-provider panels
- Search/filter by text
- Mouse support
- Bars on LIFETIME/COST

## Verification approach

No automated tests (project-wide deliberate constraint, unchanged). `go build`/`go vet` plus real-data manual checks, same as every prior phase. **One exception worth naming:** a genuinely interactive Bubble Tea TUI (raw terminal mode, real keypresses, visual color confirmation) is not something a sandboxed subagent can reliably drive end-to-end without a real TTY. Task 4's implementer does what's mechanically verifiable (build, vet, launch-and-confirm-no-crash); full interactive confirmation (pressing `h`, visually checking bar colors, scrolling) is done by the controller after the subagent's automated portion, before the task is marked reviewed.
