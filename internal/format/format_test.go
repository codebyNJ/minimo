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
	// Estimated costs gain a ~ prefix.
	est := FormatCost(provider.Cost{USD: 405.4710, Known: true, Source: provider.CostSourceEstimated})
	if est != "~$405.47" {
		t.Fatalf("estimated = %q, want ~$405.47", est)
	}
	exact := FormatCost(provider.Cost{USD: 12.5, Known: true, Source: provider.CostSourceExact})
	if exact != "$12.50" {
		t.Fatalf("exact = %q, want $12.50", exact)
	}
	if FormatCost(provider.Cost{Known: false}) != "-" {
		t.Fatal("unknown cost must render -")
	}
}

func TestFormatCostAdaptivePrecision(t *testing.T) {
	// >= $1 renders at 2 decimals (readable, fits the column); sub-dollar
	// costs keep 4 decimals so small figures stay meaningful.
	cases := map[float64]string{
		405.4710: "$405.47",
		1.0:      "$1.00",
		0.9999:   "$0.9999",
		0.0034:   "$0.0034",
		0.0:      "$0.0000",
	}
	for usd, want := range cases {
		got := FormatCost(provider.Cost{USD: usd, Known: true, Source: provider.CostSourceExact})
		if got != want {
			t.Fatalf("FormatCost(%v) = %q, want %q", usd, got, want)
		}
	}
}
