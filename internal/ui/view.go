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

var headerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).MarginBottom(1)

func (m Model) View() string {
	if m.statsView {
		return renderStats(m.stats)
	}
	out := m.dashboardChrome() + "\n" + m.table.View()
	if d := m.expandDetail(); d != "" {
		out += "\n" + d
	}
	return out
}

// dashboardChrome is everything rendered above the session table — the header
// block and the provider panels. View and relayout share it so the table is
// sized to exactly fill the rows the chrome leaves, instead of a stale
// constant that ignored the panels and pushed the top off-screen.
func (m Model) dashboardChrome() string {
	header := renderHeader(m.rows, m.showHistory) + "\n" + renderProviderStatus(m.statuses)
	// Pass store.All() separately so the 5h rolling-window count in the
	// panels stays accurate even when the table is filtered to active/idle only.
	return headerStyle.Render(header) + "\n" + renderProviderPanels(m.statuses, m.rows, m.store.All())
}

// expandDetail is the optional per-session detail block shown below the table,
// or "" when nothing is expanded.
func (m Model) expandDetail() string {
	if m.expandedID == "" {
		return ""
	}
	c, ok := expandedContext(m)
	if !ok {
		return ""
	}
	return renderExpandDetail(c, planForProvider(m.statuses, c.Session.Provider))
}

// relayout sizes the table to the terminal: full width, and the height left
// over after the chrome (and any expand detail) so header + panels + table
// together fill exactly m.height and never overflow the alt-screen.
func (m *Model) relayout() {
	if m.width > 0 {
		m.table.SetWidth(m.width)
	}
	used := lipgloss.Height(m.dashboardChrome())
	if d := m.expandDetail(); d != "" {
		used += lipgloss.Height(d)
	}
	h := m.height - used
	if h < 1 {
		h = 1
	}
	m.table.SetHeight(h)
}

func expandedContext(m Model) (provider.SessionContext, bool) {
	for _, r := range m.rows {
		if r.Session.ID == m.expandedID {
			return r, true
		}
	}
	return provider.SessionContext{}, false
}

// planForProvider returns the account plan tier for a provider name (account-
// level data lives on ProviderStatus, not on a single session).
func planForProvider(statuses []engine.ProviderStatus, name string) provider.PlanInfo {
	for _, s := range statuses {
		if s.Name == name {
			return s.Plan
		}
	}
	return provider.PlanInfo{}
}

var detailStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).MarginTop(1)

func renderExpandDetail(c provider.SessionContext, plan provider.PlanInfo) string {
	lines := []string{}

	// Plan line — flag estimated costs right next to the tier so users aren't
	// confused about whether they're seeing a real charge.
	if plan.Known {
		planLine := fmt.Sprintf("plan: %s", format.PrettifyTier(plan.Tier))
		if c.Cost.Known && c.Cost.Source == provider.CostSourceEstimated {
			planLine += "  ·  cost is API-equivalent (not a subscription charge)"
		}
		lines = append(lines, planLine)
	} else if c.Cost.Known && c.Cost.Source == provider.CostSourceEstimated {
		lines = append(lines, "cost shown is API-equivalent, not a real charge")
	}

	// Token breakdown
	lines = append(lines,
		fmt.Sprintf("tokens: in %d / out %d / cache-r %d / cache-c %d",
			c.Tokens.Input, c.Tokens.Output, c.Tokens.CacheRead, c.Tokens.CacheCreation),
	)

	// Session elapsed + burn rate (tok/min and $/hr)
	if !c.Session.StartedAt.IsZero() && !c.Session.LastActive.IsZero() {
		elapsed := c.Session.LastActive.Sub(c.Session.StartedAt)
		if elapsed >= time.Minute && c.Tokens.Total > 0 {
			tokPerMin := float64(c.Tokens.Total) / elapsed.Minutes()
			burnLine := fmt.Sprintf("elapsed: %s  ·  burn: %.0f tok/min", format.FormatDuration(elapsed), tokPerMin)
			if c.Cost.Known && c.Cost.USD > 0 && elapsed.Hours() > 0 {
				burnLine += fmt.Sprintf("  (~$%.3f/hr API-equiv)", c.Cost.USD/elapsed.Hours())
			}
			lines = append(lines, burnLine)
		}
	}

	// Subscription ROI — unique signal: how many times over you've exceeded
	// the cost of your flat-rate subscription. Makes the API-equiv number
	// meaningful instead of alarming.
	if roi := subscriptionROI(c.Cost, plan); roi != "" {
		lines = append(lines, roi)
	}

	lines = append(lines,
		fmt.Sprintf("cwd: %s", format.EmptyDash(c.Session.CWD)),
		fmt.Sprintf("label: %s", format.EmptyDash(c.Session.Label)),
	)
	return detailStyle.Render(strings.Join(lines, "\n"))
}

// subscriptionROI returns a human-readable "≈Nx Plan value" string when we
// have enough signal to make the comparison meaningful: estimated cost, known
// plan, and a cost large enough to be interesting (>$1). Returns "" otherwise.
func subscriptionROI(cost provider.Cost, plan provider.PlanInfo) string {
	if !cost.Known || cost.Source != provider.CostSourceEstimated || !plan.Known || cost.USD < 1 {
		return ""
	}
	var monthly float64
	switch strings.ToLower(strings.ReplaceAll(plan.Tier, " ", "_")) {
	case "pro":
		monthly = 20
	case "max", "max_5x":
		monthly = 100
	case "max_20x":
		monthly = 200
	default:
		return ""
	}
	ratio := cost.USD / monthly
	return fmt.Sprintf("≈%.1f× %s plan value (subscription covers this)", ratio, format.PrettifyTier(plan.Tier))
}

func renderHeader(rows []provider.SessionContext, showHistory bool) string {
	active := 0
	totalCost := 0.0
	hasEstimated := false
	for _, r := range rows {
		if r.Session.Status == provider.StatusActive {
			active++
		}
		if r.Cost.Known {
			totalCost += r.Cost.USD
			if r.Cost.Source == provider.CostSourceEstimated {
				hasEstimated = true
			}
		}
	}
	scope := "active/idle"
	if showHistory {
		scope = "all"
	}
	costStr := fmt.Sprintf("$%.2f total", totalCost)
	if hasEstimated {
		costStr = fmt.Sprintf("~$%.2f API-equiv", totalCost)
	}
	return fmt.Sprintf("ctx — %d sessions (%s) · %d active · %s · h history · s stats · q quit", len(rows), scope, active, costStr)
}

// renderProviderStatus shows every registered provider's detection
// result, intentionally diverging from btop's convention of hiding
// absent hardware — these are installable tools a user can act on, so a
// flagged ✗ with the checked path is directly useful for debugging a
// wrong CODEX_HOME/KIMI_CODE_HOME or a non-default install location.
func renderProviderStatus(statuses []engine.ProviderStatus) string {
	parts := make([]string, 0, len(statuses))
	for _, s := range statuses {
		if s.Detected {
			parts = append(parts, fmt.Sprintf("%s ✓", s.Name))
		} else {
			parts = append(parts, fmt.Sprintf("%s ✗ (%s not found)", s.Name, s.CheckedPath))
		}
	}
	return strings.Join(parts, "   ")
}
