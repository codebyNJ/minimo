package pricing

import (
	"context"
	"testing"
)

func TestEmbeddedSnapshotParses(t *testing.T) {
	cat, err := parseLiteLLM(snapshotBytes)
	if err != nil {
		t.Fatalf("embedded snapshot failed to parse: %v", err)
	}
	if len(cat.entries) == 0 {
		t.Fatal("embedded snapshot has no model entries")
	}
	// gpt-4o is a long-stable, always-present LiteLLM key.
	if _, ok := cat.Lookup("gpt-4o"); !ok {
		t.Fatal("expected gpt-4o in embedded snapshot")
	}
	// gpt-4o's context window must come through as a real number, proving the
	// max_input_tokens parse (which the catalog also encodes as a string for
	// its schema placeholder) survives the real snapshot.
	if w, ok := cat.ContextWindow("gpt-4o"); !ok || w <= 0 {
		t.Fatalf("gpt-4o context window = (%d, %v), want a positive value", w, ok)
	}
}

func TestLoadFallsBackToSnapshotOnCanceledContext(t *testing.T) {
	// A canceled context makes the live fetch fail immediately; Load must
	// still return a usable catalog from the embedded snapshot.
	ctx, cancel := newCanceledContext()
	defer cancel()
	cat := Load(ctx)
	if len(cat.entries) == 0 {
		t.Fatal("Load returned empty catalog on fetch failure")
	}
}

func newCanceledContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx, cancel
}
