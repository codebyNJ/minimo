package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.table.SetWidth(msg.Width)
		m.table.SetHeight(msg.Height - headerHeight)
		return m, nil

	case RefreshMsg:
		m.rows = visibleRows(m.store, m.showHistory)
		m.table.SetRows(rowsToTableRows(m.rows))
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "h":
			m.showHistory = !m.showHistory
			m.rows = visibleRows(m.store, m.showHistory)
			m.table.SetRows(rowsToTableRows(m.rows))
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}
