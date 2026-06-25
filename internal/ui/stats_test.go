package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/codebyNJ/minimo/internal/usage"
)

func TestRenderStatsIncludesWindowsModelsAndPercent(t *testing.T) {
	rep := usage.Report{Windows: []usage.WindowReport{
		{
			Window: usage.Window{Name: "Today", Label: "last 24h", Duration: 24 * time.Hour},
			Models: []usage.ModelStat{
				{Model: "claude-opus-4-8", Sessions: 2, TotalCost: 12.5, CostKnown: true, Estimated: true, Tokens: 1_500_000, UsedTime: 90 * time.Minute, UsedFraction: 0.0625},
			},
		},
		{
			Window: usage.Window{Name: "Week", Label: "last 7 days", Duration: 7 * 24 * time.Hour},
			Models: nil,
		},
	}}
	out := renderStats(rep)

	for _, want := range []string{
		"Today (last 24h)", // window heading
		"claude-opus-4-8",  // model
		"~$12.50",          // estimated cost, 2-decimal
		"1.5M",             // token volume
		"1h30m",            // used time
		"6.2%",             // use fraction
		"Week (last 7 days)",
		"no activity", // empty window notice
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("renderStats output missing %q.\n---\n%s", want, out)
		}
	}
}
