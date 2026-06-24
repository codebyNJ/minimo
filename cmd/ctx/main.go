package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/codebyNJ/minimo/internal/config"
	"github.com/codebyNJ/minimo/internal/engine"
	"github.com/codebyNJ/minimo/internal/export"
	"github.com/codebyNJ/minimo/internal/format"
	"github.com/codebyNJ/minimo/internal/pricing"
	"github.com/codebyNJ/minimo/internal/provider"
	_ "github.com/codebyNJ/minimo/internal/provider/claudecode"
	_ "github.com/codebyNJ/minimo/internal/provider/codex"
	"github.com/codebyNJ/minimo/internal/provider/configprovider"
	_ "github.com/codebyNJ/minimo/internal/provider/kimicode"
	_ "github.com/codebyNJ/minimo/internal/provider/opencode"
	"github.com/codebyNJ/minimo/internal/ui"
	"github.com/codebyNJ/minimo/internal/watcher"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: failed to load config:", err)
		os.Exit(1)
	}
	for name, path := range cfg.ProviderPaths {
		provider.SetPathOverride(name, path)
	}
	for _, p := range configprovider.LoadAll(configprovider.DefaultDir()) {
		provider.Register(p)
	}

	f, err := parseArgs(os.Args[1:])
	if err != nil {
		os.Exit(2) // flag package already printed the error
	}
	if f.help {
		printUsage(os.Stdout)
		return
	}
	if f.version {
		fmt.Println("ctx", version)
		return
	}

	catalog := pricing.Load(context.Background())

	switch f.subcommand {
	case "status":
		runStatus(f, cfg, catalog)
	default:
		runTUI(cfg, catalog)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `ctx — terminal context monitor for AI coding agents

Usage:
  ctx [flags]            open the TUI dashboard
  ctx status [--watch]   print a flat session table (optionally re-rendering)

Flags:
  -c, --config <path>    use an alternate config file
  -u, --update <ms>      override the poll interval for this run
      --provider <name>  restrict to one provider (claude-code|opencode|codex|kimi-code)
      --theme <name>     color theme: default or mono
      --no-color         disable colored output
      --debug            write debug logs to ~/.ctx/ctx.log
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

	m := ui.New(e.Store, engine.ProviderStatuses())
	p := tea.NewProgram(m, tea.WithAltScreen())

	watchCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := watchLoop(watchCtx, cfg, func() {
			if err := e.Refresh(); err != nil {
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
	watch := f.watch

	e := engine.New(cfg, catalog)

	if !watch {
		if err := e.Refresh(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		printTable(e)
		return
	}

	runWatch(e, cfg)
}

func runExport(args []string, cfg config.Config, catalog pricing.Catalog) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: ctx export <session-id> [--with-content]")
		os.Exit(1)
	}
	sessionID := args[0]
	withContent := false
	for _, a := range args[1:] {
		if a == "--with-content" {
			withContent = true
		}
	}

	e := engine.New(cfg, catalog)
	if err := e.Refresh(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	ctx, ok := e.Store.Get(sessionID)
	if !ok {
		fmt.Fprintf(os.Stderr, "error: unknown session %q\n", sessionID)
		os.Exit(1)
	}

	out := export.Build(ctx, withContent)
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
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

func printTable(e *engine.Engine) {
	rows := e.Store.All()
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Session.LastActive.After(rows[j].Session.LastActive)
	})

	fmt.Printf("%-12s %-8s %-18s %-10s %-12s %-9s %-10s %-24s %s\n",
		"PROVIDER", "STATUS", "MODEL", "LIFETIME", "CONTEXT", "COST", "LAST", "CWD", "LABEL")
	for _, r := range rows {
		if r.Session.LastActive.IsZero() {
			continue
		}
		fmt.Printf("%-12s %-8s %-18s %-10d %-12s %-9s %-10s %-24s %s\n",
			r.Session.Provider,
			r.Session.Status,
			format.EmptyDash(format.TruncateRight(r.Session.Model, 18)),
			r.Tokens.Total,
			format.FormatContext(r.Context),
			format.FormatCost(r.Cost),
			r.Session.LastActive.Format("15:04:05"),
			format.Truncate(r.Session.CWD, 24),
			r.Session.Label,
		)
	}
}
