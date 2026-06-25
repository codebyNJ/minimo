package ui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Low       lipgloss.Color
	Mid       lipgloss.Color
	High      lipgloss.Color
	Empty     lipgloss.Color
	Header    lipgloss.Color
	DotActive lipgloss.Color
	DotIdle   lipgloss.Color
	DotEnded  lipgloss.Color
	PanelLive lipgloss.Color
	PanelDead lipgloss.Color
	Label     lipgloss.Color
}

var themes = map[string]Theme{
	"default": {
		Low: "42", Mid: "214", High: "203", Empty: "237", Header: "117",
		DotActive: "42", DotIdle: "214", DotEnded: "237",
		PanelLive: "42", PanelDead: "237", Label: "245",
	},
	// mono: 16-color/ASCII-safe for low-color or piped terminals.
	"mono": {
		Low: "7", Mid: "7", High: "15", Empty: "8", Header: "15",
		DotActive: "15", DotIdle: "7", DotEnded: "8",
		PanelLive: "7", PanelDead: "8", Label: "7",
	},
}

// ThemeByName returns the named theme (falling back to "default"), with all
// colors blanked when noColor is set.
func ThemeByName(name string, noColor bool) Theme {
	t, ok := themes[name]
	if !ok {
		t = themes["default"]
	}
	if noColor {
		return Theme{}
	}
	return t
}

var active = themes["default"]

// SetTheme installs the active theme and rebuilds the cached render styles.
func SetTheme(t Theme) {
	active = t
	rebuildStyles()
}
