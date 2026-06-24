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

func TestBackfillContextLimitFromCatalog(t *testing.T) {
	cat, _ := pricing.LoadFromBytes([]byte(`{"m1":{"input_cost_per_token":0.000001,"max_input_tokens":200000}}`))
	c := provider.SessionContext{
		Session: provider.SessionInfo{Model: "m1"},
		Context: provider.ContextUsage{Tokens: 1000, Known: true, Limit: 0},
	}
	backfillContextLimit(cat, &c)
	if c.Context.Limit != 200000 {
		t.Fatalf("limit = %d, want 200000 (backfilled from catalog)", c.Context.Limit)
	}
}

func TestBackfillContextLimitLeavesProviderLimit(t *testing.T) {
	cat, _ := pricing.LoadFromBytes([]byte(`{"m1":{"input_cost_per_token":0.000001,"max_input_tokens":200000}}`))
	c := provider.SessionContext{
		Session: provider.SessionInfo{Model: "m1"},
		Context: provider.ContextUsage{Tokens: 1000, Known: true, Limit: 999},
	}
	backfillContextLimit(cat, &c)
	if c.Context.Limit != 999 {
		t.Fatalf("limit = %d, want 999 — a provider-supplied limit must win", c.Context.Limit)
	}
}

func TestBackfillContextLimitSkipsUnknownContextAndModel(t *testing.T) {
	cat, _ := pricing.LoadFromBytes([]byte(`{"m1":{"input_cost_per_token":0.000001,"max_input_tokens":200000}}`))
	// Unknown context usage stays untouched even if the model has a window.
	unknown := provider.SessionContext{
		Session: provider.SessionInfo{Model: "m1"},
		Context: provider.ContextUsage{Known: false},
	}
	backfillContextLimit(cat, &unknown)
	if unknown.Context.Limit != 0 || unknown.Context.Known {
		t.Fatalf("unknown context must stay unknown with 0 limit, got %+v", unknown.Context)
	}
	// Known context with a model the catalog doesn't have gets no denominator.
	noModel := provider.SessionContext{
		Session: provider.SessionInfo{Model: "mystery"},
		Context: provider.ContextUsage{Tokens: 1000, Known: true, Limit: 0},
	}
	backfillContextLimit(cat, &noModel)
	if noModel.Context.Limit != 0 {
		t.Fatalf("unknown model must not get a guessed limit, got %d", noModel.Context.Limit)
	}
}
