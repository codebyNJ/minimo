package ui

import (
	"strings"
	"testing"

	"github.com/codebyNJ/minimo/internal/engine"
	"github.com/codebyNJ/minimo/internal/provider"
)

func ctxWith(prov string, usd float64, known bool, tok, limit int, ctxKnown bool) provider.SessionContext {
	return provider.SessionContext{
		Session: provider.SessionInfo{Provider: prov},
		Cost:    provider.Cost{USD: usd, Known: known},
		Context: provider.ContextUsage{Tokens: tok, Limit: limit, Known: ctxKnown},
	}
}

func TestAggregateForSumsCostAndCounts(t *testing.T) {
	rows := []provider.SessionContext{
		ctxWith("claude-code", 0.50, true, 500, 1000, true),
		ctxWith("claude-code", 0.30, true, 0, 0, false),
		ctxWith("opencode", 1.00, true, 0, 0, false),
	}
	a := aggregateFor("claude-code", rows, rows)
	if a.Count != 2 {
		t.Fatalf("count = %d, want 2", a.Count)
	}
	if !a.CostKnown || a.TotalCost < 0.79 || a.TotalCost > 0.81 {
		t.Fatalf("total cost = %v known=%v, want ~0.80 true", a.TotalCost, a.CostKnown)
	}
	if !a.AvgKnown || a.AvgPct < 0.49 || a.AvgPct > 0.51 {
		t.Fatalf("avg pct = %v known=%v, want ~0.50 true", a.AvgPct, a.AvgKnown)
	}
}

func TestAggregateForNoContextKnown(t *testing.T) {
	rows := []provider.SessionContext{ctxWith("codex", 0, false, 0, 0, false)}
	a := aggregateFor("codex", rows, rows)
	if a.AvgKnown {
		t.Fatal("avg must be unknown when no session has known context")
	}
}

func TestRenderProviderPanelsIncludesNames(t *testing.T) {
	statuses := []engine.ProviderStatus{
		{Name: "claude-code", Detected: true},
		{Name: "codex", Detected: false, CheckedPath: "/home/u/.codex"},
	}
	out := renderProviderPanels(statuses, nil, nil)
	if !strings.Contains(out, "claude-code") || !strings.Contains(out, "codex") {
		t.Fatal("panel output must name every provider")
	}
	if !strings.Contains(out, "not found") {
		t.Fatal("undetected provider must show 'not found'")
	}
}
