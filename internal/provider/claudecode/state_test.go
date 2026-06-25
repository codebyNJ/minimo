package claudecode

import "testing"

func TestApplyNewAccumulatesTokenCategories(t *testing.T) {
	line1 := `{"type":"assistant","timestamp":"2026-06-24T10:00:00Z","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":100,"output_tokens":50,"cache_read_input_tokens":10,"cache_creation_input_tokens":5}}}`
	line2 := `{"type":"assistant","timestamp":"2026-06-24T10:01:00Z","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":200,"output_tokens":60,"cache_read_input_tokens":20,"cache_creation_input_tokens":7}}}`
	var s sessionState
	s.applyNew([]byte(line1 + "\n" + line2 + "\n"))

	if s.inputTokens != 300 || s.outputTokens != 110 || s.cacheReadTokens != 30 || s.cacheCreationTokens != 12 {
		t.Fatalf("categories = in:%d out:%d cr:%d cc:%d, want 300/110/30/12",
			s.inputTokens, s.outputTokens, s.cacheReadTokens, s.cacheCreationTokens)
	}
	if s.tokens != 452 {
		t.Fatalf("total tokens = %d, want 452", s.tokens)
	}
}
