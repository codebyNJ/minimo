# ctx Wiki

`ctx` is a terminal-native dashboard that watches the local session files your AI coding agents (Claude Code, Codex, Cursor, Gemini CLI) leave on disk, and shows you what each one is doing right now — no API keys, no cloud, no AI inside `ctx` itself.

This wiki is the living source of truth for the project. Update it whenever a design decision changes — don't let it drift from the code.

## Pages

- [PRD](PRD.md) — what we're building and why, in plain language (no CS background needed)
- [Architecture](Architecture.md) — components, data flow, gaps found in the original draft, optimization layer, folder structure
- [Research](Research.md) — verified facts about each agent's on-disk format, gathered by reading real session files and checking current library status
- [Decisions](Decisions.md) — open questions and the calls we've made, with reasoning
- [Roadmap](Roadmap.md) — the 6 build phases and what ships in each

## Status

Phase 1 (project skeleton + Claude Code provider) is being planned. See `docs/superpowers/plans/` for the task-by-task implementation plan.
