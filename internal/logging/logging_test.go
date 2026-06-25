package logging

import "testing"

func TestParseLevel(t *testing.T) {
	cases := map[string]Level{"off": LevelOff, "error": LevelError, "info": LevelInfo, "debug": LevelDebug, "": LevelOff, "bogus": LevelOff}
	for in, want := range cases {
		if got := ParseLevel(in); got != want {
			t.Fatalf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestInitUnopenablePathFallsBackToDiscard(t *testing.T) {
	// A path under a nonexistent directory cannot be opened; Init must not
	// panic and logging calls must be safe no-ops.
	Init(LevelDebug, "/this/does/not/exist/ctx.log")
	Debugf("safe %d", 1) // must not panic
}
