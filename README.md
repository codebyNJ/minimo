```
╔══════════════════════════════════════════════════════════════════════╗
║  m i n i m o                                                         ║
║  terminal dashboard for AI coding agent sessions                     ║
║  htop/btop — but for context, cost & burn rate                       ║
╚══════════════════════════════════════════════════════════════════════╝
```

> **4 providers. 1 view. Real-time context, cost, and burn rate — no browser required.**

---

## What it does

```
minimo — 8 sessions (active/idle) · 1 active · ~$369.62 API-equiv · h history · s stats · q quit
claude-code ✓   opencode ✓   codex ✗ (~/.codex not found)   kimi-code ✗

┌─ claude-code ──────┐ ┌─ opencode ──────────┐
│ claude-code         │ │ opencode             │
│ 3 sessions          │ │ 5 sessions           │
│ Pro · ~$369.62      │ │ $0.0000              │
│ $0.0005/Ktok        │ │ $0.0000/Ktok         │
│ 5h: 742.5M  73K/m   │ │ 5h: 22.1M            │
│ ████████░░          │ └─────────────────────-┘
└────────────────────┘

  STATUS  PROVIDER     MODEL              CONTEXT               LIFETIME    COST       RATE
  ●       claude-code  claude-sonnet-4-6  [████████░░] 14%/1M   742.5M     ~$369.62   73K/m
  ○       claude-code  claude-opus-4-8    [████░░░░░░]  8%/1M    40.1M     ~$34.94    31K/m
  ○       opencode     deepseek-v4-flash  -                      3.9M      $0.0000    15K/m
```

**Keys:** `↑↓` / `jk` navigate · `Enter` expand session detail · `h` toggle history · `s` usage stats · `q` quit

---

## Install

```bash
# from source (Go 1.22+)
go install github.com/codebyNJ/minimo/cmd/ctx@latest

# or build locally
git clone https://github.com/codebyNJ/minimo
cd minimo
go build -o minimo ./cmd/ctx
```

---

## Commands

```bash
minimo                        # TUI dashboard (default)
minimo status                 # one-shot flat table
minimo status --watch         # flat table, auto-refreshes on file changes
minimo status --json          # machine-readable JSON (pipe to jq, scripts, Grafana)
minimo stats                  # per-model cost & time-utilization for 24h / 7d / 30d
minimo --provider claude-code # restrict to one provider
minimo --no-color             # plain output (for pipes / CI)
minimo --default-config       # print default config YAML
```

### `minimo status`

```
PROVIDER     STATUS   MODEL              LIFETIME   CONTEXT      COST       RATE     LAST       CWD
claude-code  active   claude-sonnet-4-6  742.5M     137K/1.0M    ~$369.62   73K/m    07:36:58   ~/codes/minimo
claude-code  ended    claude-opus-4-8    40.1M      257K/1.0M    ~$34.94    31K/m    14:24:12   ~/codes/project
opencode     ended    deepseek-v4-flash  3.9M       -            $0.0000    15K/m    16:25:30   ~/codes/minimo

~ costs are API-equivalent estimates — subscription users pay a flat monthly rate, not per token
```

### `minimo status --json` (pipe-friendly)

```bash
minimo status --json | jq '.[] | select(.status == "active") | {model, cost_usd, burn_rate_tpm}'
```
```json
{
  "model": "claude-sonnet-4-6",
  "cost_usd": 369.62,
  "burn_rate_tpm": 73000.4
}
```

### `minimo stats`

```
== Today (last 24h) ==
MODEL                 SESS  COST       TOKENS    USED        USE%
claude-sonnet-4-6        1  ~$369.62   742.5M    24h00m    100.0%

== Week (last 7 days) ==
MODEL                 SESS  COST       TOKENS    USED        USE%
claude-sonnet-4-6        3  ~$401.47   791.9M    168h00m   100.0%
claude-opus-4-8          4  ~$224.52   183.4M    88h36m     52.7%
deepseek-v4-flash        3  $0.0000    22.1M     11h19m      6.7%

~ costs are API-equivalent estimates — subscription users pay a flat monthly rate, not per token
```

---

## Cost numbers explained

minimo shows **API-equivalent cost** — what the tokens would cost at pay-as-you-go API rates.

If you use Claude Code via a **Pro ($20/mo) or Max ($100/mo) subscription**, you pay a flat rate.  
The `~$369` figure is the *value* of your usage, not money you owe.

When you expand a session (`Enter`), the detail panel makes this explicit:

```
plan: Pro  ·  cost is API-equivalent (not a subscription charge)
tokens: in 153803 / out 3264262 / cache-r 658062896 / cache-c 21947720
elapsed: 7d00h  ·  burn: 73K tok/min  (~$2.18/hr API-equiv)
≈18.5× Pro plan value (subscription covers this)
```

---

## Providers

| Provider | Detected from | Token source |
|---|---|---|
| **claude-code** | `~/.claude/projects/` JSONL transcripts | exact (per-turn) |
| **opencode** | `~/.local/share/opencode/opencode.db` SQLite | exact |
| **codex** | `~/.codex/` JSONL transcripts | exact |
| **kimi-code** | `$KIMI_CODE_HOME/` JSONL transcripts | exact |

Cost estimation uses the [LiteLLM pricing catalog](https://github.com/BerriAI/litellm) (live fetch with embedded snapshot fallback). Exact costs from providers (OpenCode) are never overwritten.

---

## Config

Config is optional — defaults work out of the box.

```bash
~/.minimo/config.yaml
```

```yaml
debounce_ms: 500
poll_interval_seconds: 2
theme: default                   # default | mono
log_level: ""                    # "" | debug | info
enabled_providers: []            # empty = all providers
provider_paths:                  # override default detection paths
  claude-code: /custom/path/.claude
  codex: /custom/path/.codex
```

```bash
minimo --default-config          # print this with defaults filled in
minimo -c /path/to/config.yaml   # use alternate config
minimo --debug                   # write debug log to ~/.minimo/minimo.log
```

---

## Docker

Runs as a ~10 MB static distroless image. All mounts are read-only.

```bash
# build and run
docker build -t minimo .
docker run --rm -it \
  -v "$HOME/.claude:/home/nonroot/.claude:ro" \
  -v "$HOME/.claude.json:/home/nonroot/.claude.json:ro" \
  -v "$(pwd)/docker-config.yaml:/home/nonroot/.minimo/config.yaml:ro" \
  minimo

# or with Compose (mounts ~/.claude and ~/.codex by default)
docker compose run --rm minimo
```

`docker-config.yaml` maps each provider to its container-side mount path via `provider_paths`. Unmounted providers simply show "not found".

---

## What makes minimo different

| Feature | minimo | tokentop | abtop | ccusage |
|---|---|---|---|---|
| Multi-provider single view | ✓ (4 providers) | ✓ (11) | ✗ (2) | ✗ (1) |
| Per-model 24h/7d/30d stats | ✓ | ✗ | ✗ | partial |
| Subscription ROI line | ✓ | ✗ | ✗ | ✗ |
| Cross-provider $/Ktok | ✓ | ✗ | ✗ | ✗ |
| 5h rolling window display | ✓ | ✗ | ✓ | ✗ |
| `--json` pipe output | ✓ | ✗ | ✗ | ✗ |
| Subagent token attribution | ✓ | ✗ | ✗ | ✗ |
| Zero config, no API keys | ✓ | ✗ | ✓ | ✓ |

---

## Build from source

```bash
git clone https://github.com/codebyNJ/minimo
cd minimo
go build -o minimo ./cmd/ctx          # local binary
go test ./...                          # run tests
```

Requirements: **Go 1.22+**, no CGO, no external dependencies at runtime.
