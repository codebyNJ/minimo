package format

import (
	"testing"

	"github.com/codebyNJ/minimo/internal/provider"
)

func TestPrettifyTier(t *testing.T) {
	cases := map[string]string{
		"max":          "Max",
		"pro":          "Pro",
		"team_premium": "Team Premium",
		"":             "",
	}
	for in, want := range cases {
		if got := PrettifyTier(in); got != want {
			t.Fatalf("PrettifyTier(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatCostMarksEstimated(t *testing.T) {
	exact := FormatCost(provider.Cost{USD: 1.2345, Known: true, Source: provider.CostSourceExact})
	if exact != "$1.2345" {
		t.Fatalf("exact = %q, want $1.2345", exact)
	}
	est := FormatCost(provider.Cost{USD: 1.2345, Known: true, Source: provider.CostSourceEstimated})
	if est != "~$1.2345" {
		t.Fatalf("estimated = %q, want ~$1.2345", est)
	}
	if FormatCost(provider.Cost{Known: false}) != "-" {
		t.Fatal("unknown cost must render -")
	}
}
