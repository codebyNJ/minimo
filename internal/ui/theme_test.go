package ui

import "testing"

func TestThemeByNameKnown(t *testing.T) {
	if ThemeByName("default", false).Low != "42" {
		t.Fatal("default theme Low should be color 42")
	}
	if ThemeByName("mono", false).Low == ThemeByName("default", false).Low {
		t.Fatal("mono theme must differ from default")
	}
}

func TestThemeByNameUnknownFallsBackToDefault(t *testing.T) {
	if ThemeByName("nonexistent", false).Low != ThemeByName("default", false).Low {
		t.Fatal("unknown theme name must fall back to default")
	}
}

func TestNoColorBlanksColors(t *testing.T) {
	if ThemeByName("default", true).Low != "" {
		t.Fatal("no-color must blank all colors")
	}
}
