package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/codebyNJ/minimo/internal/format"
	"github.com/codebyNJ/minimo/internal/provider"
)

const barWidth = 20

var (
	barFillLow    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	barFillMid    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	barFillHigh   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	barEmptyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))

	dotActiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	dotIdleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	dotEndedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))
)

func renderContextBar(c provider.ContextUsage) string {
	if !c.Known {
		return "-"
	}
	if c.Limit <= 0 {
		return format.FormatCount(c.Tokens)
	}

	pct := float64(c.Tokens) / float64(c.Limit)
	filled := int(pct * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
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

	bar := style.Render(strings.Repeat("█", filled)) + barEmptyStyle.Render(strings.Repeat("░", barWidth-filled))
	return fmt.Sprintf("[%s] %s/%s", bar, format.FormatCount(c.Tokens), format.FormatCount(c.Limit))
}

func statusDot(s provider.SessionStatus) string {
	switch s {
	case provider.StatusActive:
		return dotActiveStyle.Render("●")
	case provider.StatusIdle:
		return dotIdleStyle.Render("○")
	default:
		return dotEndedStyle.Render("○")
	}
}
