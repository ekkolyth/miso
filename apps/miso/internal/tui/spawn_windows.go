//go:build windows

package tui

import (
	"io"
	"os/exec"

	"github.com/ekkolyth/miso/internal/proc"
)

// Windows has no creack/pty support (ConPTY differs), so children get pipes.
// proc.SetGroup mirrors the direct-exec teardown model; proc.KillGroup
// (taskkill /T) reaps the tree on Stop.
func spawnProcess(cmd *exec.Cmd, _, _ int) (*spawnResult, error) {
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
		closer:  func() {},
	}, nil
}
