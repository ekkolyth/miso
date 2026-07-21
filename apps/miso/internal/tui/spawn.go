package tui

import (
	"io"
	"os/exec"

	"github.com/ekkolyth/miso/internal/proc"
)

// pipe-backed spawn for plain mode: the child gets pipes instead of a pty, so a
// tool detects a non-tty and self-formats to line output rather than emitting
// cursor-driven redraws. SetGroup makes the child its own group so Stop's
// KillGroup reaps the tree; Wait closes the pipes on exit, so closer is a no-op.
// Mirrors the Windows spawnProcess wiring.
func spawnPipes(cmd *exec.Cmd) (*spawnResult, error) {
	proc.SetGroup(cmd)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &spawnResult{
		readers: []io.Reader{stdout, stderr},
		stdin:   stdin,
		resize:  func(int, int) {},
		closer:  func() {},
	}, nil
}
