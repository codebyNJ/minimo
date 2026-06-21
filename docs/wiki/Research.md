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

Sources: [Codex CLI Config Location](https://inventivehq.com/knowledge-base/openai/where-configuration-files-are-stored), [Session logs grow to 700MB-2GB · Issue #24948](https://github.com/openai/codex/issues/24948), [Advanced Configuration](https://developers.openai.com/codex/config-advanced)

## Cursor — verified via web search (2026-06-21)

- Confirms draft.md's core claim: chat/composer state lives in `state.vscdb` (SQLite), table `cursorDiskKV`, keys `composerData:<composerId>` (session metadata) and `bubbleId:<composerId>:<bubbleId>` (individual messages, ordered by `rowid`).
- New finding not in draft.md: there's also a `messageRequestContext` key family (prompt-building context snapshots) and `checkpointId` (restore/inline-diff state). The **largest key family in practice is `agentKv`**, not `composerData` — worth re-checking schema empirically once a Cursor install is available to test against, since this evolves with Cursor versions.
- draft.md's instinct to poll rather than fsnotify `state.vscdb` is correct — SQLite write-ahead-log files (`-wal`, `-shm`) make file-level watching unreliable.

Sources: [What Does Cursor Store on Your Machine?](https://vibe-replay.com/blog/cursor-local-storage/), [Cursor Data Storage Model](https://zread.ai/S2thend/cursor-history/7-cursor-data-storage-model)

## Gemini CLI — verified via web search (2026-06-21)

- Confirms draft.md: sessions/chats under `~/.gemini/tmp/<project_hash>/chats/`, project hash derived from the project root path (same idea as Claude Code's path sanitization, different algorithm — needs empirical check once installed).
- New detail: explicit `/chat save` checkpoints are plain JSON files named `checkpoint-<name>.json`; automatic pre-edit checkpoints live under `~/.gemini/tmp/<project_hash>/checkpoints/`. draft.md only mentioned the chats directory.

Sources: [Checkpointing — Gemini CLI](https://geminicli.com/docs/cli/checkpointing/), [Session management — Gemini CLI](https://geminicli.com/docs/cli/session-management/)

## Libraries

| Library in draft.md | Status found | Recommendation |
|---|---|---|
| `LarsArtmann/go-filewatcher` | **Could not find this repository via web search.** Likely a hallucinated/mis-remembered dependency. | Don't depend on it. Roll a ~80-line recursive watcher directly on `fsnotify` (see [Architecture](Architecture.md#recursive-watch)) — fsnotify confirms recursive watching is still not in its public API as of 2026, third-party wrappers are themselves small, so wrapping it ourselves avoids an unverifiable dependency for little code savings. |
| `mattn/go-sqlite3` (implied for Cursor) | Requires cgo → complicates cross-compilation (a real concern since Phase 6 explicitly targets Windows/macOS/Linux). | Use `modernc.org/sqlite` instead — pure Go, no cgo, ~2x slower on inserts but `ctx` is read-only and only touches small KV rows, so the speed cost doesn't matter here. |
| `pkoukk/tiktoken-go` (token counting v2 idea) | Original is unmaintained. Forks exist (`weaviate/tiktoken-go`, `tiktoken-go/tokenizer`). | Not needed for v1 at all — Claude Code already gives exact counts (see above). Defer this whole line item; only revisit if a provider without usage data needs better-than-heuristic estimates. |
| Bubble Tea / Lip Gloss / Bubbles | Active, current, this is still the standard Go TUI stack in 2026. | Keep as planned for Phase 3. No change. |
