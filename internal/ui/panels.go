package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/codebyNJ/minimo/internal/engine"
	"github.com/codebyNJ/minimo/internal/provider"
)

const panelWidth = 18

var (
	panelLiveBorder lipgloss.Style
	panelDeadBorder lipgloss.Style
	panelLabel      lipgloss.Style
)

type providerAggregate struct {
	Count     int
	TotalCost float64
	CostKnown bool
	AvgPct    float64
	AvgKnown  bool
}

func aggregateFor(name string, rows []provider.SessionContext) providerAggregate {
	var a providerAggregate
	var pctSum float64
	var pctN int
	for _, r := range rows {
		if r.Session.Provider != name {
			continue
		}
		a.Count++
		if r.Cost.Known {
			a.TotalCost += r.Cost.USD
			a.CostKnown = true
		}
		if r.Context.Known && r.Context.Limit > 0 {
			pctSum += float64(r.Context.Tokens) / float64(r.Context.Limit)
			pctN++
		}
	}
	if pctN > 0 {
		a.AvgPct = pctSum / float64(pctN)
		a.AvgKnown = true
	}
	return a
}

func renderProviderPanels(statuses []engine.ProviderStatus, rows []provider.SessionContext) string {
	panels := make([]string, 0, len(statuses))
	for _, s := range statuses {
		if s.Detected {
			panels = append(panels, livePanel(s, aggregateFor(s.Name, rows)))
		} else {
			panels = append(panels, deadPanel(s))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, panels...)
}

func livePanel(s engine.ProviderStatus, a providerAggregate) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", s.Name)
	fmt.Fprintf(&b, "%s\n", panelLabel.Render(fmt.Sprintf("%d sessions", a.Count)))
	planCost := ""
	if s.Plan.Known {
		planCost = s.Plan.Tier
	}
	if a.CostKnown {
		c := fmt.Sprintf("$%.2f", a.TotalCost)
		if planCost != "" {
			planCost += " · " + c
		} else {
			planCost = c
		}
	}
	if planCost != "" {
		fmt.Fprintf(&b, "%s\n", planCost)
	}
	if a.AvgKnown {
		fmt.Fprintf(&b, "%s", renderMiniBar(a.AvgPct))
	}
	return panelLiveBorder.Render(strings.TrimRight(b.String(), "\n"))
}

func deadPanel(s engine.ProviderStatus) string {
	hint := "not found"
	if s.CheckedPath != "" {
		hint = "not found\n" + panelLabel.Render(s.CheckedPath)
	}
	return panelDeadBorder.Render(s.Name + "\n" + hint)
}

// renderMiniBar draws a fixed 10-cell fullness bar for the average context %.
func renderMiniBar(pct float64) string {
	const w = 10
	filled := int(pct * w)
	if filled > w {
		filled = w
	}
	if filled < 0 {
		filled = 0
	}
	style := barFillLow
	switch {
	case pct >= 0.9:
		style = barFillHigh
	case pct >= 0.7:
		style = barFillMid
	}
	return style.Render(strings.Repeat("█", filled)) + barEmptyStyle.Render(strings.Repeat("░", w-filled))
}
