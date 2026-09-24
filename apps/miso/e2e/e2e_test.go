package e2e

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

var misoBin string

// stamped into the e2e binary via -ldflags so `miso version` has a known value
const testVersion = "9.9.9-e2e"

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "miso-e2e")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	misoBin = filepath.Join(dir, "miso")

	build := exec.Command("go", "build",
		"-ldflags", "-X github.com/ekkolyth/miso/internal/cli/commands.Version="+testVersion,
		"-o", misoBin, "./cmd")
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

// like run, but a hang fails the test instead of stalling go test
func runTimeout(t *testing.T, workdir string, timeout time.Duration, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, misoBin, args...)
	cmd.Dir = workdir
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("miso %v did not finish within %s (out: %s)", args, timeout, out)
	}
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
	if !strings.Contains(out, testVersion) {
		t.Errorf("version output %q, want to contain %q", out, testVersion)
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

func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

// a script that appends its name (and args) to order.log at the fixture root,
// then runs tail
func writeOrderScript(t *testing.T, root, name, tail string) {
	t.Helper()
	body := "#!/bin/sh\necho " + name + " \"$@\" | sed 's/ *$//' >> \"" + filepath.Join(root, "order.log") + "\"\n" + tail
	writeFixtureFile(t, filepath.Join(root, "scripts", name+".sh"), body)
}

func readOrder(t *testing.T, root string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "order.log"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	return strings.Fields(strings.TrimSpace(string(data)))
}

// the MISO-7 fixture: dev depends on migrate and build, migrate on services
func writeDependsOnFixture(t *testing.T, packageManager bool, failing string) string {
	t.Helper()
	root := t.TempDir()
	pm := "false"
	if packageManager {
		pm = "true"
		writeFixtureFile(t, filepath.Join(root, "package.json"), `{"name":"fx","private":true}`)
		writeFixtureFile(t, filepath.Join(root, "package-lock.json"), `{"name":"fx","lockfileVersion":3,"packages":{}}`)
	}
	writeFixtureFile(t, filepath.Join(root, "miso.json"), `{
  "packageManager": `+pm+`,
  "repo": {
    "tasks": {
      "migrate": { "dependsOn": ["services"] },
      "dev": { "dependsOn": ["migrate", "build"] }
    }
  }
}`)
	for _, name := range []string{"services", "migrate", "build", "dev"} {
		tail := ""
		if name == failing {
			tail = "exit 1\n"
		}
		writeOrderScript(t, root, name, tail)
	}
	return root
}

func assertBefore(t *testing.T, order []string, first, second string) {
	t.Helper()
	i, j := slices.Index(order, first), slices.Index(order, second)
	if i < 0 || j < 0 || i > j {
		t.Errorf("order %v: want %s before %s", order, first, second)
	}
}

func assertDependsOnOrder(t *testing.T, root string) {
	t.Helper()
	order := readOrder(t, root)
	if len(order) != 4 {
		t.Fatalf("order.log = %v, want four entries", order)
	}
	assertBefore(t, order, "services", "migrate")
	assertBefore(t, order, "migrate", "dev")
	assertBefore(t, order, "build", "dev")
	if order[3] != "dev" {
		t.Errorf("order %v: want dev last", order)
	}
}

func TestE2E_DependsOnSimpleMode(t *testing.T) {
	root := writeDependsOnFixture(t, false, "")
	out, code := runTimeout(t, root, 30*time.Second, "dev")
	if code != 0 {
		t.Fatalf("miso dev exit = %d, want 0 (out: %s)", code, out)
	}
	assertDependsOnOrder(t, root)
}

func TestE2E_DependsOnPackageManagerMode(t *testing.T) {
	root := writeDependsOnFixture(t, true, "")
	out, code := runTimeout(t, root, 30*time.Second, "dev")
	if code != 0 {
		t.Fatalf("miso dev exit = %d, want 0 (out: %s)", code, out)
	}
	assertDependsOnOrder(t, root)
}

func TestE2E_DependsOnFailureStopsDependent(t *testing.T) {
	root := writeDependsOnFixture(t, false, "migrate")
	out, code := runTimeout(t, root, 10*time.Second, "dev")
	if code == 0 {
		t.Fatalf("miso dev exit = 0, want non-zero (out: %s)", out)
	}
	if order := readOrder(t, root); slices.Contains(order, "dev") {
		t.Errorf("order.log = %v, want no dev after migrate failed", order)
	}
}

func TestE2E_CaretDependsOnFailureDoesNotHang(t *testing.T) {
	root := t.TempDir()
	writeFixtureFile(t, filepath.Join(root, "package.json"), `{"name":"root","private":true,"workspaces":["packages/*"]}`)
	writeFixtureFile(t, filepath.Join(root, "package-lock.json"), `{"name":"root","lockfileVersion":3,"packages":{}}`)
	writeFixtureFile(t, filepath.Join(root, "miso.json"), `{"repo":{"tasks":{"build":{"dependsOn":["^build"]}}}}`)
	writeFixtureFile(t, filepath.Join(root, "packages", "a", "package.json"), `{"name":"a"}`)
	writeFixtureFile(t, filepath.Join(root, "packages", "b", "package.json"), `{"name":"b","dependencies":{"a":"*"}}`)
	writeOrderScript(t, filepath.Join(root, "packages", "a"), "build", "exit 1\n")
	writeFixtureFile(t, filepath.Join(root, "packages", "b", "scripts", "build.sh"),
		"#!/bin/sh\ntouch \""+filepath.Join(root, "b-ran")+"\"\n")

	out, code := runTimeout(t, root, 10*time.Second, "build")
	if code == 0 {
		t.Fatalf("miso build exit = 0, want non-zero (out: %s)", out)
	}
	if _, err := os.Stat(filepath.Join(root, "b-ran")); err == nil {
		t.Error("b built after its upstream a failed")
	}
}

func TestE2E_SimpleModeDependsOnOnlyTask(t *testing.T) {
	root := t.TempDir()
	writeFixtureFile(t, filepath.Join(root, "miso.json"),
		`{"packageManager":false,"repo":{"tasks":{"setup":{"dependsOn":["a","b"]}}}}`)
	writeOrderScript(t, root, "a", "")
	writeOrderScript(t, root, "b", "")

	out, code := runTimeout(t, root, 30*time.Second, "setup")
	if code != 0 {
		t.Fatalf("miso setup exit = %d, want 0 (out: %s)", code, out)
	}
	order := readOrder(t, root)
	slices.Sort(order)
	if !slices.Equal(order, []string{"a", "b"}) {
		t.Errorf("order.log = %v, want a and b", order)
	}
}

func TestE2E_SimpleModeRunSubcommand(t *testing.T) {
	root := t.TempDir()
	writeFixtureFile(t, filepath.Join(root, "miso.json"), `{"packageManager":false}`)
	writeOrderScript(t, root, "hello", "")

	out, code := runTimeout(t, root, 30*time.Second, "run", "hello", "--", "x")
	if code != 0 {
		t.Fatalf("miso run hello exit = %d, want 0 (out: %s)", code, out)
	}
	data, err := os.ReadFile(filepath.Join(root, "order.log"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(data)); got != "hello x" {
		t.Errorf("order.log = %q, want %q", got, "hello x")
	}

	out, code = runTimeout(t, root, 30*time.Second, "run")
	if code == 0 {
		t.Fatalf("miso run exit = 0, want non-zero (out: %s)", out)
	}
	if !strings.Contains(out, "usage: miso run") {
		t.Errorf("output %q, want the miso run usage", out)
	}
}
