package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/codebyNJ/minimo/internal/config"
	"github.com/codebyNJ/minimo/internal/engine"
	"github.com/codebyNJ/minimo/internal/export"
	"github.com/codebyNJ/minimo/internal/provider"
	_ "github.com/codebyNJ/minimo/internal/provider/claudecode"
	"github.com/codebyNJ/minimo/internal/provider/configprovider"
	_ "github.com/codebyNJ/minimo/internal/provider/opencode"
	"github.com/codebyNJ/minimo/internal/watcher"
)

func main() {
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: failed to load config:", err)
		os.Exit(1)
	}
	for _, p := range configprovider.LoadAll(configprovider.DefaultDir()) {
		provider.Register(p)
	}

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: ctx status [--watch] | ctx export <session-id> [--with-content]")
		os.Exit(1)
	}
	switch os.Args[1] {
	case "status":
		runStatus(os.Args[2:], cfg)
	case "export":
		runExport(os.Args[2:], cfg)
	default:
		fmt.Fprintln(os.Stderr, "usage: ctx status [--watch] | ctx export <session-id> [--with-content]")
		os.Exit(1)
	}
}

func runStatus(args []string, cfg config.Config) {
	watch := false
	for _, a := range args {
		if a == "--watch" {
			watch = true
		}
	}

	e := engine.New(cfg)

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

func runExport(args []string, cfg config.Config) {
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

	e := engine.New(cfg)
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

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	projectsDir := filepath.Join(home, ".claude", "projects")

	w, err := watcher.New(cfg.Debounce())
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer w.Close()
	if err := w.AddRecursive(projectsDir); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	go w.Run(ctx)

	ticker := time.NewTicker(cfg.PollInterval())
	defer ticker.Stop()

	refresh := func() {
		if err := e.Refresh(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		fmt.Print("\033[H\033[2J")
		printTable(e)
	}

	refresh()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.Events:
			refresh()
		case <-ticker.C:
			refresh()
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
			emptyDash(truncateRight(r.Session.Model, 18)),
			r.Tokens.Total,
			formatContext(r.Context),
			formatCost(r.Cost),
			r.Session.LastActive.Format("15:04:05"),
			truncate(r.Session.CWD, 24),
			r.Session.Label,
		)
	}
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func formatCount(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		// 999,500–999,999 would round up to "1000K"; promote to "1.0M".
		if float64(n)/1_000 >= 999.5 {
			return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
		}
		return fmt.Sprintf("%.0fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func formatContext(c provider.ContextUsage) string {
	if !c.Known {
		return "-"
	}
	if c.Limit > 0 {
		return fmt.Sprintf("%s/%s", formatCount(c.Tokens), formatCount(c.Limit))
	}
	return formatCount(c.Tokens)
}

func formatCost(c provider.Cost) string {
	if !c.Known {
		return "-"
	}
	return fmt.Sprintf("$%.4f", c.USD)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n+3:]
}

func truncateRight(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
