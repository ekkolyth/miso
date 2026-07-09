//go:build !windows

package tui

import (
	"bufio"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestSpawnProcessPTYEchoesStdin(t *testing.T) {
	cmd := exec.Command("cat")
	res, err := spawnProcess(cmd, 24, 80)
	if err != nil {
		t.Fatalf("spawnProcess: %v", err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		res.closer()
	})
	if len(res.readers) != 1 {
		t.Fatalf("pty path should expose one merged reader, got %d", len(res.readers))
	}
	if _, err := res.stdin.Write([]byte("ping\n")); err != nil {
		t.Fatalf("write stdin: %v", err)
	}

	done := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(res.readers[0])
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "ping") {
				done <- scanner.Text()
				return
			}
		}
		done <- ""
	}()

	select {
	case got := <-done:
		if !strings.Contains(got, "ping") {
			t.Errorf("expected pty to echo 'ping', got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for pty echo")
	}
}
