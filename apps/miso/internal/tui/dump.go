package tui

import (
	"fmt"
	"os"
	"strings"
)

// DumpLogs prints all buffered process output to stdout, grouped by process.
// Each process section has a header with the label and exit code.
// This is called on TUI exit to preserve logs that would otherwise be lost
// when the alternate screen buffer is discarded.
func DumpLogs(pm *ProcessManager) {
	pm.mu.Lock()
	procs := make([]*Process, len(pm.Processes))
	copy(procs, pm.Processes)
	pm.mu.Unlock()

	if len(procs) == 0 {
		return
	}

	// Find max label width for the separator lines
	maxLabel := 0
	for _, p := range procs {
		if len(p.Entry.Label) > maxLabel {
			maxLabel = len(p.Entry.Label)
		}
	}

	for _, p := range procs {
		lines := p.Buffer.Lines()
		if len(lines) == 0 {
			continue
		}

		// Print a header for each process
		status := "exit 0"
		if p.ExitCode != 0 {
			status = fmt.Sprintf("exit %d", p.ExitCode)
		}
		header := fmt.Sprintf("── %s (%s) ", p.Entry.Label, status)
		remaining := 80 - len(header)
		if remaining > 0 {
			header += strings.Repeat("─", remaining)
		}
		fmt.Fprintln(os.Stdout, header)

		for _, line := range lines {
			fmt.Fprintln(os.Stdout, line)
		}

		fmt.Fprintln(os.Stdout)
	}
}
