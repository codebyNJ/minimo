package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/codebyNJ/minimo/internal/engine"
	"github.com/codebyNJ/minimo/internal/format"
	"github.com/codebyNJ/minimo/internal/provider"
)

const panelWidth = 18

var (
	panelLiveBorder lipgloss.Style
	panelDeadBorder lipgloss.Style
	panelLabel      lipgloss.Style
)

type providerAggregate struct {
	Count         int
	TotalCost     float64
	CostKnown     bool
	Estimated     bool    // true when any session cost is a catalog estimate
	TotalTokens   int     // lifetime sum across all visible sessions
	Tokens5h      int     // tokens in the last 5-hour rolling window (from all sessions)
	ActiveBurnTpm float64 // tok/min summed across currently active sessions
	AvgPct        float64
	AvgKnown      bool
}

// aggregateFor computes metrics for one provider. rows is the visible set
// (used for cost/context/burn); all is the full store (used for the 5h
// rolling-window count so history-off mode doesn't undercount).
func aggregateFor(name string, rows []provider.SessionContext, all []provider.SessionContext) providerAggregate {
	var a providerAggregate
	var pctSum float64
	var pctN int
	for _, r := range rows {
		if r.Session.Provider != name {
			continue
		}
		a.Count++
		a.TotalTokens += r.Tokens.Total
		if r.Cost.Known {
			a.TotalCost += r.Cost.USD
			a.CostKnown = true
			if r.Cost.Source == provider.CostSourceEstimated {
				a.Estimated = true
			}
		}
		if r.Context.Known && r.Context.Limit > 0 {
			pctSum += float64(r.Context.Tokens) / float64(r.Context.Limit)
			pctN++
		}
		if r.Session.Status == provider.StatusActive {
			elapsed := r.Session.LastActive.Sub(r.Session.StartedAt)
			if elapsed >= time.Minute && r.Tokens.Total > 0 {
				a.ActiveBurnTpm += float64(r.Tokens.Total) / elapsed.Minutes()
			}
		}
	}
	if pctN > 0 {
		a.AvgPct = pctSum / float64(pctN)
		a.AvgKnown = true
	}
	// 5h window from ALL sessions so the count is accurate even when the
	// table is filtered to active/idle only.
	cutoff := time.Now().Add(-5 * time.Hour)
	for _, r := range all {
		if r.Session.Provider == name && r.Session.LastActive.After(cutoff) {
			a.Tokens5h += r.Tokens.Total
		}
	}
	return a
}

func renderProviderPanels(statuses []engine.ProviderStatus, rows []provider.SessionContext, all []provider.SessionContext) string {
	panels := make([]string, 0, len(statuses))
	for _, s := range statuses {
		if s.Detected {
			panels = append(panels, livePanel(s, aggregateFor(s.Name, rows, all)))
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

	// Plan tier + cost on one line
	planCost := ""
	if s.Plan.Known {
		planCost = format.PrettifyTier(s.Plan.Tier)
	}
	if a.CostKnown {
		src := provider.CostSourceExact
		if a.Estimated {
			src = provider.CostSourceEstimated
		}
		c := format.FormatCost(provider.Cost{USD: a.TotalCost, Known: true, Source: src})
		if planCost != "" {
			planCost += " · " + c
		} else {
			planCost = c
		}
	}
	if planCost != "" {
		fmt.Fprintf(&b, "%s\n", planCost)
	}

	// Cross-provider efficiency: $/Ktok lets users compare providers at a glance.
	if a.CostKnown && a.TotalTokens > 1000 && a.TotalCost > 0 {
		perKtok := (a.TotalCost / float64(a.TotalTokens)) * 1000
		fmt.Fprintf(&b, "%s\n", panelLabel.Render(fmt.Sprintf("$%.4f/Ktok", perKtok)))
	}

	// 5h rolling window token usage + active burn rate on one line.
	if a.Tokens5h > 0 {
		line := fmt.Sprintf("5h: %s", format.FormatCount(a.Tokens5h))
		if a.ActiveBurnTpm > 0 {
			line += "  " + formatBurnTpm(a.ActiveBurnTpm)
		}
		fmt.Fprintf(&b, "%s\n", panelLabel.Render(line))
	}

	// Average context fill bar
	if a.AvgKnown {
		fmt.Fprintf(&b, "%s", renderMiniBar(a.AvgPct))
	}
	return panelLiveBorder.Render(strings.TrimRight(b.String(), "\n"))
}

// formatBurnTpm formats a token/min rate compactly: "64K/m", "1.2M/m".
func formatBurnTpm(tpm float64) string {
	switch {
	case tpm >= 1_000_000:
		return fmt.Sprintf("%.1fM/m", tpm/1_000_000)
	case tpm >= 1_000:
		return fmt.Sprintf("%.0fK/m", tpm/1_000)
	default:
		return fmt.Sprintf("%.0f/m", tpm)
	}
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
