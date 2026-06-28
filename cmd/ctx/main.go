package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/codebyNJ/minimo/internal/config"
	"github.com/codebyNJ/minimo/internal/engine"
	"github.com/codebyNJ/minimo/internal/format"
	"github.com/codebyNJ/minimo/internal/logging"
	"github.com/codebyNJ/minimo/internal/pricing"
	"github.com/codebyNJ/minimo/internal/provider"
	_ "github.com/codebyNJ/minimo/internal/provider/claudecode"
	_ "github.com/codebyNJ/minimo/internal/provider/codex"
	"github.com/codebyNJ/minimo/internal/provider/configprovider"
	_ "github.com/codebyNJ/minimo/internal/provider/kimicode"
	_ "github.com/codebyNJ/minimo/internal/provider/opencode"
	"github.com/codebyNJ/minimo/internal/ui"
	"github.com/codebyNJ/minimo/internal/usage"
	"github.com/codebyNJ/minimo/internal/watcher"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

// applyOverrides layers one-run CLI flag values over the loaded config.
func applyOverrides(cfg config.Config, f cliFlags) config.Config {
	if f.update > 0 {
		cfg.PollIntervalSec = f.update / 1000
		if cfg.PollIntervalSec < 1 {
			cfg.PollIntervalSec = 1
		}
	}
	if f.provider != "" {
		cfg.EnabledProviders = []string{f.provider}
	}
	return cfg
}

func main() {
	f, err := parseArgs(os.Args[1:])
	if err != nil {
		os.Exit(2) // flag package already printed the error
	}

	if f.defaultConfig {
		data, err := config.DefaultYAML()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		os.Stdout.Write(data)
		return
	}

	if f.help {
		printUsage(os.Stdout)
		return
	}

	if f.version {
		fmt.Println("minimo", version)
		return
	}

	configPath := config.DefaultPath()
	if f.config != "" {
		configPath = f.config
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: failed to load config:", err)
		os.Exit(1)
	}

	for name, path := range cfg.ProviderPaths {
		provider.SetPathOverride(name, path)
	}
	cfg = applyOverrides(cfg, f)

	themeName := cfg.Theme
	if f.theme != "" {
		themeName = f.theme
	}
	ui.SetTheme(ui.ThemeByName(themeName, f.noColor))

	logLevel := logging.ParseLevel(cfg.LogLevel)
	if f.debug {
		logLevel = logging.LevelDebug
	}
	logPath := ""
	if home, err := os.UserHomeDir(); err == nil {
		logPath = filepath.Join(home, ".minimo", "minimo.log")
	}
	logging.Init(logLevel, logPath)

	for _, p := range configprovider.LoadAll(configprovider.DefaultDir()) {
		provider.Register(p)
	}

	catalog := pricing.Load(context.Background())

	switch f.subcommand {
	case "status":
		runStatus(f, cfg, catalog)
	case "stats":
		runStats(cfg, catalog)
	default:
		runTUI(cfg, catalog)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `minimo — terminal dashboard for AI coding agent sessions

Usage:
  minimo [flags]                    open the TUI dashboard
  minimo status [--watch] [--json]  print a flat session table
  minimo stats                      print per-model cost & time usage for 24h/7d/30d

Flags:
  -c, --config <path>    use an alternate config file
  -u, --update <ms>      override the poll interval for this run
      --provider <name>  restrict to one provider (claude-code|opencode|codex|kimi-code)
      --theme <name>     color theme: default or mono
      --no-color         disable colored output
      --debug            write debug logs to ~/.minimo/minimo.log
      --default-config   print the default config YAML and exit
  -V, --version          print version and exit
  -h, --help             show this help and exit`)
}

func runTUI(cfg config.Config, catalog pricing.Catalog) {
	e := engine.New(cfg, catalog)
	if err := e.Refresh(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	m := ui.New(e.Store, engine.ProviderStatuses(cfg.EnabledProviders))
	p := tea.NewProgram(m, tea.WithAltScreen())

	watchCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := watchLoop(watchCtx, cfg, func() {
			if err := e.Refresh(); err != nil {
				logging.Debugf("tui refresh failed, showing prior data: %v", err)
				return
			}
			p.Send(ui.RefreshMsg{})
		}); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		p.Quit()
	}()

	_, err := p.Run()
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runStatus(f cliFlags, cfg config.Config, catalog pricing.Catalog) {
	e := engine.New(cfg, catalog)

	if !f.watch {
		if err := e.Refresh(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		if f.json {
			printTableJSON(e)
		} else {
			printTable(e)
		}
		return
	}

	runWatch(e, cfg)
}

func runStats(cfg config.Config, catalog pricing.Catalog) {
	e := engine.New(cfg, catalog)
	if err := e.Refresh(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	printStats(usage.Build(e.Store.All(), time.Now()))
}

// printStats renders the per-model usage report as one flat section per window.
func printStats(rep usage.Report) {
	hasEstimated := false
	for _, w := range rep.Windows {
		fmt.Printf("\n== %s (%s) ==\n", w.Window.Name, w.Window.Label)
		if len(w.Models) == 0 {
			fmt.Println("  (no activity)")
			continue
		}
		fmt.Printf("%-20s %5s  %-10s %-9s %-9s %6s\n",
			"MODEL", "SESS", "COST", "TOKENS", "USED", "USE%")
		for _, m := range w.Models {
			cost := provider.Cost{USD: m.TotalCost, Known: m.CostKnown}
			if m.Estimated {
				cost.Source = provider.CostSourceEstimated
				hasEstimated = true
			}
			fmt.Printf("%-20s %5d  %-10s %-9s %-9s %5.1f%%\n",
				format.TruncateRight(m.Model, 20),
				m.Sessions,
				format.FormatCost(cost),
				format.FormatCount(m.Tokens),
				format.FormatDuration(m.UsedTime),
				m.UsedFraction*100,
			)
		}
	}
	if hasEstimated {
		fmt.Println("\n~ costs are API-equivalent estimates — subscription users pay a flat monthly rate, not per token")
	}
}

func runWatch(e *engine.Engine, cfg config.Config) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	refresh := func() {
		if err := e.Refresh(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		fmt.Print("\033[H\033[2J")
		printTable(e)
	}

	if err := watchLoop(ctx, cfg, refresh); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func watchLoop(ctx context.Context, cfg config.Config, onTrigger func()) error {
	w, err := watcher.New(cfg.Debounce())
	if err != nil {
		return err
	}
	defer w.Close()

	for _, p := range provider.All() {
		if !p.Detect() {
			continue
		}
		wp, ok := p.(provider.Watchable)
		if !ok {
			continue
		}
		for _, path := range wp.WatchPaths() {
			if err := w.AddRecursive(path); err != nil {
				continue
			}
		}
	}
	go w.Run(ctx)

	ticker := time.NewTicker(cfg.PollInterval())
	defer ticker.Stop()

	onTrigger()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-w.Events:
			onTrigger()
		case <-ticker.C:
			onTrigger()
		}
	}
}

// sessionJSON is the shape emitted by ctx status --json. Field names are
// snake_case so the output is easy to consume with jq and shell scripts.
type sessionJSON struct {
	Provider          string  `json:"provider"`
	ID                string  `json:"id"`
	Status            string  `json:"status"`
	Model             string  `json:"model"`
	TokensTotal       int     `json:"tokens_total"`
	TokensInput       int     `json:"tokens_input"`
	TokensOutput      int     `json:"tokens_output"`
	TokensCacheRead   int     `json:"tokens_cache_read"`
	TokensCacheCreate int     `json:"tokens_cache_create"`
	CostUSD           float64 `json:"cost_usd"`
	CostKnown         bool    `json:"cost_known"`
	CostEstimated     bool    `json:"cost_estimated"`
	ContextTokens     int     `json:"context_tokens"`
	ContextLimit      int     `json:"context_limit"`
	BurnRateTpm       float64 `json:"burn_rate_tpm,omitempty"` // tok/min; 0 when unknown
	StartedAt         string  `json:"started_at,omitempty"`
	LastActive        string  `json:"last_active"`
	CWD               string  `json:"cwd"`
	Label             string  `json:"label"`
}

func printTableJSON(e *engine.Engine) {
	rows := e.Store.All()
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Session.LastActive.After(rows[j].Session.LastActive)
	})
	out := make([]sessionJSON, 0, len(rows))
	for _, r := range rows {
		if r.Session.LastActive.IsZero() {
			continue
		}
		var burnTpm float64
		if !r.Session.StartedAt.IsZero() {
			elapsed := r.Session.LastActive.Sub(r.Session.StartedAt)
			if elapsed >= time.Minute && r.Tokens.Total > 0 {
				burnTpm = float64(r.Tokens.Total) / elapsed.Minutes()
			}
		}
		startedAt := ""
		if !r.Session.StartedAt.IsZero() {
			startedAt = r.Session.StartedAt.UTC().Format(time.RFC3339)
		}
		out = append(out, sessionJSON{
			Provider:          r.Session.Provider,
			ID:                r.Session.ID,
			Status:            string(r.Session.Status),
			Model:             r.Session.Model,
			TokensTotal:       r.Tokens.Total,
			TokensInput:       r.Tokens.Input,
			TokensOutput:      r.Tokens.Output,
			TokensCacheRead:   r.Tokens.CacheRead,
			TokensCacheCreate: r.Tokens.CacheCreation,
			CostUSD:           r.Cost.USD,
			CostKnown:         r.Cost.Known,
			CostEstimated:     r.Cost.Source == provider.CostSourceEstimated,
			ContextTokens:     r.Context.Tokens,
			ContextLimit:      r.Context.Limit,
			BurnRateTpm:       burnTpm,
			StartedAt:         startedAt,
			LastActive:        r.Session.LastActive.UTC().Format(time.RFC3339),
			CWD:               r.Session.CWD,
			Label:             r.Session.Label,
		})
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, "error encoding JSON:", err)
		os.Exit(1)
	}
}

func printTable(e *engine.Engine) {
	rows := e.Store.All()
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Session.LastActive.After(rows[j].Session.LastActive)
	})

	fmt.Printf("%-12s %-8s %-18s %-10s %-12s %-9s %-8s %-10s %-24s %s\n",
		"PROVIDER", "STATUS", "MODEL", "LIFETIME", "CONTEXT", "COST", "RATE", "LAST", "CWD", "LABEL")
	hasEstimated := false
	for _, r := range rows {
		if r.Session.LastActive.IsZero() {
			continue
		}
		if r.Cost.Known && r.Cost.Source == provider.CostSourceEstimated {
			hasEstimated = true
		}
		burnRate := "-"
		if !r.Session.StartedAt.IsZero() {
			elapsed := r.Session.LastActive.Sub(r.Session.StartedAt)
			if elapsed >= time.Minute && r.Tokens.Total > 0 {
				tpm := float64(r.Tokens.Total) / elapsed.Minutes()
				if tpm >= 1000 {
					burnRate = fmt.Sprintf("%.0fK/m", tpm/1000)
				} else {
					burnRate = fmt.Sprintf("%.0f/m", tpm)
				}
			}
		}
		fmt.Printf("%-12s %-8s %-18s %-10s %-12s %-9s %-8s %-10s %-24s %s\n",
			r.Session.Provider,
			r.Session.Status,
			format.EmptyDash(format.TruncateRight(r.Session.Model, 18)),
			format.FormatCount(r.Tokens.Total),
			format.FormatContext(r.Context),
			format.FormatCost(r.Cost),
			burnRate,
			r.Session.LastActive.Format("15:04:05"),
			format.Truncate(r.Session.CWD, 24),
			r.Session.Label,
		)
	}
	if hasEstimated {
		fmt.Println("\n~ costs are API-equivalent estimates — subscription users pay a flat monthly rate, not per token")
	}
}
