package main

import "testing"

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
