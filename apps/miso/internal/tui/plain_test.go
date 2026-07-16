package tui

import (
	"bytes"
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
