# Code Review & Risk Audit: Context-Fullness & Cost Metrics

**Date:** 2026-06-22
**Scope:** The 5-commit increment adding context-fullness + exact-cost metrics to `ctx` (the `MODEL`/`CONTEXT`/`COST` columns, the `TOKENS`→`LIFETIME` rename, and the supporting data-model/provider changes).
**Git range:** `861c282` (start) → `d00759e` (end of feature), plus follow-up fix `2257ca3`.
**Plan/spec:** [docs/superpowers/plans/2026-06-22-context-cost-metrics.md](../plans/2026-06-22-context-cost-metrics.md), [docs/superpowers/specs/2026-06-22-context-cost-metrics-design.md](../specs/2026-06-22-context-cost-metrics-design.md)

This is the single consolidated record requested: it folds together (1) the per-task reviews and final whole-branch review done during implementation, and (2) a separate forward-looking risk review focused on what could break later — with different data, future model strings, schema drift, or as the code evolves.

---

## 1. Reviews performed

| Review | When | Outcome |
|---|---|---|
| Per-task review ×4 (spec + quality) | During subagent-driven implementation | All 4 approved; 2 plan-mandated/consistent Minors deferred |
| Mid-implementation bug + fix | Task 4 manual verification | OpenCode `model` column found to be JSON-wrapped, not a bare string; fixed in `d00759e`, re-reviewed clean |
| Final whole-branch review (Opus) | After all tasks | **Ready to merge** — no Critical/Important findings |
| Forward-looking risk review | This audit | No Critical; 2 Important (display layer); rest Minor/acceptable |

---

## 2. What was built (one paragraph)

Three new fields on the provider data model — `SessionInfo.Model`, `SessionContext.Context` (`ContextUsage{Tokens, Known, Limit}`), and `SessionContext.Cost` (`Cost{USD, Known}`), each gated by a `Known bool` so a provider reports only what it can actually determine. Claude Code computes real context-fullness from the **latest** assistant turn's `input_tokens + cache_creation + cache_read` (replace-assigned, distinct from the unchanged lifetime `tokens` sum), with a window-size `Limit` from a hardcoded table containing only the two verified models (`claude-sonnet-4-6`, `claude-opus-4-8` → 1,000,000). OpenCode surfaces its exact `cost` column (previously queried and discarded) and its model name (parsed out of a JSON wrapper). The flat `ctx status` table renders `MODEL`/`CONTEXT`/`COST` columns and renames the ambiguous `TOKENS` header to `LIFETIME`. `internal/export` was deliberately left untouched.

---

## 3. Issues by severity

### Critical
None.

### Important

**I-1. CONTEXT can silently exceed its limit with no visual signal** — `cmd/ctx/main.go` `formatContext`
A row reading `2.0M/1.0M` renders as two plain numbers with no `%`, color, or warning glyph, so an over-limit session looks the same shape as a healthy one at a glance — degrading the exact "is this about to blow its context window" signal the feature exists for.
- **Trigger:** `contextTokens > Limit`. Note this can only happen when the *hardcoded `Limit` is wrong* (a single real API turn can't exceed the model's actual window), i.e. it's a downstream symptom of **I-3 / table staleness**, not an independent bug.
- **Decision: documented, not fixed this round.** The just-approved design deliberately chose the `current/limit` format (not a percentage); changing it unilaterally would deviate from the agreed spec. Overflow signaling (bars/color) is squarely Phase 3 (TUI) territory. **Tracked here so it isn't silently assumed-closed if Phase 3 slips.**

**I-2. `formatCount` rounding produced a `1000K` artifact at the most safety-relevant boundary** — `cmd/ctx/main.go` `formatCount`
Values in `[999,500, 999,999]` fell into the `>= 1_000` branch and rounded via `%.0f` to `"1000K"` instead of `"1.0M"` — precisely the near-full-context regime users most need to read correctly.
- **Decision: FIXED** in `2257ca3`. The K-branch now promotes to `M` when `n/1000 >= 999.5`. Verified across boundary values: `999499→999K`, `999500→1.0M`, `999999→1.0M`, `471000→471K` (unchanged), `1950000→1.9M`.

### Minor (deferred, with reasoning)

**M-1. Zero-turn Claude session reports `Known: true` with `Tokens: 0`** — `internal/provider/claudecode/provider.go` `ReadContext`
A session observed before its first assistant turn shows `0/1.0M` rather than `-`, asserting more confidence than the data supports. Plan-mandated; the window is a few seconds before the first reply; `0` is not damagingly wrong (the context genuinely is empty). **Defer.** If tightened later, gate `Known: state.model != ""`.

**M-2. OpenCode `model` scanned as plain `string`, not `sql.NullString`** — `internal/provider/opencode/queries.go`
A NULL `model` column would make `Scan` return a hard error for that row (fails loud, not silent corruption). Matches the pre-existing treatment of `title`/`directory`/`cost`. **Defer** — when hardened, do all four columns in one pass so the row struct stays uniform; don't special-case `model`.

**M-3. `parseModelName`: valid JSON missing an `id` key returns `""`** — `internal/provider/opencode/provider.go`
A future OpenCode schema using a different key (e.g. `modelId`) would parse without error, yield `ID == ""`, and render as `-` (via `emptyDash`) — indistinguishable from "no model data," with no diagnostic breadcrumb. The raw-string fallback only fires on a JSON *parse error*, not on valid-but-wrong-shape. **Defer** (optional): also fall back to the raw string when `m.ID == ""`, honoring the existing comment's stated intent ("rather than silently showing nothing").

**M-4. `truncate`/`truncateRight` slice on byte length, not runes** — `cmd/ctx/main.go`
A multi-byte/unicode `Model` or LLM-generated `Label` could slice mid-codepoint (producing a `�`) and under-pad the fixed-width columns. Dormant: all model strings and titles observed so far are ASCII. **Defer** — convert to `[]rune` before slicing if it ever surfaces.

### Acceptable as-is (explicitly confirmed, no action)

- **Unknown/future model string** (`contextwindow.go`): map miss → `Limit: 0` → `formatContext`'s `if c.Limit > 0` guard shows the raw count with no percentage. No crash, no bogus denominator. Verified graceful.
- **`configprovider` zero values:** `Model:""`, `Context{Known:false}`, `Cost{Known:false}` all degrade to `-` through the display helpers with no provider-specific code. Confirmed.
- **`contextTokens` holds its previous value** across a refresh batch that contains no assistant turn — correct ("context hasn't changed since the last turn"), not stale.
- **Cost precision floor:** `$%.4f` rounds a sub-$0.00005 cost to `$0.0000`, but `Known:true`+`$0.0000` is still distinct from `Known:false`+`-`, and `Cost.USD` retains full precision underneath. Acceptable for glance-monitoring.
- **SQLite WAL concurrency:** the new `model`/`cost` reads ride the same atomic single-row `SELECT` as the pre-existing columns — no new locking, join, or check-then-read race introduced. No new risk class.
- **Integer overflow / locale:** token `int` counts are ~8 orders of magnitude below `MaxInt64` on 64-bit targets; Go `fmt` is locale-independent (always `.` decimal). No action.

### I-3. Latent: the hardcoded window table will go stale silently

Not a bug today, but the one risk with no possible automated detection (by the project's no-tests design). A *new* model string degrades gracefully (raw count, no %). The dangerous case is a model string being **reused with a changed effective window** — the lookup still hits and shows a confident-but-wrong percentage with no signal. The table's `// Verified 2026-06-22` comment is good hygiene; the gap is the absence of a re-verification trigger.
- **Recommendation:** re-verify `contextwindow.go` window sizes whenever Claude Code's CLI version bumps. This is the natural follow-up obligation from this work — see §5.

---

## 4. Overall risk posture

Low-risk, additive, read-mostly feature. No Critical or crash-class defects. The single real bug found (I-2) is fixed. The remaining Important item (I-1) and the staleness risk (I-3) both live in the display/data-currency layer around the new CONTEXT column and are deliberately routed to Phase 3 (TUI) and a re-verification habit respectively, rather than papered over. Everything else is dormant or already graceful.

## 5. Follow-up obligations (forward-looking)

1. **Phase 3 (TUI):** add the over-limit visual signal (I-1) when it builds bars/color over these same fields.
2. **`contextwindow.go` re-verification:** re-check the two models' window sizes on Claude Code CLI version bumps (I-3) — silent-staleness risk, undetectable by design.
3. **Optional hardening passes** (M-2 NullString across all 4 columns; M-3 empty-id fallback; M-4 rune-aware truncation) — pick up only if a concrete trigger appears.
