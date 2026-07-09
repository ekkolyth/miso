package tui

import (
	"os"

	"github.com/charmbracelet/x/term"
)

// isTerminal is a seam so tests can stub TTY detection.
var isTerminal = term.IsTerminal

// TUI needs a real terminal for the alt screen; absent one (CI, pipes, docker
// without -t, agents) the caller falls through to plain execution.
func hasInteractiveTTY() bool {
	return isTerminal(os.Stdout.Fd()) && isTerminal(os.Stdin.Fd())
}
