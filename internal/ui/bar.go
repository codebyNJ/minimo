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
	barFillLow     lipgloss.Style
	barFillMid     lipgloss.Style
	barFillHigh    lipgloss.Style
	barEmptyStyle  lipgloss.Style
	dotActiveStyle lipgloss.Style
	dotIdleStyle   lipgloss.Style
	dotEndedStyle  lipgloss.Style
)

func init() { rebuildStyles() }

func rebuildStyles() {
	barFillLow = lipgloss.NewStyle().Foreground(active.Low)
	barFillMid = lipgloss.NewStyle().Foreground(active.Mid)
	barFillHigh = lipgloss.NewStyle().Foreground(active.High)
	barEmptyStyle = lipgloss.NewStyle().Foreground(active.Empty)
	dotActiveStyle = lipgloss.NewStyle().Foreground(active.DotActive)
	dotIdleStyle = lipgloss.NewStyle().Foreground(active.DotIdle)
	dotEndedStyle = lipgloss.NewStyle().Foreground(active.DotEnded)

	panelLiveBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(active.PanelLive).
		Width(panelWidth).Padding(0, 1)
	panelDeadBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(active.PanelDead).
		Foreground(active.PanelDead).
		Width(panelWidth).Padding(0, 1)
	panelLabel = lipgloss.NewStyle().Foreground(active.Label)
}

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
