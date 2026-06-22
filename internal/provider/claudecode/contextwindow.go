package claudecode

// Verified 2026-06-22 against Anthropic's published API context-window
// docs. Claude Code is an API-driven CLI, so the API tier's window size
// applies — not the lower limit some platforms impose on the claude.ai
// web-chat product. Only models actually observed in real session data on
// this machine are listed; anything else returns 0 (unknown), which the
// CLI renders as a raw token count with no percentage rather than risk a
// wrong denominator.
var contextWindowSizes = map[string]int{
	"claude-sonnet-4-6": 1_000_000,
	"claude-opus-4-8":   1_000_000,
}

func contextLimitFor(model string) int {
	return contextWindowSizes[model]
}
