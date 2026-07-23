package tui

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPlainStreamsPrefixedOutput(t *testing.T) {
	dir := t.TempDir()
	scriptA := filepath.Join(dir, "a.sh")
	scriptB := filepath.Join(dir, "b.sh")
	if err := os.WriteFile(scriptA, []byte("echo alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scriptB, []byte("echo beta\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pm := NewProcessManager()
	for _, s := range []struct{ label, path string }{{"a", scriptA}, {"b", scriptB}} {
		cmd, args, err := folderSpawn(s.path, "", "")
		if err != nil {
			t.Fatalf("folderSpawn: %v", err)
		}
		pm.Add(TuiScriptEntry{Label: s.label, ScriptSource: "folder", ScriptPath: s.path}, cmd, args, dir, nil)
	}

	var buf bytes.Buffer
	ok, err := RunPlain(pm, &buf, nil, nil)
	if err != nil {
		t.Fatalf("RunPlain: %v", err)
	}
	if !ok {
		t.Fatal("RunPlain returned not-applicable")
	}
	out := buf.String()
	if !strings.Contains(out, "[a] alpha") {
		t.Errorf("missing [a] alpha in:\n%s", out)
	}
	if !strings.Contains(out, "[b] beta") {
		t.Errorf("missing [b] beta in:\n%s", out)
	}
}

// addPlainScript writes a shell script that runs body and registers it on pm.
func addPlainScript(t *testing.T, pm *ProcessManager, dir, label, body string) {
	t.Helper()
	path := filepath.Join(dir, label+".sh")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd, args, err := folderSpawn(path, "", "")
	if err != nil {
		t.Fatalf("folderSpawn: %v", err)
	}
	pm.Add(TuiScriptEntry{Label: label, ScriptSource: "folder", ScriptPath: path}, cmd, args, dir, nil)
}

// TestRunPlainReturnsErrorOnChildFailure fences the CI/agent exit-code gap: a
// non-zero child must surface a non-nil error so main.go exits non-zero.
func TestRunPlainReturnsErrorOnChildFailure(t *testing.T) {
	dir := t.TempDir()
	pm := NewProcessManager()
	addPlainScript(t, pm, dir, "ok", "exit 0\n")
	addPlainScript(t, pm, dir, "bad", "exit 1\n")

	ran, err := RunPlain(pm, io.Discard, nil, nil)
	if !ran {
		t.Fatal("RunPlain returned not-applicable")
	}
	if err == nil {
		t.Fatal("expected non-nil error when a child exits non-zero")
	}
	if got := pm.FailedCount(); got != 1 {
		t.Errorf("FailedCount = %d, want 1", got)
	}
}

// TestRunPlainReturnsNilOnAllSuccess verifies an all-zero run stays clean.
func TestRunPlainReturnsNilOnAllSuccess(t *testing.T) {
	dir := t.TempDir()
	pm := NewProcessManager()
	addPlainScript(t, pm, dir, "a", "exit 0\n")
	addPlainScript(t, pm, dir, "b", "exit 0\n")

	ran, err := RunPlain(pm, io.Discard, nil, nil)
	if !ran {
		t.Fatal("RunPlain returned not-applicable")
	}
	if err != nil {
		t.Fatalf("RunPlain: unexpected error %v", err)
	}
	if got := pm.FailedCount(); got != 0 {
		t.Errorf("FailedCount = %d, want 0", got)
	}
}
