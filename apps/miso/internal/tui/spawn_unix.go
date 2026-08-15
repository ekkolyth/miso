//go:build !windows

package tui

import (
	"io"
	"os"
	"os/exec"
	"syscall"

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
	ptmx, tty, err := pty.Open()
	if err != nil {
		return nil, err
	}
	if err := pty.Setsize(ptmx, ws); err != nil {
		_ = ptmx.Close()
		_ = tty.Close()
		return nil, err
	}
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
	cmd.SysProcAttr.Setctty = true
	if err := cmd.Start(); err != nil {
		_ = ptmx.Close()
		_ = tty.Close()
		return nil, err
	}
	return &spawnResult{
		readers: []io.Reader{ptmx},
		stdin:   ptmx,
		resize: func(rows, cols int) {
			if rows > 0 && cols > 0 {
				_ = pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
			}
		},
		releaseAfterWait: func() { _ = tty.Close() },
		closer: func() {
			_ = tty.Close()
			_ = ptmx.Close()
		},
	}, nil
}
