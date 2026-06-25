package claudecode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/codebyNJ/minimo/internal/tailreader"
)

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

func TestApplySubagentTokensFoldsUsageOnly(t *testing.T) {
	var s sessionState
	// Main assistant turn.
	s.applyNew([]byte(`{"type":"assistant","message":{"model":"claude-opus-4-8","usage":{"input_tokens":100,"output_tokens":50}}}` + "\n"))
	// Subagent turn adds token volume but must not touch model/context.
	s.applySubagentTokens([]byte(`{"type":"assistant","message":{"model":"ignored","usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":3,"cache_creation_input_tokens":2}}}` + "\n"))

	if s.tokens != 100+50+10+5+3+2 {
		t.Fatalf("tokens = %d, want 170 (main + subagent)", s.tokens)
	}
	if s.inputTokens != 110 || s.outputTokens != 55 || s.cacheReadTokens != 3 || s.cacheCreationTokens != 2 {
		t.Fatalf("categories wrong: in=%d out=%d cr=%d cc=%d", s.inputTokens, s.outputTokens, s.cacheReadTokens, s.cacheCreationTokens)
	}
	if s.model != "claude-opus-4-8" {
		t.Fatalf("subagent must not change parent model, got %q", s.model)
	}
}

func TestApplySubagentsReadsSubagentDir(t *testing.T) {
	dir := t.TempDir()
	sid := "sess-1"
	subDir := filepath.Join(dir, sid, "subagents")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "agent-a.jsonl"),
		[]byte(`{"type":"assistant","message":{"usage":{"input_tokens":1000,"output_tokens":200}}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := New()
	state := &sessionState{id: sid, cursor: tailreader.Cursor{Path: filepath.Join(dir, sid+".jsonl")}}
	p.sessions[sid] = state

	p.applySubagents(state)
	if state.tokens != 1200 || state.inputTokens != 1000 || state.outputTokens != 200 {
		t.Fatalf("subagent fold from disk wrong: tokens=%d in=%d out=%d", state.tokens, state.inputTokens, state.outputTokens)
	}

	// Second call with no new data must not double-count (cursor is incremental).
	p.applySubagents(state)
	if state.tokens != 1200 {
		t.Fatalf("re-read double-counted: tokens=%d, want 1200", state.tokens)
	}
}
