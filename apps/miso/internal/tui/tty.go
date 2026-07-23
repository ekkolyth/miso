package tui

import (
	"os"

	"github.com/charmbracelet/x/term"
)

// seam for stubbing TTY detection in tests
var isTerminal = term.IsTerminal

// TUI needs a real terminal for the alt screen; absent one (CI, pipes, docker
// without -t, agents) the caller falls through to plain execution.
func hasInteractiveTTY() bool {
	return isTerminal(os.Stdout.Fd()) && isTerminal(os.Stdin.Fd())
}

// exported so cmd/main can reach the package-private check
func InteractiveTTY() bool {
	return hasInteractiveTTY()
}
