//go:build windows

package proc

import (
	"os/exec"
	"strconv"
	"syscall"
)

func SetGroup(cmd *exec.Cmd) {}

func KillGroup(pid int, _ syscall.Signal) error {
	return exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
}
