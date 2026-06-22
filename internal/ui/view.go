package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/codebyNJ/minimo/internal/provider"
)

const headerHeight = 2

var headerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).MarginBottom(1)

func (m Model) View() string {
	return headerStyle.Render(renderHeader(m.rows, m.showHistory)) + "\n" + m.table.View()
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
