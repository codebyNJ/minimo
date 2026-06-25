//go:build windows

package claudecode

import (
	"os"
	"testing"
)

func TestIsAliveSelfAndDead(t *testing.T) {
	if !isAlive(os.Getpid()) {
		t.Fatal("the test process itself must report alive")
	}
	// PID 0xFFFFFFF0 is effectively never a real process; OpenProcess fails.
	if isAlive(0xFFFFFFF0) {
		t.Fatal("an impossible PID must report not alive")
	}
}
