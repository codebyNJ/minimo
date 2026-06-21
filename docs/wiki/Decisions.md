# Decisions Log

Every entry here is a point where we deliberately chose not to assume and asked instead. Keep this updated — it's the record of *why* the code looks the way it does.

| Date | Question | Decision | Why |
|---|---|---|---|
| 2026-06-21 | Should this folder become a git repo now? | Yes | Plan commit steps and wiki versioning need it. |
| 2026-06-21 | Where does the wiki live? | `docs/wiki/*.md`, plain markdown, versioned with the code | No external dependency, viewable anywhere, no GitHub-Wiki setup overhead before a remote even exists. |
| 2026-06-21 | How much should the first implementation plan cover? | Phase 1 only (skeleton + Claude Code provider + `ctx status`) | Later phases depend on design choices (provider interface shape, token accounting) that should be proven against one real provider before being locked in for four. |
| 2026-06-21 | Should the plan include automated tests? | No — manual run/verify steps only, no test files | Explicit user instruction: exclude testing code entirely. |
| 2026-06-21 | Go module path | `github.com/codebyNJ/minimo` | User's GitHub identity. |
| 2026-06-21 | What does `Context.Decisions` mean, given "zero AI" can't summarize intent from logs? | Dropped from v1 entirely | Re-add later only with a named, non-AI source (e.g. git commits in the session window) — not as a vague freeform list. |
| 2026-06-21 | License | MIT | Standard for Go CLI tools, no reason to deviate. |

## Open questions not yet resolved

- Cursor's `state.vscdb` schema is reported to have shifted (`agentKv` now the largest key family per 2026 web research) — needs empirical verification against an actual Cursor install before a Cursor provider is built. **No longer urgent**: Cursor was deferred to [Future scope](Roadmap.md#future-scope--deferred-not-actively-planned) on 2026-06-21, so this only matters again if/when Cursor comes back into scope. Also now confirmed the `cursor-agent` CLI uses a *different* storage path (`~/.config/cursor/chats/`) than the desktop IDE's `state.vscdb` — whoever revisits this still needs to decide whether a Cursor provider targets the CLI surface, the desktop surface, or both.
- Windows live-PID check (`isAlive(pid)`) needs a concrete implementation choice: shell out to `tasklist`, or use a Windows-specific syscall via `golang.org/x/sys/windows`. Not resolved yet — flagged as a task-level decision for whoever implements [Architecture#1](Architecture.md#1-session-status-must-come-from-the-live-pid-registry-not-file-mtimes).
- **(New, 2026-06-21)** Should `ctx` build a generic OS-process-scan liveness fallback (match a known binary name to a running PID, cross-reference cwd) for the 7 of 8 researched tools that have no vendor live-PID registry? See the [comparison table](Research.md#live-pid-registry-comparison-across-all-researched-tools) — this is a bigger lift than Phase 1's single Windows `tasklist` check (needs per-OS cwd-of-pid logic) and isn't scoped into any phase yet. **Lower urgency than it looked at first** — see [Architecture #11](Architecture.md#11-liveness-and-usage-are-independent-signals--dont-conflate-them): usage/context numbers (the main reason to look at the dashboard) don't depend on this at all, so liveness precision is a polish item, not a blocker. Still not decided — needs your call before it's added to the Roadmap.

## Resolved

- ~~Codex and Gemini CLI don't have a confirmed live-PID-style registry~~ — **confirmed absent**, not just unverified, after a dedicated 2026-06-21 search (see [Research](Research.md#live-pid-registry-comparison-across-all-researched-tools)). Both fall back to mtime-based inference, same as originally planned, now on firmer evidence.
- ~~Aider has no per-session storage at all... not decided whether Aider is in scope at all~~ — **deferred to Future scope** alongside Cursor/Gemini CLI/Copilot CLI on 2026-06-21 (see [Main vs. future scope provider set](#main-vs-future-scope-provider-set-2026-06-21) below). The `Provider`-interface-fit problem is real but moot until Aider is actually picked back up.

## Main vs. future scope provider set (2026-06-21)

| Question | Decision | Why |
|---|---|---|
| Which providers does `ctx` actively build toward (beyond Phase 1's Claude Code), and which get parked? | **Active scope (Phase 2): OpenCode, Codex, Kimi Code** — the 3 tools where exact usage data is confirmed on real evidence (ground-truth DB inspection for OpenCode; documented wire-format fields for Codex/Kimi Code). **Future scope (deferred, no phase): Gemini CLI, Cursor, GitHub Copilot CLI, Aider** — usage data unconfirmed, unchecked, or (Aider) structurally absent. | User's explicit call: "what ever you have found the 4 keep that as the main remaining unfound just keep in future scope." Confirmed-data tools are lower-risk to build and include both tools the user runs daily (Claude Code, OpenCode) — see [Research](Research.md#contexttoken-usage-comparison--the-question-that-actually-matters-more-than-liveness) for the underlying comparison. |

See [Roadmap](Roadmap.md#phase-2--remaining-providers-scope-locked-2026-06-21-the-4-confirmed-usage-data-tools) for the resulting phase plan and [Roadmap — Future scope](Roadmap.md#future-scope--deferred-not-actively-planned) for the parked list.
