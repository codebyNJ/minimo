package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/codebyNJ/minimo/internal/engine"
	"github.com/codebyNJ/minimo/internal/provider"
)

const headerHeight = 3

var headerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).MarginBottom(1)

func (m Model) View() string {
	header := renderHeader(m.rows, m.showHistory) + "\n" + renderProviderStatus(m.statuses)
	return headerStyle.Render(header) + "\n" + m.table.View()
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
	return fmt.Sprintf("ctx — %d sessions (%s) · %d active · $%.4f total · q quit · h history", len(rows), scope, active, totalCost)
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
