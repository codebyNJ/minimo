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

type fakeProv struct{ name string }

func (f fakeProv) Name() string                                         { return f.name }
func (f fakeProv) Detect() bool                                         { return false }
func (f fakeProv) ListSessions() ([]provider.SessionInfo, error)        { return nil, nil }
func (f fakeProv) ReadContext(string) (*provider.SessionContext, error) { return nil, nil }

func TestFilterEnabledRestrictsProviders(t *testing.T) {
	all := []provider.Provider{fakeProv{"claude-code"}, fakeProv{"opencode"}, fakeProv{"codex"}}
	if got := filterEnabled(all, nil); len(got) != 3 {
		t.Fatalf("empty enabled = %d providers, want all 3", len(got))
	}
	got := filterEnabled(all, []string{"claude-code"})
	if len(got) != 1 || got[0].Name() != "claude-code" {
		t.Fatalf("enabled [claude-code] = %v, want just claude-code", got)
	}
}

func TestStoreRetainEvictsOnlyEnumeratedAndUnseen(t *testing.T) {
	s := NewStateStore()
	s.Put("keep", provider.SessionContext{Session: provider.SessionInfo{ID: "keep", Provider: "claude-code"}})
	s.Put("gone", provider.SessionContext{Session: provider.SessionInfo{ID: "gone", Provider: "claude-code"}})
	s.Put("other", provider.SessionContext{Session: provider.SessionInfo{ID: "other", Provider: "opencode"}})

	// claude-code was enumerated this cycle and only "keep" was seen; opencode
	// was NOT enumerated (e.g. transient error), so "other" must survive.
	s.Retain(map[string]bool{"keep": true}, map[string]bool{"claude-code": true})

	if _, ok := s.Get("keep"); !ok {
		t.Fatal("seen session must be retained")
	}
	if _, ok := s.Get("gone"); ok {
		t.Fatal("unseen session from an enumerated provider must be evicted")
	}
	if _, ok := s.Get("other"); !ok {
		t.Fatal("session from a non-enumerated provider must be kept (no flicker on transient errors)")
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
