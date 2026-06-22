package ui

import (
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/codebyNJ/minimo/internal/engine"
	"github.com/codebyNJ/minimo/internal/provider"
)

type RefreshMsg struct{}

type Model struct {
	store       *engine.StateStore
	table       table.Model
	rows        []provider.SessionContext
	showHistory bool
}

func New(store *engine.StateStore) Model {
	t := table.New(
		table.WithColumns(tableColumns()),
		table.WithFocused(true),
	)
	t.SetStyles(tableStyles())
	return Model{store: store, table: t}
}

func (m Model) Init() tea.Cmd {
	return nil
}
