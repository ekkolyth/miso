//go:build !windows

package tui

import (
	"os/exec"
	"syscall"
)

// setProcGroup puts the command in its own process group.
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killGroup sends a signal to the entire process group.
func killGroup(pid int, sig syscall.Signal) error {
	return syscall.Kill(-pid, sig)
}
