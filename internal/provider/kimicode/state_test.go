package kimicode

import "testing"

func TestApplyNewSetsTokenCategories(t *testing.T) {
	lines := []wireLine{
		{statusFields: statusFields{TokenUsage: &tokenUsage{
			InputOther: 500, Output: 200, InputCacheRead: 80, InputCacheCreation: 20,
		}}},
	}
	var s sessionState
	s.applyNew(lines)

	if s.inputTokens != 500 || s.outputTokens != 200 || s.cacheReadTokens != 80 || s.cacheCreationTokens != 20 {
		t.Fatalf("categories = in:%d out:%d cr:%d cc:%d, want 500/200/80/20",
			s.inputTokens, s.outputTokens, s.cacheReadTokens, s.cacheCreationTokens)
	}
	if s.tokens != 800 {
		t.Fatalf("total = %d, want 800", s.tokens)
	}
}
