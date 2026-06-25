package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/codebyNJ/minimo/internal/format"
	"github.com/codebyNJ/minimo/internal/provider"
	"github.com/codebyNJ/minimo/internal/usage"
)

var statsTitle = lipgloss.NewStyle().Bold(true)

// renderStats renders the usage report as one block per window, each with a
// per-model row carrying cost, token volume, used time, and a use-% bar.
// Styling is applied only to whole lines (title, header, empty notice) so it
// never interferes with the fixed-width column math in the data rows.
func renderStats(rep usage.Report) string {
	var b strings.Builder
	fmt.Fprintln(&b, headerStyle.Render("ctx — usage stats · s dashboard · q quit"))
	for _, w := range rep.Windows {
		fmt.Fprintf(&b, "\n%s\n", statsTitle.Render(fmt.Sprintf("%s (%s)", w.Window.Name, w.Window.Label)))
		if len(w.Models) == 0 {
			fmt.Fprintln(&b, panelLabel.Render("  no activity"))
			continue
		}
		header := fmt.Sprintf("%-20s %4s  %-10s %-8s %-8s  %s",
			"MODEL", "SESS", "COST", "TOKENS", "USED", "USE%")
		fmt.Fprintln(&b, panelLabel.Render(header))
		for _, m := range w.Models {
			cost := provider.Cost{USD: m.TotalCost, Known: m.CostKnown}
			if m.Estimated {
				cost.Source = provider.CostSourceEstimated
			}
			// Columns up to USED are fixed-width plain text; the bar (which
			// carries color escapes) and the trailing percent come last, where
			// its escape codes can't throw off earlier columns.
			fmt.Fprintf(&b, "%-20s %4d  %-10s %-8s %-8s  %s %5.1f%%\n",
				format.TruncateRight(m.Model, 20),
				m.Sessions,
				format.FormatCost(cost),
				format.FormatCount(m.Tokens),
				format.FormatDuration(m.UsedTime),
				renderMiniBar(m.UsedFraction),
				m.UsedFraction*100,
			)
		}
	}
	return b.String()
}
