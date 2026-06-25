package watcher

import (
	"fmt"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

// TestHandleTimerMapDoesNotLeak schedules debounce timers for many distinct
// paths and confirms that once they fire the timer map is emptied again,
// rather than retaining one dead entry per path forever.
func TestHandleTimerMapDoesNotLeak(t *testing.T) {
	w, err := New(5 * time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	const n = 20
	for i := 0; i < n; i++ {
		w.handle(fsnotify.Event{Name: fmt.Sprintf("/tmp/session-%d.jsonl", i), Op: fsnotify.Write})
	}

	// Each fired timer deletes its own entry just before sending on Events, so
	// once we have drained all n events every delete has run.
	deadline := time.After(2 * time.Second)
	for got := 0; got < n; got++ {
		select {
		case <-w.Events:
		case <-deadline:
			t.Fatalf("only %d/%d debounce events fired before timeout", got, n)
		}
	}

	if pending := w.pendingTimers(); pending != 0 {
		t.Fatalf("timer map leaked: %d entries remain after all timers fired, want 0", pending)
	}
}

// TestHandleCoalescesRapidWritesToSamePath confirms repeated writes to one
// path collapse to a single pending timer (debounce), not one per write.
func TestHandleCoalescesRapidWritesToSamePath(t *testing.T) {
	w, err := New(50 * time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	for i := 0; i < 5; i++ {
		w.handle(fsnotify.Event{Name: "/tmp/same.jsonl", Op: fsnotify.Write})
	}
	if pending := w.pendingTimers(); pending != 1 {
		t.Fatalf("rapid writes to one path = %d pending timers, want 1", pending)
	}

	select {
	case <-w.Events:
	case <-time.After(2 * time.Second):
		t.Fatal("debounced event never fired")
	}
	if pending := w.pendingTimers(); pending != 0 {
		t.Fatalf("timer map not cleared after fire: %d remain", pending)
	}
}
