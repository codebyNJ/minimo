package format

import (
	"fmt"
	"strings"

	"github.com/codebyNJ/minimo/internal/provider"
)

func EmptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func FormatCount(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		// 999,500–999,999 would round up to "1000K"; promote to "1.0M".
		if float64(n)/1_000 >= 999.5 {
			return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
		}
		return fmt.Sprintf("%.0fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func FormatContext(c provider.ContextUsage) string {
	if !c.Known {
		return "-"
	}
	if c.Limit > 0 {
		return fmt.Sprintf("%s/%s", FormatCount(c.Tokens), FormatCount(c.Limit))
	}
	return FormatCount(c.Tokens)
}

func FormatCost(c provider.Cost) string {
	if !c.Known {
		return "-"
	}
	return fmt.Sprintf("$%.4f", c.USD)
}

func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n+3:]
}

func TruncateRight(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// PrettifyTier turns a raw tier token ("team_premium") into a display string
// ("Team Premium"): underscores to spaces, each word capitalized.
func PrettifyTier(s string) string {
	parts := strings.Split(strings.ReplaceAll(s, "_", " "), " ")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
