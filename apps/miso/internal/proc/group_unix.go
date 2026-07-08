//go:build !windows

package proc

import (
	"os/exec"
	"syscall"
)

// SetGroup puts the command in its own process group.
func SetGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// KillGroup signals the entire process group led by pid.
func KillGroup(pid int, sig syscall.Signal) error {
	return syscall.Kill(-pid, sig)
}
