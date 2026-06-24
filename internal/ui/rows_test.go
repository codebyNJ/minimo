package ui

import (
	"testing"

	"github.com/codebyNJ/minimo/internal/provider"
)

func TestTableHasSevenColumns(t *testing.T) {
	if got := len(tableColumns()); got != 7 {
		t.Fatalf("columns = %d, want 7 (CWD/LABEL dropped)", got)
	}
}

func TestRowsHaveSevenCells(t *testing.T) {
	rows := []provider.SessionContext{{Session: provider.SessionInfo{Provider: "opencode"}}}
	got := rowsToTableRows(rows)
	if len(got) != 1 || len(got[0]) != 7 {
		t.Fatalf("row cells = %d, want 7", len(got[0]))
	}
}
