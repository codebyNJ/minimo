package main

import (
	"testing"

	"github.com/codebyNJ/minimo/internal/config"
)

func TestParseArgsVersionAndHelp(t *testing.T) {
	for _, a := range []string{"-V", "--version"} {
		f, err := parseArgs([]string{a})
		if err != nil || !f.version {
			t.Fatalf("%s: version=%v err=%v", a, f.version, err)
		}
	}
	for _, a := range []string{"-h", "--help"} {
		f, err := parseArgs([]string{a})
		if err != nil || !f.help {
			t.Fatalf("%s: help=%v err=%v", a, f.help, err)
		}
	}
}

func TestParseArgsNoArgsIsTUI(t *testing.T) {
	f, err := parseArgs(nil)
	if err != nil || f.subcommand != "" {
		t.Fatalf("no args should be empty subcommand, got %q err=%v", f.subcommand, err)
	}
}

func TestParseArgsStatusWatch(t *testing.T) {
	f, err := parseArgs([]string{"status", "--watch"})
	if err != nil || f.subcommand != "status" || !f.watch {
		t.Fatalf("status --watch: %+v err=%v", f, err)
	}
}

func TestParseArgsStats(t *testing.T) {
	f, err := parseArgs([]string{"stats"})
	if err != nil || f.subcommand != "stats" {
		t.Fatalf("stats: subcommand=%q err=%v, want stats", f.subcommand, err)
	}
	// Global flags still apply to a subcommand.
	f2, err := parseArgs([]string{"stats", "--no-color"})
	if err != nil || f2.subcommand != "stats" || !f2.noColor {
		t.Fatalf("stats --no-color: %+v err=%v", f2, err)
	}
}

func TestParseArgsSubcommandAfterGlobalFlag(t *testing.T) {
	// Regression: a global flag before the subcommand must not hide it.
	// `ctx -c cfg.yaml status` used to fall through and open the TUI.
	f, err := parseArgs([]string{"-c", "cfg.yaml", "status"})
	if err != nil || f.subcommand != "status" || f.config != "cfg.yaml" {
		t.Fatalf("-c cfg.yaml status: %+v err=%v, want subcommand=status config=cfg.yaml", f, err)
	}
	// Flag on either side of the subcommand both work.
	f2, err := parseArgs([]string{"--no-color", "status", "--watch"})
	if err != nil || f2.subcommand != "status" || !f2.noColor || !f2.watch {
		t.Fatalf("--no-color status --watch: %+v err=%v", f2, err)
	}
}

func TestParseArgsBareWordAsFlagValueNotSubcommand(t *testing.T) {
	// A config path that happens to be "status" is a value, not the subcommand.
	f, err := parseArgs([]string{"-c", "status"})
	if err != nil || f.subcommand != "" || f.config != "status" {
		t.Fatalf("-c status: %+v err=%v, want TUI with config=status", f, err)
	}
}

func TestApplyOverrides(t *testing.T) {
	base := config.Default()
	f := cliFlags{update: 5000, provider: "codex"}
	got := applyOverrides(base, f)
	if got.PollIntervalSec != 5 {
		t.Fatalf("update 5000ms → %d sec, want 5", got.PollIntervalSec)
	}
	if len(got.EnabledProviders) != 1 || got.EnabledProviders[0] != "codex" {
		t.Fatalf("provider override = %v, want [codex]", got.EnabledProviders)
	}
}

func TestApplyOverridesNoop(t *testing.T) {
	base := config.Default()
	got := applyOverrides(base, cliFlags{})
	if got.PollIntervalSec != base.PollIntervalSec || len(got.EnabledProviders) != 0 {
		t.Fatalf("empty flags must not change config")
	}
}

func TestExportNoLongerASubcommand(t *testing.T) {
	// "export" must not be treated as a subcommand; only "status" is.
	f, err := parseArgs([]string{"export"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.subcommand == "export" {
		t.Fatal("export must no longer be a recognized subcommand")
	}
}
