//go:build !windows

package proc

import (
	"bufio"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestSetGroup_SetsPgid(t *testing.T) {
	cmd := exec.Command("true")
	SetGroup(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Fatal("SetGroup did not set Setpgid")
	}
}

func TestKillGroup_ReapsChildTree(t *testing.T) {
	// Parent shell backgrounds a grandchild sleep and prints its pid, then
	// waits. Killing the group must terminate the grandchild too, not just
	// the shell — that's the distinction a plain cmd.Wait() can't catch.
	cmd := exec.Command("sh", "-c", "sleep 30 & echo $!; wait")
	SetGroup(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Wait() }()

	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() {
		t.Fatalf("failed to read grandchild pid: %v", scanner.Err())
	}
	grandchildPid, err := strconv.Atoi(scanner.Text())
	if err != nil {
		t.Fatalf("failed to parse grandchild pid: %v", err)
	}

	pid := cmd.Process.Pid
	if err := KillGroup(pid, syscall.SIGKILL); err != nil {
		t.Fatalf("KillGroup: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if signalErr := syscall.Kill(grandchildPid, 0); signalErr == syscall.ESRCH {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("grandchild pid %d still alive after KillGroup", grandchildPid)
}
