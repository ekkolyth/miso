package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ekkolyth/miso/internal/config"
)

func TestFilterEntriesByWorkspace(t *testing.T) {
	entries := []TuiScriptEntry{
		{Label: "web", WorkspaceName: "@ekko/web"},
		{Label: "api", WorkspaceName: "@ekko/api"},
	}
	got := filterEntriesByWorkspace(entries, []string{"@ekko/web"})
	if len(got) != 1 || got[0].WorkspaceName != "@ekko/web" {
		t.Errorf("got %v, want only @ekko/web", got)
	}
	if len(filterEntriesByWorkspace(entries, nil)) != 2 {
		t.Error("nil filter keeps all entries")
	}
}

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

// TestBuildRunRootScriptArgsReachScript is the regression fence for the Task 8
// arg-drop: a root single-script run must thread its invocation args through to
// the spawned script instead of dropping them.
func TestBuildRunRootScriptArgsReachScript(t *testing.T) {
	root := t.TempDir()
	scriptsDir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "echo-args.sh"), []byte("echo \"got:[$@]\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{Scripts: "./scripts"}
	pm, levels, concurrentProcs, ran, err := buildRun(cfg, "echo-args", root, nil, nil, []string{"staging"})
	if err != nil {
		t.Fatalf("buildRun: %v", err)
	}
	if !ran {
		t.Fatal("buildRun returned not-applicable")
	}

	var buf bytes.Buffer
	if _, err := RunPlain(pm, &buf, levels, concurrentProcs); err != nil {
		t.Fatalf("RunPlain: %v", err)
	}

	if out := buf.String(); !strings.Contains(out, "got:[staging]") {
		t.Errorf("script args did not reach the script; output:\n%s", out)
	}
}

// TestBuildRunConcurrentCompanionDoesNotReceiveArgs verifies that when a root
// script has a concurrent companion, invocation args reach only the main entry
// — the companion always spawns arg-less.
func TestBuildRunConcurrentCompanionDoesNotReceiveArgs(t *testing.T) {
	root := t.TempDir()
	scriptsDir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "dev.sh"), []byte("echo \"main:[$@]\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "worker.sh"), []byte("echo \"conc:[$@]\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Scripts: "./scripts",
		Tasks:   map[string]config.TaskConfig{"dev": {Concurrent: []string{"worker"}}},
	}
	pm, levels, concurrentProcs, ran, err := buildRun(cfg, "dev", root, nil, nil, []string{"staging"})
	if err != nil {
		t.Fatalf("buildRun: %v", err)
	}
	if !ran {
		t.Fatal("buildRun returned not-applicable")
	}

	var buf bytes.Buffer
	if _, err := RunPlain(pm, &buf, levels, concurrentProcs); err != nil {
		t.Fatalf("RunPlain: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "main:[staging]") {
		t.Errorf("main entry did not receive args; output:\n%s", out)
	}
	if !strings.Contains(out, "conc:[]") {
		t.Errorf("concurrent companion did not run arg-less; output:\n%s", out)
	}
	if strings.Contains(out, "conc:[staging]") {
		t.Errorf("concurrent companion leaked the main entry's args; output:\n%s", out)
	}
}

// TestBuildRunMemberFanOutDropsArgs verifies that when a script has no root
// resolution and fans out to 2+ members, invocation args are dropped rather
// than broadcast to every member — which member they'd apply to is ambiguous.
func TestBuildRunMemberFanOutDropsArgs(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"workspaces":["apps/web","apps/api"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, member := range []string{"web", "api"} {
		scriptsDir := filepath.Join(root, "apps", member, "scripts")
		if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		content := "echo \"" + member + ":[$@]\"\n"
		if err := os.WriteFile(filepath.Join(scriptsDir, "dev.sh"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.Config{Scripts: "./scripts"}
	pm, levels, concurrentProcs, ran, err := buildRun(cfg, "dev", root, nil, nil, []string{"staging"})
	if err != nil {
		t.Fatalf("buildRun: %v", err)
	}
	if !ran {
		t.Fatal("buildRun returned not-applicable")
	}

	var buf bytes.Buffer
	if _, err := RunPlain(pm, &buf, levels, concurrentProcs); err != nil {
		t.Fatalf("RunPlain: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "web:[]") || !strings.Contains(out, "api:[]") {
		t.Errorf("expected both fan-out members to run arg-less; output:\n%s", out)
	}
	if strings.Contains(out, "staging") {
		t.Errorf("fan-out entries unexpectedly received args; output:\n%s", out)
	}
}
