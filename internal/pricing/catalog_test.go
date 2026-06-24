package pricing

import (
	"math"
	"testing"

	"github.com/codebyNJ/minimo/internal/provider"
)

const sampleJSON = `{
  "sample_spec": {"litellm_provider": "x"},
  "claude-sonnet-4-6": {
    "input_cost_per_token": 0.000003,
    "output_cost_per_token": 0.000015,
    "cache_read_input_token_cost": 0.0000003,
    "cache_creation_input_token_cost": 0.00000375
  },
  "gpt-5": {
    "input_cost_per_token": 0.00000125,
    "output_cost_per_token": 0.00001
  }
}`

func TestParseLiteLLMSkipsNonModels(t *testing.T) {
	cat, err := parseLiteLLM([]byte(sampleJSON))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cat.Lookup("sample_spec"); ok {
		t.Fatal("sample_spec has no input cost and must be skipped")
	}
	e, ok := cat.Lookup("claude-sonnet-4-6")
	if !ok {
		t.Fatal("claude-sonnet-4-6 missing")
	}
	if math.Abs(e.InputPerMTok-3.0) > 1e-9 || math.Abs(e.OutputPerMTok-15.0) > 1e-9 {
		t.Fatalf("rates = in:%v out:%v, want 3.0/15.0 per MTok", e.InputPerMTok, e.OutputPerMTok)
	}
}

func TestLookupNormalizesDateSuffix(t *testing.T) {
	cat, _ := parseLiteLLM([]byte(sampleJSON))
	if _, ok := cat.Lookup("claude-sonnet-4-6-20260514"); !ok {
		t.Fatal("dated model variant should normalize to base key")
	}
}

func TestEstimatePricesCategories(t *testing.T) {
	cat, _ := parseLiteLLM([]byte(sampleJSON))
	u := provider.TokenUsage{Input: 1_000_000, Output: 1_000_000, CacheRead: 1_000_000, CacheCreation: 1_000_000}
	got, ok := cat.Estimate("claude-sonnet-4-6", u)
	if !ok {
		t.Fatal("expected estimate")
	}
	want := 3.0 + 15.0 + 0.3 + 3.75
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("estimate = %v, want %v", got, want)
	}
}

func TestEstimateUnknownModelReturnsFalse(t *testing.T) {
	cat, _ := parseLiteLLM([]byte(sampleJSON))
	if _, ok := cat.Estimate("no-such-model", provider.TokenUsage{Input: 5}); ok {
		t.Fatal("unknown model must return ok=false (no guessing)")
	}
}
