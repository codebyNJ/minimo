package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/codebyNJ/minimo/internal/usage"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.relayout()
		return m, nil

	case RefreshMsg:
		m.rows = visibleRows(m.store, m.showHistory)
		m.table.SetRows(rowsToTableRows(m.rows))
		if m.statsView {
			m.stats = usage.Build(m.store.All(), time.Now())
		}
		m.relayout()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "s":
			m.statsView = !m.statsView
			if m.statsView {
				m.stats = usage.Build(m.store.All(), time.Now())
			}
			return m, nil
		case "h":
			m.showHistory = !m.showHistory
			m.rows = visibleRows(m.store, m.showHistory)
			m.table.SetRows(rowsToTableRows(m.rows))
			m.relayout()
			return m, nil
		case "enter":
			m.expandedID = toggleExpand(m.expandedID, selectedSessionID(m))
			m.relayout()
			return m, nil
		}
	}

	// The stats screen is non-interactive; don't feed keys to the table.
	if m.statsView {
		return m, nil
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func toggleExpand(current, selected string) string {
	if selected == "" || current == selected {
		return ""
	}
	return selected
}

// selectedSessionID returns the session ID for the highlighted table row by
// matching the visible row index against m.rows (same order as rowsToTableRows).
func selectedSessionID(m Model) string {
	i := m.table.Cursor()
	if i < 0 || i >= len(m.rows) {
		return ""
	}
	return m.rows[i].Session.ID
}
