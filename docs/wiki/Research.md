# Research Findings

Everything below was verified against either (a) real files on this machine, or (b) a live web search done on 2026-06-21. Anything not verified is marked **unverified — assumption carried from draft.md**.

## Claude Code — verified by reading real files on this machine

Source of truth: `~/.claude/projects/<sanitized-cwd>/<sessionId>.jsonl`

- Path sanitization: the project's absolute path has `:` and `\`/`/` replaced with `-`. `d:\codes\minimo` → `d--codes-minimo`. Confirmed by listing `~/.claude/projects/`.
- One file per session, named `<uuid>.jsonl`. A single active project can have multiple session files (old + current).
- **File size is unbounded and can get large** — observed a 41MB JSONL from one ongoing session on this machine. Any reader must not load+reparse the whole file on every change (see [Architecture](Architecture.md#streaming-tail-reader)).
- Each line is one JSON object. Observed `type` values: `user`, `assistant`, `ai-title`, `file-history-snapshot`, `last-prompt`, `attachment`, `queue-operation`.
- **`assistant` lines carry an exact token count** in `message.usage`: `input_tokens`, `output_tokens`, `cache_read_input_tokens`, `cache_creation_input_tokens`. This means Claude Code token usage does **not** need to be estimated — sum the real numbers. (draft.md assumed a char/4 heuristic for everyone; that's only needed for agents that don't expose usage.)
- **`ai-title` lines carry a human-readable session title** in `aiTitle` (e.g. `"Design architecture and create project wiki"`). Use this directly for `SessionInfo.Label` — no heuristic needed, draft.md left this unspecified.
- Tool calls live inside `message.content[]` items with `type: "tool_use"`, carrying `name` (e.g. `"Bash"`, `"Read"`) and `input`. Files read/written can be derived from `Read`/`Write`/`Edit` tool_use entries.
- `cwd`, `sessionId`, `gitBranch`, `version`, `timestamp` are present on every line — useful for correlating a session to a project and a point in time.

### Live-session registry (bigger finding than anything in draft.md)

`~/.claude/sessions/<pid>.json` — one file per **running** Claude Code process, named by OS process ID, containing:

```json
{"pid":14580,"sessionId":"...","cwd":"d:\\codes\\minimo","startedAt":1782024051338,"version":"2.1.183","peerProtocol":1,"kind":"interactive","entrypoint":"claude-vscode"}
```

This is the authoritative "is this session live right now" signal. draft.md's design inferred Active/Idle/Ended purely from JSONL file mtimes — that's a weaker, race-prone signal (a file can go quiet because the agent is "thinking" for 30s, not because it ended). The correct check: read `~/.claude/sessions/*.json`, then verify the PID is actually still running (`os.FindProcess` / `tasklist` check on Windows). If the PID is alive → Active. If the JSONL has changed in the last debounce window but no live PID file → Idle (process exited uncleanly) or Ended.

## OpenAI Codex CLI — verified via web search (2026-06-21)

- Sessions live at `~/.codex/sessions/YYYY/MM/DD/rollout-<timestamp>-<uuid>.jsonl` — **not** `~/.codex/session_index.jsonl` as draft.md assumed. There is no separate index file; the date-bucketed directory tree is the index.
- First line of each rollout file is a metadata header (session id, source, timestamp, model provider); subsequent lines are turn/tool-call events.
- Resuming a session appends to the same rollout file rather than creating a new one.
- Confirmed real-world pain point: [openai/codex#24948](https://github.com/openai/codex/issues/24948) reports session logs growing to 700MB–2GB from repeated compaction history — this is direct evidence the streaming-tail-reader requirement in [Architecture](Architecture.md) is not optional, it's necessary for Codex specifically.
- `history.persistence` and `history.max_bytes` config options exist to cap file growth — worth detecting in `ctx` to set reader expectations, but not required for v1.

- **No live-PID registry found** (checked again specifically on 2026-06-21 while researching liveness signals across tools — see [comparison table](#live-pid-registry-comparison-across-all-researched-tools) below). Codex does maintain `~/.codex/session_index.jsonl`, a metadata cache (id, timestamp, cwd, model, **status**) covering both active and archived sessions — but its `status` enum isn't documented anywhere public, and nothing suggests it's a live-process signal rather than a completed/archived/interrupted marker written on exit. Treat Codex liveness as mtime-based inference only, same conclusion as before, now confirmed by a dedicated search rather than just "no local install to check."
- **Usage data: exact field names confirmed (2026-06-21).** Inside the rollout `.jsonl`, lines with `event_msg.payload.type == "token_count"` carry a `total_token_usage` object (`input_tokens`, `cached_input_tokens`, `output_tokens`, `reasoning_output_tokens`, `total_tokens`) plus a `last_token_usage` object of the same shape for the most recent turn. `total_token_usage` is already cumulative, so a Codex provider can just read the **last** `token_count` event in the file and use `total_tokens` directly — no summation needed, similar to OpenCode's precomputed columns below. One sharp edge reported in the wild: Codex re-emits a `token_count` event on rate-limit-only updates without changing `total_token_usage`, which has caused real overcounting bugs in at least one third-party tool — a provider must key off the latest event, not count/sum every `token_count` line seen.
- Prior art worth knowing about, not building from scratch blind: [codex-viz](https://github.com/onewesong/codex-viz) is an existing local-first dashboard for Codex session trends/token usage, and [ccusage](https://ccusage.com/guide/codex/) is a multi-agent usage-analytics CLI that already parses Claude Code, Codex, **and** Kimi Code usage data (see below) — different goal from `ctx` (offline reporting vs. live monitoring), but proof the rollout format is solvable and worth a skim before writing the Codex provider.

Sources: [Codex CLI Config Location](https://inventivehq.com/knowledge-base/openai/where-configuration-files-are-stored), [Session logs grow to 700MB-2GB · Issue #24948](https://github.com/openai/codex/issues/24948), [Advanced Configuration](https://developers.openai.com/codex/config-advanced), [Codex CLI Session Archiving](https://codex.danielvaughan.com/2026/06/02/codex-cli-session-archiving-lifecycle-management-v0136/), [Reverse engineering Codex CLI rollout traces](https://dev.to/milkoor/reverse-engineering-codex-cli-rollout-traces-3b9b), [ccusage Codex guide](https://ccusage.com/guide/codex/)

## Cursor — verified via web search (2026-06-21)

- Confirms draft.md's core claim: chat/composer state lives in `state.vscdb` (SQLite), table `cursorDiskKV`, keys `composerData:<composerId>` (session metadata) and `bubbleId:<composerId>:<bubbleId>` (individual messages, ordered by `rowid`).
- New finding not in draft.md: there's also a `messageRequestContext` key family (prompt-building context snapshots) and `checkpointId` (restore/inline-diff state). The **largest key family in practice is `agentKv`**, not `composerData` — worth re-checking schema empirically once a Cursor install is available to test against, since this evolves with Cursor versions.
- draft.md's instinct to poll rather than fsnotify `state.vscdb` is correct — SQLite write-ahead-log files (`-wal`, `-shm`) make file-level watching unreliable.
- **Refinement (2026-06-21):** the `state.vscdb` SQLite path above is the **desktop IDE's** storage. The separate `cursor-agent` **CLI** (headless/terminal mode, what `ctx` would actually integrate with) stores chats differently: `~/.config/cursor/chats/` on Linux (or `~/.cursor/chats/` per a third-party extraction tool). These are two different surfaces with two different storage mechanisms — a Cursor provider needs to decide which one (or both) it targets. Not yet empirically confirmed against a real install; still needs the verification flagged in [Decisions](Decisions.md#open-questions-not-yet-resolved).
- No PID file or liveness registry found for either surface.

Sources: [What Does Cursor Store on Your Machine?](https://vibe-replay.com/blog/cursor-local-storage/), [Cursor Data Storage Model](https://zread.ai/S2thend/cursor-history/7-cursor-data-storage-model), [cursor-session extraction tool](https://github.com/iksnae/cursor-session), [Cursor CLI Configuration](https://cursor.com/docs/cli/reference/configuration)

## Gemini CLI — verified via web search (2026-06-21)

- Confirms draft.md: sessions/chats under `~/.gemini/tmp/<project_hash>/chats/`, project hash derived from the project root path (same idea as Claude Code's path sanitization, different algorithm — needs empirical check once installed).
- New detail: explicit `/chat save` checkpoints are plain JSON files named `checkpoint-<name>.json`; automatic pre-edit checkpoints live under `~/.gemini/tmp/<project_hash>/checkpoints/`. draft.md only mentioned the chats directory.
- **No live-PID registry found.** There is a lock file in Gemini CLI, but it guards the unrelated "Auto Memory" feature — it makes sure only one CLI instance runs the background memory-extraction job at a time across a project. It is not a "this session is currently running" marker and isn't keyed by session ID. Searched GitHub issues directly for session+pid/lock discussion on 2026-06-21 and found nothing describing a liveness registry. Same conclusion as Codex: mtime-based inference only, now confirmed by search rather than "unverified."
- **Ground-truth caveat (2026-06-21):** this machine has Gemini CLI data at `~/.gemini/tmp/<hash>/logs.json` — but the one real file found contains only `{sessionId, messageId, type:"user", message, timestamp}` entries, i.e. the literal text the user typed, with **no assistant responses and no token/usage data at all**. This doesn't match the docs' description of a `chats/` subdirectory with full transcripts — either this install's data is sparse/stale (the file's dates are old), or full-transcript persistence isn't on by default and needs an explicit setting. Flagging honestly rather than assuming the docs' happy path is what actually happens: **Gemini CLI usage-data availability is unconfirmed and may be weaker than Codex/OpenCode/Claude Code/Kimi Code**, pending a check against a more actively-used Gemini CLI install.

Sources: [Checkpointing — Gemini CLI](https://geminicli.com/docs/cli/checkpointing/), [Session management — Gemini CLI](https://geminicli.com/docs/cli/session-management/), [Auto Memory — Gemini CLI](https://geminicli.com/docs/cli/auto-memory/)

## OpenCode — verified via web search (2026-06-21)

Not in draft.md at all — added after the user asked specifically about this tool. Note the naming collision: `opencode-ai/opencode` (Go) is a *different* project from `sst/opencode`, which appears to have since moved to `anomalyco/opencode` (the one with an active GitHub issue tracker and the `opencode.ai` docs site). The findings below are for the `anomalyco/opencode` (ex-`sst/opencode`) one, since it's the one with docs and an active community.

- Session data lives under `~/.local/share/opencode` by default, in a **SQLite database** (not JSONL). The data directory is configurable via `"data": {"directory": ...}` in config.
- OpenCode has a client/server architecture: `opencode serve [--port] [--hostname] [--cors]` starts a standalone HTTP server; the TUI itself launches a server on a **randomly assigned port** unless one is passed explicitly.
- **No live-PID registry or instance-tracking file found** — and this looks like a confirmed *absence*, not just an unverified gap: [issue #7629](https://github.com/anomalyco/opencode/issues/7629) is an open feature request asking OpenCode to "connect to existing server instead of starting a new one when port is already in use," i.e. OpenCode doesn't yet track its own running instances. A `ctx` integration would have to detect a running `opencode serve`/TUI process by OS process scan, not by reading a file OpenCode writes for this purpose.

Sources: [OpenCode CLI docs](https://opencode.ai/docs/cli/), [OpenCode Server docs](https://opencode.ai/docs/server/), [anomalyco/opencode issue #7629](https://github.com/anomalyco/opencode/issues/7629), [anomalyco/opencode issue #10349 — cross-platform session visibility](https://github.com/anomalyco/opencode/issues/10349)

### OpenCode — ground-truth verified by inspecting a real install on this machine (2026-06-21)

The user runs OpenCode daily, so this was checked directly against `~/.local/share/opencode/opencode.db` (a real, 35MB, actively-written WAL-mode SQLite file) using Node's built-in `node:sqlite` module — no assumptions, actual `CREATE TABLE` statements and real rows.

**This is the single best usage-data finding of this research round.** The `session` table has these columns directly on it, no JSONL parsing or transcript summation needed at all:

```sql
CREATE TABLE `session` (
  `id` text PRIMARY KEY, `project_id` text NOT NULL, `directory` text NOT NULL,
  `title` text NOT NULL, `model` text NOT NULL,            -- JSON: {"id":"...","providerID":"...","variant":"..."}
  `cost` real DEFAULT 0 NOT NULL,
  `tokens_input` integer DEFAULT 0 NOT NULL,
  `tokens_output` integer DEFAULT 0 NOT NULL,
  `tokens_reasoning` integer DEFAULT 0 NOT NULL,
  `tokens_cache_read` integer DEFAULT 0 NOT NULL,
  `tokens_cache_write` integer DEFAULT 0 NOT NULL,
  `time_created` integer NOT NULL, `time_updated` integer NOT NULL,
  `time_compacting` integer, `time_archived` integer,
  ...
)
```

Real sample row (numbers only, no message content read or quoted): a session on this machine shows `tokens_input: 9358855`, `tokens_output: 53877`, `tokens_reasoning: 20493`, `tokens_cache_read: 6559360`, model `deepseek-v4-flash-free`. These are live, non-zero, already-aggregated totals OpenCode maintains for its own UI — `ctx` just has to `SELECT` them.

Implications:
- **OpenCode's usage data is exact (`TokenSourceExact`) and requires zero parsing** — easier than Claude Code's design, which sums per-turn `usage` objects across an entire JSONL. OpenCode also tracks `cost` (real dollars) and separates `tokens_reasoning` and `tokens_cache_write` from plain input/output — finer-grained than Claude Code's four fields.
- `time_archived` (nullable) and `time_updated` give a usable Ended/likely-idle signal without any process check.
- Other tables worth knowing about for later: `session_context_epoch` (tracks context-compaction baseline/snapshot/revision — not raw usage, this is OpenCode's own context-compaction bookkeeping) and `message`/`part` (the actual per-turn transcript content, stored as opaque `data text` JSON blobs — only needed if `ctx` ever wants per-turn detail, not needed for the aggregate-tokens use case).
- Confirms the earlier finding: this is SQLite with `-wal`/`-shm` sidecar files, same caution as Cursor — poll on a timer, don't try to `fsnotify` the file directly (see [Architecture #6](Architecture.md#6-file-change-detection-must-handle-truncationrewrite-not-just-append), now generalized to any SQLite-backed provider, not just Cursor).
- Schema uses Drizzle ORM migrations (`__drizzle_migrations` table) — column names have been stable enough to show up consistently across the rows checked, but as with any unversioned local schema, a provider implementation should tolerate a missing column rather than fail hard if a future OpenCode update renames one.

## Kimi Code CLI (Moonshot AI) — verified via web search (2026-06-21)

Not in draft.md. Real directory layout, fetched directly from the official docs:

```
$KIMI_CODE_HOME (default ~/.kimi-code)
├── config.toml, tui.toml, AGENTS.md, mcp.json
├── session_index.jsonl
├── sessions/<workDirKey>/<sessionId>/
│   ├── state.json
│   ├── upcoming-goals.json
│   ├── agents/main/wire.jsonl       # the actual conversation transcript
│   ├── agents/main/plans/
│   ├── logs/
│   └── tasks/<task_id>.json         # status/pid/exit code — see below
├── credentials/, plugins/, skills/, logs/, updates/, user-history/
```

- Per-session storage is path-keyed by working directory (`workDirKey`) + session ID, same shape as Claude Code's sanitized-cwd directories.
- **Partial PID finding:** `tasks/<task_id>.json` stores `status`/`pid`/`exit code` — but this is for **background sub-tasks spawned during a session**, not the top-level interactive session process itself. Useful later if `ctx` ever wants to show "this session has a running background task," but it doesn't answer "is the session itself live." No PID file was found for the session level — same mtime-fallback conclusion as the others.
- Whole tree is relocatable via `KIMI_CODE_HOME`; multiple Kimi instances pointed at the same `KIMI_CODE_HOME` share config/credentials.
- **Usage data: confirmed via the wire protocol docs (2026-06-21).** `agents/main/wire.jsonl` records `StatusUpdate` messages that include a `TokenUsage` object: `input_other` (input tokens excluding cache fields), `output`, `input_cache_read`, `input_cache_creation`. Only `StatusUpdate` messages with non-zero usage are written, so a reader should scan for the latest one rather than expect one per line. Same shape of problem as Claude Code/Codex — sum or take-latest from a structured field, no heuristics needed.
- `ccusage` (the prior-art usage-analytics CLI mentioned in the Codex section) also already covers Kimi Code, per its own docs — further confirms this format is real and stable enough for a third party to depend on.

Sources: [Kimi Code Data Locations](https://www.kimi.com/code/docs/en/kimi-code-cli/configuration/data-locations.html), [Kimi Code Environment Variables](https://www.kimi.com/code/docs/en/kimi-code-cli/configuration/env-vars.html), [MoonshotAI/kimi-cli on GitHub](https://github.com/MoonshotAI/kimi-cli), [Kimi Code Wire Protocol](https://www.kimi.com/code/docs/en/kimi-code-cli/customization/wire-protocol.html), [ccusage Kimi guide](https://ccusage.com/guide/kimi/)

## GitHub Copilot CLI — verified via web search (2026-06-21)

Not in draft.md; the user asked for "a few more famous" harnesses, this is one of the obvious ones.

- First-party docs ([GitHub Docs](https://docs.github.com/en/copilot/concepts/agents/copilot-cli/chronicle)) state sessions persist under `~/.copilot/session-state/{session-id}/`, containing `events.jsonl` (full history), `workspace.yaml` (metadata), `plan.md` (if a plan was created), `checkpoints/` (compaction history), and `files/` (persisted artifacts). A third-party technical writeup (DeepWiki) names the active-session directory as `~/.copilot/sessions/` instead — possibly version drift between when each source was written, not yet reconciled against a real install.
- Config dir is `~/.copilot/` (override via `XDG_CONFIG_HOME` or `COPILOT_HOME`).
- **No live-PID/lock file found.** Stronger signal than "we didn't find one": [github/copilot-cli issue #1792](https://github.com/github/copilot-cli/issues/1792) is an open feature request for a "Global session history registry with persistent cross-session stats accessible via `/history`" — confirming this registry doesn't exist yet as of the issue's filing. Mtime-based inference is the only option today.

Sources: [About GitHub Copilot CLI session data](https://docs.github.com/en/copilot/concepts/agents/copilot-cli/chronicle), [Copilot CLI config dir reference](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-config-dir-reference), [DeepWiki: copilot-cli session management](https://deepwiki.com/github/copilot-cli/3.3-session-management-and-history), [Issue #1792](https://github.com/github/copilot-cli/issues/1792)

## Aider — verified via web search (2026-06-21)

Not in draft.md. Structurally the odd one out among everything researched so far:

- Storage is a **single flat Markdown file per repo**, `.aider.chat.history.md`, in the project's working directory — not a per-session JSONL/SQLite store, and not under a `~/.aider*` home directory. There's no per-session ID at all; every run appends to the same file.
- Process model: no daemon/server. "There is no separate server mode — Aider is just a CLI." It runs in the terminal in the foreground (or one-shot via `--message` for headless/non-interactive use) and exits when done.
- **Consequence for `ctx`:** liveness for Aider isn't a "read a registry" problem at all — it's "is a process named `aider` currently running," answerable only via OS process scan, and even then there's no clean way to map a running `aider` process back to a specific *session* the way the others allow, since the chat history file has no session boundaries. This is a meaningfully different integration shape from every other tool researched — flagging as a design question rather than assuming it fits the existing `Provider` interface unchanged (see [Decisions](Decisions.md#open-questions-not-yet-resolved)).

Sources: [Aider GitHub](https://github.com/Aider-AI/aider), [Aider docs](https://aider.chat/docs/), [Aider FAQ](https://aider.chat/docs/faq.html)

## Live-PID-registry comparison across all researched tools

Direct answer to "does this tool tell us a session is live without guessing from file timestamps":

| Tool | Storage shape | Live-PID / liveness registry? |
|---|---|---|
| Claude Code | per-session JSONL | **Yes** — `~/.claude/sessions/<pid>.json`, confirmed real on this machine |
| Codex CLI | per-session JSONL (date-bucketed) + `session_index.jsonl` cache | No — index has an undocumented `status` field, not a process registry |
| Gemini CLI | per-project chat JSON + checkpoints | No — the only lock file found guards an unrelated background feature |
| OpenCode | SQLite db, client/server | No — confirmed absent; open feature request asks for this exact capability |
| Kimi Code CLI | per-session directory tree | Partial — PID tracked only for background *tasks*, not the session itself |
| GitHub Copilot CLI | per-session directory (`events.jsonl`, etc.) | No — open feature request confirms no registry exists yet |
| Cursor (`cursor-agent` CLI) | per-chat files (`~/.config/cursor/chats/`) | No — and the CLI's storage differs from the desktop IDE's `state.vscdb` |
| Aider | single flat `.aider.chat.history.md` per repo | N/A — no daemon, no session boundary; liveness = OS process scan only |

**Takeaway:** Claude Code is the only tool in this set that ships a vendor-provided "is this session live right now" file. Every other provider `ctx` adds will need the mtime-based-inference fallback already designed in [Architecture #1](Architecture.md#1-session-status-must-come-from-the-live-pid-registry-not-file-mtimes) — that fallback isn't a stopgap for one or two providers, it's the *default path* for 7 of the 8 tools researched so far. A generic OS-process-scan fallback (match a known binary name to a running PID, cross-reference its working directory with the session's cwd) would improve accuracy for all of them, but needs real per-OS work (`/proc/<pid>/cwd` on Linux, `lsof`/`proc_pidinfo` on macOS, WMI/`Get-CimInstance Win32_Process` on Windows) — bigger than the single Windows `tasklist` check Phase 1 already needs, and not yet scoped into any phase.

## Context/token usage comparison — the question that actually matters more than liveness

Liveness (Active/Idle/Ended) and usage (tokens/cost/context) are **independent** questions. The table above is about the first. This one is about the second, and the answer is much better news:

| Tool | Usage data available? | Where, exactly |
|---|---|---|
| Claude Code | **Yes, exact** | sum `message.usage.{input_tokens,output_tokens,cache_read_input_tokens,cache_creation_input_tokens}` across the session JSONL |
| OpenCode | **Yes, exact — ground-truth confirmed on this machine** | `session` table, columns `tokens_input`/`tokens_output`/`tokens_reasoning`/`tokens_cache_read`/`tokens_cache_write`/`cost` — already aggregated, one `SELECT`, no parsing |
| Codex CLI | **Yes, exact** | latest `event_msg` with `payload.type=="token_count"` → `total_token_usage.{input_tokens,cached_input_tokens,output_tokens,reasoning_output_tokens,total_tokens}`, already cumulative |
| Kimi Code CLI | **Yes, exact** | latest `StatusUpdate` in `wire.jsonl` → `TokenUsage.{input_other,output,input_cache_read,input_cache_creation}` |
| Gemini CLI | **Unconfirmed** | docs describe a `chats/` transcript directory; the one real file found on this machine had no usage data at all (see caveat above) — needs a more active install to verify |
| GitHub Copilot CLI | Not yet checked for usage fields specifically | `events.jsonl` per session is the likely place; not fetched/confirmed this round |
| Cursor (`cursor-agent`) | Not yet checked for usage fields specifically | would be inside the per-chat files under `~/.config/cursor/chats/`; not fetched/confirmed this round |
| Aider | **No** | `.aider.chat.history.md` is prose/diff output, not structured usage data; would need the chars/4-style heuristic if ever supported |

**This is the more important takeaway than the liveness table above.** 4 of the tools researched (Claude Code, OpenCode, Codex, Kimi Code) — including both tools the user actually runs day to day — expose **exact, pre-computed or trivially-summed** token usage with no heuristics. The `Provider` interface's `TokenUsage{Total, Source}` (see [Architecture #5](Architecture.md#5-exact-token-counts-where-available-heuristic-only-where-not)) already accommodates all of them without changes — `TokenSourceExact` just gets populated by a SQL query instead of a JSONL sum for OpenCode, that's an implementation detail inside each provider, not an interface change. A "btop for AI agents" is not blocked by the liveness gap — it loses precision on *when exactly* a session went idle for 7 of 8 tools, but it does **not** lose the core context-usage numbers that are the main reason to glance at the dashboard in the first place.

## Re-verification before building Codex + Kimi Code providers (2026-06-23)

Neither tool is installed on this machine, so this is a fresh web-search re-check of the 2026-06-21 findings above (not a ground-truth file inspection like OpenCode got) — done because schemas for actively-developed CLIs can drift in two days. Two real updates found:

- **Codex CLI — file path refined, one new fact.** Active sessions: `~/.codex/sessions/YYYY/MM/DD/rollout-<session-id>.jsonl` (the session-id is embedded in the filename itself, not a separate timestamp+uuid pair as the original note implied). **New:** archived sessions move to a parallel `~/.codex/archived_sessions/YYYY/MM/DD/rollout-<session-id>.jsonl` tree — a provider that only scans `sessions/` will silently miss archived ones. Token schema unchanged and reconfirmed directly against `codex-rs` source discussion: `token_count` events carry `total_token_usage.{input_tokens,cached_input_tokens,output_tokens,reasoning_output_tokens,total_tokens}` (cumulative) and `last_token_usage` (same shape, per-turn delta). The rate-limit-only re-emission gotcha is a tracked upstream issue ([openai/codex#14489](https://github.com/openai/codex/issues/14489)), confirming the original "key off the latest event, don't sum" guidance still holds. Sources: [Codex CLI Session Archiving](https://codex.danielvaughan.com/2026/06/02/codex-cli-session-archiving-lifecycle-management-v0136/), [openai/codex#14489](https://github.com/openai/codex/issues/14489), [openai/codex codex.rs](https://github.com/openai/codex/blob/e2c994e32a31415e87070bef28ed698968d2e549/codex-rs/core/src/codex.rs).

- **Kimi Code CLI — genuinely new fields confirmed.** Fetched the official wire-protocol docs directly: `StatusUpdate` is documented as
  ```typescript
  interface StatusUpdate {
    context_usage?: number | null        // ratio 0-1
    context_tokens?: number | null        // tokens currently in context
    max_context_tokens?: number | null    // context window size
    token_usage?: TokenUsage | null
    message_id?: string | null
    plan_mode?: boolean | null
  }
  interface TokenUsage {
    input_other: number
    output: number
    input_cache_read: number
    input_cache_creation: number
  }
  ```
  `context_usage`/`context_tokens`/`max_context_tokens` were **not** in the 2026-06-21 research — this is new. It means Kimi Code reports exact context-fullness (current tokens *and* the window ceiling) directly in the wire protocol, unlike Claude Code, which requires `ctx`'s own hardcoded per-model window-size table ([contextwindow.go](../../internal/provider/claudecode/contextwindow.go)). All `StatusUpdate` fields are optional/nullable — a reader must tolerate any subset being absent on a given line, not assume every field is always present. Source: [Wire mode — Kimi Code CLI Docs](https://moonshotai.github.io/kimi-cli/en/customization/wire-mode.html).

Both are still **not ground-truth-verified against a real install** — if Codex or Kimi Code usage on this machine starts in the future, re-check against real files before trusting this section blindly, same standard as everything else here.

## Codex rollout JSONL — exact schema from source (2026-06-23)

The two sources above (a blog post, a GitHub issue) only paraphrased the format. Pulled the actual struct definitions from `codex-rs/protocol/src/protocol.rs` (fetched via `gh api repos/openai/codex/contents/...`, the current `main` branch) for exact field names — this supersedes the paraphrased version above wherever they differ.

Every rollout line is a `RolloutLine { timestamp: String, #[serde(flatten)] item: RolloutItem }`, and `RolloutItem` is `#[serde(tag = "type", content = "payload", rename_all = "snake_case")]` with variants `session_meta`, `response_item`, `inter_agent_communication`, `compacted`, `turn_context`, `event_msg`. So every line on disk is shaped `{"timestamp": "...", "type": "<variant>", "payload": {...}}`.

- **`session_meta`** payload (`SessionMetaLine`, flattening `SessionMeta`): `session_id`, `id` (ThreadId), `timestamp` (session start, string), **`cwd` (PathBuf — confirmed present, contradicting the earlier assumption that Codex's metadata header has no cwd field)**, `originator`, `cli_version`, plus optional `git: GitInfo{commit_hash, branch, repository_url}`. No `model` field here.
- **`turn_context`** payload (`TurnContextItem`): `cwd` (AbsolutePathBuf, present on every turn, can differ from session_meta's if the agent changes directory), `model: String` (this is where the active model name actually comes from, not session_meta), plus approval/sandbox policy fields not relevant to `ctx`.
- **`event_msg`** payload is itself tagged (`EventMsg`, `#[serde(tag = "type", rename_all = "snake_case")]`) — for the token-count case: `{"type": "token_count", "info": TokenUsageInfo | null, "rate_limits": ... | null}`.
- **`TokenUsageInfo`**: `total_token_usage: TokenUsage`, `last_token_usage: TokenUsage`, and **`model_context_window: Option<i64>`** — Codex *does* report the context-window ceiling directly, contradicting the 2026-06-21 research's assumption that no window-size field exists. `info` itself is `Option` (can be `null`), and `model_context_window` is independently optional within it — both must be nil-checked, but when present, no hardcoded per-model table is needed at all, mirroring Kimi Code's `max_context_tokens` rather than Claude Code's hardcoded table.
- **`TokenUsage`** (the per-event fields, confirmed exact field names): `input_tokens`, `cached_input_tokens`, `output_tokens`, `reasoning_output_tokens`, `total_tokens` — all `i64`. Matches the 2026-06-21 finding's field names exactly; this round just confirmed the nesting (`payload.info.total_token_usage`, not `payload.total_token_usage` directly as the earlier paraphrase implied).

Source: [codex-rs/protocol/src/protocol.rs](https://github.com/openai/codex/blob/main/codex-rs/protocol/src/protocol.rs), fetched directly via `gh api repos/openai/codex/contents/codex-rs/protocol/src/protocol.rs` on 2026-06-23. This is the actual current source, not a secondary description — treat it as authoritative over the paraphrased findings above wherever they conflict.

## Libraries

| Library in draft.md | Status found | Recommendation |
|---|---|---|
| `LarsArtmann/go-filewatcher` | **Could not find this repository via web search.** Likely a hallucinated/mis-remembered dependency. | Don't depend on it. Roll a ~80-line recursive watcher directly on `fsnotify` (see [Architecture](Architecture.md#recursive-watch)) — fsnotify confirms recursive watching is still not in its public API as of 2026, third-party wrappers are themselves small, so wrapping it ourselves avoids an unverifiable dependency for little code savings. |
| `mattn/go-sqlite3` (implied for Cursor) | Requires cgo → complicates cross-compilation (a real concern since Phase 6 explicitly targets Windows/macOS/Linux). | Use `modernc.org/sqlite` instead — pure Go, no cgo, ~2x slower on inserts but `ctx` is read-only and only touches small KV rows, so the speed cost doesn't matter here. |
| `pkoukk/tiktoken-go` (token counting v2 idea) | Original is unmaintained. Forks exist (`weaviate/tiktoken-go`, `tiktoken-go/tokenizer`). | Not needed for v1 at all — Claude Code already gives exact counts (see above). Defer this whole line item; only revisit if a provider without usage data needs better-than-heuristic estimates. |
| Bubble Tea / Lip Gloss / Bubbles | Active, current, this is still the standard Go TUI stack in 2026. | Keep as planned for Phase 3. No change. |
