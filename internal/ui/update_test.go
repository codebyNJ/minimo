package ui

import (
	"strings"
	"testing"

	"github.com/codebyNJ/minimo/internal/engine"
	"github.com/codebyNJ/minimo/internal/provider"
)

func TestToggleExpand(t *testing.T) {
	if got := toggleExpand("", "abc"); got != "abc" {
		t.Fatalf("collapsed→expand: got %q, want abc", got)
	}
	if got := toggleExpand("abc", "abc"); got != "" {
		t.Fatalf("same row toggles off: got %q, want empty", got)
	}
	if got := toggleExpand("abc", "xyz"); got != "xyz" {
		t.Fatalf("switch row: got %q, want xyz", got)
	}
}

func TestRenderExpandDetailShowsBreakdownAndPlan(t *testing.T) {
	c := provider.SessionContext{
		Session: provider.SessionInfo{CWD: "/home/u/proj", Label: "my task"},
		Tokens:  provider.TokenUsage{Input: 100, Output: 50, CacheRead: 10, CacheCreation: 5},
	}
	out := renderExpandDetail(c, provider.PlanInfo{Tier: "Max", Known: true})
	for _, want := range []string{"/home/u/proj", "my task", "100", "50", "Max"} {
		if !strings.Contains(out, want) {
			t.Fatalf("detail missing %q in: %s", want, out)
		}
	}
}

func TestPlanForProvider(t *testing.T) {
	statuses := []engine.ProviderStatus{
		{Name: "claude-code", Plan: provider.PlanInfo{Tier: "Max", Known: true}},
		{Name: "codex"},
	}
	if p := planForProvider(statuses, "claude-code"); !p.Known || p.Tier != "Max" {
		t.Fatalf("got %+v, want {Max true}", p)
	}
	if planForProvider(statuses, "codex").Known {
		t.Fatal("codex has no plan; want Known=false")
	}
	if planForProvider(statuses, "missing").Known {
		t.Fatal("unknown provider must yield Known=false")
	}
}
