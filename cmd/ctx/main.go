package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/codebyNJ/minimo/internal/engine"
	_ "github.com/codebyNJ/minimo/internal/provider/claudecode"
	"github.com/codebyNJ/minimo/internal/watcher"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "status" {
		fmt.Fprintln(os.Stderr, "usage: ctx status [--watch]")
		os.Exit(1)
	}
	runStatus(os.Args[2:])
}

func runStatus(args []string) {
	watch := false
	for _, a := range args {
		if a == "--watch" {
			watch = true
		}
	}

	e := engine.New()

	if !watch {
		if err := e.Refresh(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		printTable(e)
		return
	}

	runWatch(e)
}

func runWatch(e *engine.Engine) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	projectsDir := filepath.Join(home, ".claude", "projects")

	w, err := watcher.New(engine.DebounceDefault)
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
		}
	}
}

func printTable(e *engine.Engine) {
	rows := e.Store.All()
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Session.LastActive.After(rows[j].Session.LastActive)
	})

	fmt.Printf("%-12s %-8s %-8s %-10s %-24s %s\n", "PROVIDER", "STATUS", "TOKENS", "LAST", "CWD", "LABEL")
	for _, r := range rows {
		fmt.Printf("%-12s %-8s %-8d %-10s %-24s %s\n",
			r.Session.Provider,
			r.Session.Status,
			r.Tokens.Total,
			r.Session.LastActive.Format("15:04:05"),
			truncate(r.Session.CWD, 24),
			r.Session.Label,
		)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n+3:]
}
