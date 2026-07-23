//go:build !windows

package proc

import (
	"os/exec"
	"syscall"
)

func SetGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// process group led by pid.
func KillGroup(pid int, sig syscall.Signal) error {
	return syscall.Kill(-pid, sig)
}
