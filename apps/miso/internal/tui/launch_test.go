package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFolderSpawn_ShellScript(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task.sh")
	if err := os.WriteFile(path, []byte("echo hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd, args, err := folderSpawn(path, "", "")
	if err != nil {
		t.Fatalf("folderSpawn() error: %v", err)
	}
	if cmd != "sh" {
		t.Errorf("cmd = %q, want sh (not the old hardcode path only)", cmd)
	}
	// args must end with the script path and carry -e for the shell.
	if len(args) != 2 || args[0] != "-e" || args[1] != path {
		t.Errorf("args = %v, want [-e %s]", args, path)
	}
}
