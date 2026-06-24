package codex

import "testing"

func TestApplyNewSetsTokenCategoriesFromTotal(t *testing.T) {
	line := `{"timestamp":"2026-06-24T10:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":200,"output_tokens":300,"reasoning_output_tokens":40,"total_tokens":1540},"last_token_usage":{"input_tokens":10,"cached_input_tokens":2,"output_tokens":3,"reasoning_output_tokens":0,"total_tokens":15},"model_context_window":272000}}}`
	var s sessionState
	s.applyNew([]byte(line + "\n"))

	if s.inputTokens != 1000 || s.cacheReadTokens != 200 || s.outputTokens != 340 {
		t.Fatalf("categories = in:%d cr:%d out:%d, want 1000/200/340 (output incl. reasoning)",
			s.inputTokens, s.cacheReadTokens, s.outputTokens)
	}
	if s.cacheCreationTokens != 0 {
		t.Fatalf("codex has no cache-creation; want 0, got %d", s.cacheCreationTokens)
	}
	if s.tokens != 1540 {
		t.Fatalf("total = %d, want 1540", s.tokens)
	}
}
