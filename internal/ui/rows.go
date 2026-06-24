package ui

import (
	"sort"

	"github.com/charmbracelet/bubbles/table"

	"github.com/codebyNJ/minimo/internal/engine"
	"github.com/codebyNJ/minimo/internal/format"
	"github.com/codebyNJ/minimo/internal/provider"
)

func visibleRows(store *engine.StateStore, showHistory bool) []provider.SessionContext {
	all := store.All()
	var out []provider.SessionContext
	for _, r := range all {
		if r.Session.LastActive.IsZero() {
			continue
		}
		if !showHistory && r.Session.Status == provider.StatusEnded {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Session.LastActive.After(out[j].Session.LastActive)
	})
	return out
}

func rowsToTableRows(rows []provider.SessionContext) []table.Row {
	out := make([]table.Row, 0, len(rows))
	for _, r := range rows {
		out = append(out, table.Row{
			statusDot(r.Session.Status),
			r.Session.Provider,
			format.EmptyDash(format.TruncateRight(r.Session.Model, 18)),
			renderContextBar(r.Context),
			format.FormatCount(r.Tokens.Total),
			format.FormatCost(r.Cost),
			r.Session.LastActive.Format("15:04:05"),
		})
	}
	return out
}

func tableColumns() []table.Column {
	return []table.Column{
		{Title: "", Width: 1},
		{Title: "PROVIDER", Width: 12},
		{Title: "MODEL", Width: 18},
		{Title: "CONTEXT", Width: 32},
		{Title: "LIFETIME", Width: 10},
		{Title: "COST", Width: 9},
		{Title: "LAST", Width: 10},
	}
}

func tableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true).Foreground(active.Header)
	s.Selected = s.Selected.Background(active.Empty)
	return s
}
