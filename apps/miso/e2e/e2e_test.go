package e2e

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var misoBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "miso-e2e")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	misoBin = filepath.Join(dir, "miso")
	if os.PathSeparator == '\\' {
		misoBin += ".exe"
	}

	build := exec.Command("go", "build", "-o", misoBin, "./cmd")
	build.Dir = ".." // apps/miso
	if out, err := build.CombinedOutput(); err != nil {
		panic("build miso: " + err.Error() + "\n" + string(out))
	}

	os.Exit(m.Run())
}

// run execs the built binary in workdir, returning combined output + exit code.
func run(t *testing.T, workdir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(misoBin, args...)
	cmd.Dir = workdir
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("run %v: %v", args, err)
		}
	}
	return string(out), code
}

func TestE2E_Version(t *testing.T) {
	out, code := run(t, ".", "version")
	if code != 0 {
		t.Fatalf("version exit = %d, want 0 (out: %s)", code, out)
	}
	if !strings.Contains(out, "miso") {
		t.Errorf("version output %q, want to contain 'miso'", out)
	}
}

func TestE2E_EnvPass(t *testing.T) {
	_, code := run(t, "testdata/env-pass", "env")
	if code != 0 {
		t.Errorf("env (valid) exit = %d, want 0", code)
	}
}

func TestE2E_EnvFail(t *testing.T) {
	out, code := run(t, "testdata/env-fail", "env")
	if code == 0 {
		t.Errorf("env (invalid) exit = 0, want non-zero (out: %s)", out)
	}
	if !strings.Contains(out, "port must be 1-65535") {
		t.Errorf("env fail output %q, want port range message", out)
	}
}

func TestE2E_NoProject(t *testing.T) {
	dir := t.TempDir()
	out, code := run(t, dir, "env")
	if code == 0 {
		t.Errorf("env with no project exit = 0, want non-zero (out: %s)", out)
	}
}
