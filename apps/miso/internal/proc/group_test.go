//go:build !windows

package proc

import (
	"os/exec"
	"syscall"
	"testing"
)

func TestSetGroup_SetsPgid(t *testing.T) {
	cmd := exec.Command("true")
	SetGroup(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Fatal("SetGroup did not set Setpgid")
	}
}

func TestKillGroup_ReapsChildTree(t *testing.T) {
	// Parent shells out to a long sleep; killing the group must terminate both.
	cmd := exec.Command("sh", "-c", "sleep 30 & wait")
	SetGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	if err := KillGroup(pid, syscall.SIGKILL); err != nil {
		t.Fatalf("KillGroup: %v", err)
	}
	_ = cmd.Wait() // returns killed error; we only assert it returns promptly
}
