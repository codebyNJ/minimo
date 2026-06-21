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

- Codex and Gemini CLI don't have a confirmed live-PID-style registry like Claude Code's `~/.claude/sessions/*.json`. Status for those two providers will be mtime-based and less reliable until/unless one is found. Revisit once Phase 2 starts and a Codex/Gemini install is available to inspect directly.
- Cursor's `state.vscdb` schema is reported to have shifted (`agentKv` now the largest key family per 2026 web research) — needs empirical verification against an actual Cursor install before Phase 2's Cursor provider is built; don't trust the schema notes in [Research](Research.md) blindly.
- Windows live-PID check (`isAlive(pid)`) needs a concrete implementation choice: shell out to `tasklist`, or use a Windows-specific syscall via `golang.org/x/sys/windows`. Not resolved yet — flagged as a task-level decision for whoever implements [Architecture#1](Architecture.md#1-session-status-must-come-from-the-live-pid-registry-not-file-mtimes).
