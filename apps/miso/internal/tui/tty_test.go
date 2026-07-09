package tui

import "testing"

func TestHasInteractiveTTY(t *testing.T) {
	orig := isTerminal
	t.Cleanup(func() { isTerminal = orig })

	isTerminal = func(uintptr) bool { return true }
	if !hasInteractiveTTY() {
		t.Error("expected true when both fds are terminals")
	}

	isTerminal = func(uintptr) bool { return false }
	if hasInteractiveTTY() {
		t.Error("expected false when fds are not terminals")
	}
}
