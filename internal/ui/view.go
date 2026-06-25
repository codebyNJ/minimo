package ui

import (
	"fmt"
	"strings"

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
	return headerStyle.Render(header) + "\n" + renderProviderPanels(m.statuses, m.rows)
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
	if plan.Known {
		lines = append(lines, fmt.Sprintf("plan: %s", plan.Tier))
	}
	lines = append(lines,
		fmt.Sprintf("tokens: in %d / out %d / cache-r %d / cache-c %d",
			c.Tokens.Input, c.Tokens.Output, c.Tokens.CacheRead, c.Tokens.CacheCreation),
		fmt.Sprintf("cwd: %s", format.EmptyDash(c.Session.CWD)),
		fmt.Sprintf("label: %s", format.EmptyDash(c.Session.Label)),
	)
	return detailStyle.Render(strings.Join(lines, "\n"))
}

func renderHeader(rows []provider.SessionContext, showHistory bool) string {
	active := 0
	totalCost := 0.0
	for _, r := range rows {
		if r.Session.Status == provider.StatusActive {
			active++
		}
		if r.Cost.Known {
			totalCost += r.Cost.USD
		}
	}
	scope := "active/idle"
	if showHistory {
		scope = "all"
	}
	return fmt.Sprintf("ctx — %d sessions (%s) · %d active · $%.2f total · h history · s stats · q quit", len(rows), scope, active, totalCost)
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
