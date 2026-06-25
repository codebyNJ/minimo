package opencode

import (
	"testing"

	"github.com/codebyNJ/minimo/internal/provider"
)

func TestToTokenUsageMapsCategories(t *testing.T) {
	r := sessionRow{
		tokensInput: 1000, tokensOutput: 300, tokensReasoning: 40,
		tokensCacheRead: 80, tokensCacheWrite: 20,
	}
	u := toTokenUsage(r)
	if u.Input != 1000 || u.Output != 340 || u.CacheRead != 80 || u.CacheCreation != 20 {
		t.Fatalf("got in:%d out:%d cr:%d cc:%d, want 1000/340/80/20",
			u.Input, u.Output, u.CacheRead, u.CacheCreation)
	}
	if u.Total != 1440 {
		t.Fatalf("total = %d, want 1440", u.Total)
	}
	if u.Source != provider.TokenSourceExact {
		t.Fatalf("source = %d, want exact", u.Source)
	}
}

func TestRowCostKnownOnlyWhenPositive(t *testing.T) {
	// A positive stored cost is authoritative and exact.
	if c := rowCost(sessionRow{cost: 1.23}); !c.Known || c.USD != 1.23 || c.Source != provider.CostSourceExact {
		t.Fatalf("positive cost = %+v, want exact known 1.23", c)
	}
	// A zero cost is reported unknown so the engine can estimate it, rather
	// than showing a misleading exact $0.00.
	if c := rowCost(sessionRow{cost: 0, tokensInput: 1_000_000}); c.Known {
		t.Fatalf("zero cost must be unknown, got %+v", c)
	}
}
