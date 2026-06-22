# ctx

Terminal dashboard for AI coding agent sessions (Claude Code, OpenCode) — htop/btop, but for context usage and cost.

## Build

```bash
go build -o bin/ctx ./cmd/ctx
```

## Run

```bash
./bin/ctx
```

Opens the TUI. Keys: arrow keys/`j`/`k` to scroll, `h` to toggle full history (active/idle sessions only by default), `q` or `Ctrl+C` to quit.

## Other commands

```bash
./bin/ctx status            # one-shot flat table, no TUI
./bin/ctx status --watch    # flat table, auto-refreshes
./bin/ctx export <session-id>                 # export a session
./bin/ctx export <session-id> --with-content  # include file contents
```

## Config (optional)

No config needed to try it — defaults work out of the box. To customize, create `~/.ctx/config.yaml`:

```yaml
debounce_ms: 500
poll_interval_seconds: 2
enabled_providers: []   # empty = all providers enabled
```

## Testing it right now

1. Build with the command above.
2. Run `./bin/ctx status` first — confirms it can find your Claude Code/OpenCode session data and prints a flat table.
3. Run `./bin/ctx` to launch the TUI and confirm the same sessions render with context bars, cost, and live updates while you use an agent in another terminal.
