package engine

import (
	"testing"

	"github.com/codebyNJ/minimo/internal/pricing"
	"github.com/codebyNJ/minimo/internal/provider"
)

func TestEstimatedCostFillsUnknownCost(t *testing.T) {
	cat, _ := pricing.LoadFromBytes([]byte(`{"m1":{"input_cost_per_token":0.000001}}`))
	c := provider.SessionContext{
		Session: provider.SessionInfo{Model: "m1"},
		Tokens:  provider.TokenUsage{Input: 1_000_000},
		Cost:    provider.Cost{Known: false},
	}
	cost, changed := estimatedCost(cat, c)
	if !changed || !cost.Known || cost.Source != provider.CostSourceEstimated {
		t.Fatalf("want estimated known cost, got %+v changed=%v", cost, changed)
	}
	if cost.USD < 0.99 || cost.USD > 1.01 {
		t.Fatalf("cost.USD = %v, want ~1.0", cost.USD)
	}
}

func TestEstimatedCostLeavesExactUntouched(t *testing.T) {
	cat, _ := pricing.LoadFromBytes([]byte(`{"m1":{"input_cost_per_token":0.000001}}`))
	c := provider.SessionContext{
		Session: provider.SessionInfo{Model: "m1"},
		Tokens:  provider.TokenUsage{Input: 1_000_000},
		Cost:    provider.Cost{USD: 5, Known: true, Source: provider.CostSourceExact},
	}
	_, changed := estimatedCost(cat, c)
	if changed {
		t.Fatal("must not overwrite an exact cost")
	}
}
