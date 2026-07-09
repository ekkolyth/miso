//go:build !windows

package tui

import (
	"io"
	"os"
	"os/exec"

	"github.com/charmbracelet/x/term"
	"github.com/creack/pty"
)

// pty-backed spawn: children see a TTY (color + interactive prompts) with a
// live stdin; the pty master is both output reader and stdin writer
func spawnProcess(cmd *exec.Cmd, rows, cols int) (*spawnResult, error) {
	if rows <= 0 || cols <= 0 {
		if w, h, err := term.GetSize(os.Stdout.Fd()); err == nil && w > 0 && h > 0 {
			cols, rows = w, h
		} else {
			rows, cols = 24, 80
		}
	}
	ws := &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}
	ptmx, err := pty.StartWithSize(cmd, ws)
	if err != nil {
		return nil, err
	}
	return &spawnResult{
		readers: []io.Reader{ptmx},
		stdin:   ptmx,
		closer:  func() { _ = ptmx.Close() },
	}, nil
}
