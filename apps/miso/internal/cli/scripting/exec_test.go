package scripting

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetInterpreterByExtension_TsBun(t *testing.T) {
	if got := getInterpreterByExtension(".ts", "bun"); got != "bun" {
		t.Errorf(".ts with bun = %q, want bun", got)
	}
}

func TestGetInterpreterByExtension_TsNonBun(t *testing.T) {
	for _, mgr := range []string{"npm", "pnpm", "yarn", ""} {
		if got := getInterpreterByExtension(".ts", mgr); got != "node" {
			t.Errorf(".ts with %q = %q, want node", mgr, got)
		}
	}
}

func TestGetInterpreterByExtension_Unchanged(t *testing.T) {
	cases := map[string]string{".sh": "sh", ".py": "python3", ".js": "node", ".rb": "ruby", ".xyz": ""}
	for ext, want := range cases {
		if got := getInterpreterByExtension(ext, "bun"); got != want {
			t.Errorf("%s = %q, want %q", ext, got, want)
		}
	}
}

func writeScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveInterpreter_ShellGetsDashE(t *testing.T) {
	path := writeScript(t, t.TempDir(), "task.sh", "echo hi\n")
	interp, args, err := ResolveInterpreter(path, "", "")
	if err != nil {
		t.Fatalf("ResolveInterpreter() error: %v", err)
	}
	if interp != "sh" || len(args) != 1 || args[0] != "-e" {
		t.Errorf("got (%q, %v), want (sh, [-e])", interp, args)
	}
}

func TestResolveInterpreter_ShebangWins(t *testing.T) {
	// .py extension but a /bin/sh shebang — shebang takes precedence.
	path := writeScript(t, t.TempDir(), "task.py", "#!/bin/sh\necho hi\n")
	interp, _, err := ResolveInterpreter(path, "", "bun")
	if err != nil {
		t.Fatalf("ResolveInterpreter() error: %v", err)
	}
	if interp != "/bin/sh" {
		t.Errorf("interpreter = %q, want /bin/sh (shebang wins)", interp)
	}
}

func TestResolveInterpreter_UnknownExtFallsBackToDefaultShell(t *testing.T) {
	path := writeScript(t, t.TempDir(), "task.xyz", "echo hi\n")
	interp, args, err := ResolveInterpreter(path, "sh", "")
	if err != nil {
		t.Fatalf("ResolveInterpreter() error: %v", err)
	}
	if interp != "sh" || len(args) != 1 || args[0] != "-e" {
		t.Errorf("got (%q, %v), want (sh, [-e])", interp, args)
	}
}

func TestResolveInterpreter_LookPathError(t *testing.T) {
	path := writeScript(t, t.TempDir(), "task.sh", "#!/no/such/interp-xyz\necho hi\n")
	if _, _, err := ResolveInterpreter(path, "", ""); err == nil {
		t.Error("expected LookPath error for missing interpreter")
	}
}

func TestExecScriptFile_RunsScript(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "ran")
	script := writeScript(t, dir, "task.sh", "touch "+marker+"\n")
	if err := ExecScriptFile(script, nil, dir, "sh", "", nil); err != nil {
		t.Fatalf("ExecScriptFile() error: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("script did not run (marker missing): %v", err)
	}
}

func TestExecScriptFile_FailingScriptErrors(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "fail.sh", "exit 3\n")
	if err := ExecScriptFile(script, nil, dir, "sh", "", nil); err == nil {
		t.Error("expected error from failing script")
	}
}
