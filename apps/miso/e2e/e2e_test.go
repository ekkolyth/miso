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
	return runEnv(t, workdir, nil, args...)
}

// like run, with environ as the process env (nil inherits the test's, minus CI)
func runEnv(t *testing.T, workdir string, environ []string, args ...string) (string, int) {
	t.Helper()
	if environ == nil {
		environ = environWithout(nil)
	}
	cmd := exec.Command(misoBin, args...)
	cmd.Dir = workdir
	cmd.Env = environ
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
	if !strings.Contains(out, "expected port 1-65535, got integer above maximum") {
		t.Errorf("env fail output %q, want port range message", out)
	}
	if strings.Contains(out, "99999") {
		t.Errorf("env fail output %q leaks the value", out)
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
		// "<name>-done" proves the script finished, not just that it started
		tail := "echo " + name + "-done >> \"" + filepath.Join(root, "order.log") + "\"\n"
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
	if len(order) != 8 {
		t.Fatalf("order.log = %v, want a start and a finish for each of four scripts", order)
	}
	assertBefore(t, order, "services-done", "migrate")
	assertBefore(t, order, "migrate-done", "dev")
	assertBefore(t, order, "build-done", "dev")
}

func TestE2E_DependsOnStandaloneMode(t *testing.T) {
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

func TestE2E_StandaloneModeDependsOnOnlyTask(t *testing.T) {
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

// the test's env minus the named keys, plus extra. CI is always dropped so a
// CI runner doesn't switch miso into CI mode — tests that want it pass it in extra
func environWithout(keys []string, extra ...string) []string {
	var environ []string
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if key != "CI" && !slices.Contains(keys, key) {
			environ = append(environ, kv)
		}
	}
	return append(environ, extra...)
}

const shellDefaultURL = "postgres://test:test@localhost:55432/ekkolore_test?sslmode=disable"

func TestE2E_ShellDefaultInjected(t *testing.T) {
	environ := environWithout([]string{"TEST_DATABASE_URL"})
	out, code := runEnv(t, "testdata/env-shell-default", environ, "show")
	if code != 0 {
		t.Fatalf("show exit = %d, want 0 (out: %s)", code, out)
	}
	if !strings.Contains(out, "TEST_DATABASE_URL="+shellDefaultURL) {
		t.Errorf("show output %q, want the default URL", out)
	}

	out, code = runEnv(t, "testdata/env-shell-default", environ, "env")
	if code != 0 {
		t.Errorf("env exit = %d, want 0 (out: %s)", code, out)
	}
}

func TestE2E_ShellDefault_ProcessEnvWins(t *testing.T) {
	environ := environWithout([]string{"TEST_DATABASE_URL"}, "TEST_DATABASE_URL=postgres://ci:1/x")
	out, code := runEnv(t, "testdata/env-shell-default", environ, "show")
	if code != 0 {
		t.Fatalf("show exit = %d, want 0 (out: %s)", code, out)
	}
	if !strings.Contains(out, "TEST_DATABASE_URL=postgres://ci:1/x") {
		t.Errorf("show output %q, want the process env value", out)
	}
}

func TestE2E_UnsupportedExpansionStopsBeforeSpawn(t *testing.T) {
	environ := environWithout([]string{"X"})
	for _, args := range [][]string{{"show"}, {"env"}} {
		out, code := runEnv(t, "testdata/env-unsupported", environ, args...)
		if code == 0 {
			t.Errorf("%v exit = 0, want non-zero (out: %s)", args, out)
		}
		if !strings.Contains(out, "${X:?") {
			t.Errorf("%v output %q, want the unsupported form named", args, out)
		}
		if strings.Contains(out, "SPAWNED") || strings.Contains(out, "s3cr3t") {
			t.Errorf("%v output %q: spawned the script or leaked the value", args, out)
		}
	}
}

// every command that reports an env failure prints it through one renderer
func assertSameEnvOutput(t *testing.T, dir string, commands ...[]string) {
	t.Helper()
	environ := environWithout([]string{"PORT", "X", "Y"})
	var first string
	for i, args := range commands {
		out, code := runEnv(t, dir, environ, args...)
		if code == 0 {
			t.Errorf("%v exit = 0, want non-zero (out: %s)", args, out)
		}
		if strings.Contains(out, "SPAWNED") || strings.Contains(out, "s3cr3t") {
			t.Errorf("%v output %q: spawned the script or leaked the value", args, out)
		}
		if i == 0 {
			first = out
			continue
		}
		if out != first {
			t.Errorf("%v printed\n%q\n%v printed\n%q", commands[0], first, args, out)
		}
	}
}

func TestE2E_EnvFailuresRenderIdentically(t *testing.T) {
	assertSameEnvOutput(t, "testdata/env-render-full", []string{"env"}, []string{"--env", "show"})
}

// without --env nothing is validated, so a plain run matches on load failures only
func TestE2E_LoadFailuresRenderIdenticallyWithoutEnvFlag(t *testing.T) {
	assertSameEnvOutput(t, "testdata/env-render-load", []string{"env"}, []string{"--env", "show"}, []string{"show"})
}

// a clean checkout: member web declares .env.local, which isn't there
func writeCleanCheckoutFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixtureFile(t, filepath.Join(root, "package.json"), `{"name":"fx","private":true,"workspaces":["apps/*"]}`)
	writeFixtureFile(t, filepath.Join(root, "package-lock.json"), `{"name":"fx","lockfileVersion":3,"packages":{}}`)
	writeFixtureFile(t, filepath.Join(root, "miso.json"), `{}`)
	web := filepath.Join(root, "apps", "web")
	writeFixtureFile(t, filepath.Join(web, "package.json"), `{"name":"web","private":true}`)
	writeFixtureFile(t, filepath.Join(web, "miso.json"), `{"env":[{"path":".env.local","variables":{"AUTH0_SECRET":"string"}}]}`)
	writeFixtureFile(t, filepath.Join(web, "scripts", "format.sh"), "#!/bin/sh\necho formatter ran\n")
	writeFixtureFile(t, filepath.Join(root, "scripts", "show.sh"), "#!/bin/sh\necho \"args: $*\"\n")
	return root
}

func TestE2E_CleanCheckoutFailsOutsideCI(t *testing.T) {
	out, code := run(t, writeCleanCheckoutFixture(t), "format", "@web")
	if code == 0 {
		t.Fatalf("format exit = 0, want non-zero (out: %s)", out)
	}
	if !strings.Contains(out, "env file not found") || strings.Contains(out, "formatter ran") {
		t.Errorf("format output %q, want env file not found before the formatter starts", out)
	}
}

func TestE2E_CleanCheckoutRunsWithCIFlag(t *testing.T) {
	out, code := run(t, writeCleanCheckoutFixture(t), "--ci", "format", "@web")
	if code != 0 || !strings.Contains(out, "formatter ran") {
		t.Fatalf("--ci format exit = %d, out %q; want 0 and the formatter's output", code, out)
	}
}

func TestE2E_CleanCheckoutRunsWithCIEnv(t *testing.T) {
	out, code := runEnv(t, writeCleanCheckoutFixture(t), environWithout(nil, "CI=true"), "format", "@web")
	if code != 0 || !strings.Contains(out, "formatter ran") {
		t.Fatalf("CI=true format exit = %d, out %q; want 0 and the formatter's output", code, out)
	}
}

func TestE2E_CIEnvValidatesShellVariables(t *testing.T) {
	root := writeCleanCheckoutFixture(t)

	environ := environWithout([]string{"AUTH0_SECRET"}, "CI=true", "AUTH0_SECRET=x")
	if out, code := runEnv(t, root, environ, "env"); code != 0 {
		t.Errorf("env with the secret in the shell exit = %d, want 0 (out: %s)", code, out)
	}

	environ = environWithout([]string{"AUTH0_SECRET"}, "CI=true")
	out, code := runEnv(t, root, environ, "env")
	if code == 0 {
		t.Fatalf("env without the secret exit = 0, want non-zero (out: %s)", out)
	}
	if !strings.Contains(out, "AUTH0_SECRET:") || !strings.Contains(out, "missing required variable") {
		t.Errorf("env output %q, want AUTH0_SECRET reported missing", out)
	}
}

func TestE2E_CIFlagAfterCommandPassesThrough(t *testing.T) {
	out, code := run(t, writeCleanCheckoutFixture(t), "--ci", "show", "--ci")
	if code != 0 || !strings.Contains(out, "args: --ci") {
		t.Fatalf("--ci show --ci exit = %d, out %q; want the trailing --ci passed to the script", code, out)
	}
}

func TestE2E_CIEnvFlagValidatesShellVariables(t *testing.T) {
	root := writeCleanCheckoutFixture(t)

	environ := environWithout([]string{"AUTH0_SECRET"}, "CI=true", "AUTH0_SECRET=x")
	out, code := runEnv(t, root, environ, "--env", "show")
	if code != 0 || !strings.Contains(out, "args:") {
		t.Errorf("--env show with the secret in the shell exit = %d, out %q; want 0 and the script's output", code, out)
	}

	environ = environWithout([]string{"AUTH0_SECRET"}, "CI=true")
	out, code = runEnv(t, root, environ, "--env", "show")
	if code == 0 || strings.Contains(out, "args:") {
		t.Fatalf("--env show without the secret exit = %d, out %q; want non-zero before the script starts", code, out)
	}
	if !strings.Contains(out, "AUTH0_SECRET:") || !strings.Contains(out, "missing required variable") {
		t.Errorf("--env show output %q, want AUTH0_SECRET reported missing", out)
	}
}

// members web and api each define show; web declares AUTH0_SECRET, which its
// .env.local doesn't have. memberConfig false moves web's schema to the root
func writeScopedFixture(t *testing.T, memberConfig bool) string {
	t.Helper()
	root := t.TempDir()
	rootPackage := `{"name":"fx","private":true,"workspaces":["apps/*"]}`
	rootConfig := `{}`
	if !memberConfig {
		rootPackage = `{"name":"fx","private":true,"workspaces":["apps/*"],"scripts":{"show":"echo root-pkg"}}`
		rootConfig = `{"env":[{"scope":"web","path":"apps/web/.env.local","variables":{"AUTH0_SECRET":"string"}}]}`
	}
	writeFixtureFile(t, filepath.Join(root, "package.json"), rootPackage)
	writeFixtureFile(t, filepath.Join(root, "package-lock.json"), `{"name":"fx","lockfileVersion":3,"packages":{}}`)
	writeFixtureFile(t, filepath.Join(root, "miso.json"), rootConfig)
	for _, member := range []string{"web", "api"} {
		dir := filepath.Join(root, "apps", member)
		writeFixtureFile(t, filepath.Join(dir, "package.json"), `{"name":"`+member+`","private":true}`)
		writeFixtureFile(t, filepath.Join(dir, "scripts", "show.sh"), "#!/bin/sh\necho \""+member+" args: $*\"\n")
	}
	if memberConfig {
		writeFixtureFile(t, filepath.Join(root, "apps", "web", "miso.json"), `{"env":[{"path":".env.local","variables":{"AUTH0_SECRET":"string"}}]}`)
	}
	writeFixtureFile(t, filepath.Join(root, "apps", "web", ".env.local"), "OTHER=1\n")
	return root
}

func assertMissingSecret(t *testing.T, args []string, out string, code int) {
	t.Helper()
	if code == 0 {
		t.Errorf("%v exit = 0, want non-zero (out: %s)", args, out)
	}
	if !strings.Contains(out, "AUTH0_SECRET:") || !strings.Contains(out, "missing required variable") {
		t.Errorf("%v output %q, want AUTH0_SECRET reported missing", args, out)
	}
}

func TestE2E_ScopedEnvFlagValidates(t *testing.T) {
	args := []string{"--env", "show", "@web"}
	environ := environWithout([]string{"AUTH0_SECRET"})
	out, code := runEnv(t, writeScopedFixture(t, true), environ, args...)
	assertMissingSecret(t, args, out, code)
	if strings.Contains(out, "web args:") {
		t.Errorf("%v output %q: the script ran despite the failed validation", args, out)
	}
}

func TestE2E_PassthroughEnvFlagValidates(t *testing.T) {
	args := []string{"--env", "outdated"}
	environ := environWithout([]string{"AUTH0_SECRET"})
	out, code := runEnv(t, writeScopedFixture(t, true), environ, args...)
	assertMissingSecret(t, args, out, code)
	if strings.Contains(out, "forwarding to") {
		t.Errorf("%v output %q: forwarded to the package manager despite the failed validation", args, out)
	}
}

func TestE2E_WorkspaceScriptEnvFlagValidates(t *testing.T) {
	args := []string{"--env", "show"}
	environ := environWithout([]string{"AUTH0_SECRET"})
	out, code := runEnv(t, filepath.Join(writeScopedFixture(t, false), "apps", "web"), environ, args...)
	assertMissingSecret(t, args, out, code)
	if strings.Contains(out, "web args:") {
		t.Errorf("%v output %q: the script ran despite the failed validation", args, out)
	}
}

func TestE2E_ScopedRunPassesArgs(t *testing.T) {
	root := writeScopedFixture(t, true)

	out, code := run(t, root, "show", "@web", "foo")
	if code != 0 || !strings.Contains(out, "web args: foo") || strings.Contains(out, "api args:") {
		t.Errorf("show @web foo exit = %d, out %q; want only web, with foo", code, out)
	}

	out, code = run(t, root, "show", "foo")
	if code != 0 || !strings.Contains(out, "web args: foo") || !strings.Contains(out, "api args: foo") {
		t.Errorf("show foo exit = %d, out %q; want both members with foo", code, out)
	}
}

func TestE2E_ScopedRunDoubleDashArgs(t *testing.T) {
	out, code := run(t, writeScopedFixture(t, true), "show", "@web", "--", "foo")
	if code != 0 || !strings.Contains(out, "web args: foo") {
		t.Errorf("show @web -- foo exit = %d, out %q; want web args: foo", code, out)
	}
}

func TestE2E_LeadingEnvFlagValidatesThenRuns(t *testing.T) {
	root := writeScopedFixture(t, true)
	writeFixtureFile(t, filepath.Join(root, "apps", "web", ".env.local"), "AUTH0_SECRET=x\n")
	out, code := run(t, root, "--env", "show", "@web", "foo")
	if code != 0 || !strings.Contains(out, "web args: foo") || strings.Contains(out, "--env") {
		t.Errorf("--env show @web foo exit = %d, out %q; want web args: foo", code, out)
	}
}

// after the command --env belongs to the script, like --ci
func TestE2E_TrailingEnvFlagReachesScript(t *testing.T) {
	out, code := run(t, writeScopedFixture(t, true), "show", "@web", "--env", "foo")
	if code != 0 || !strings.Contains(out, "web args: --env foo") {
		t.Errorf("show @web --env foo exit = %d, out %q; want the script to get --env foo unvalidated", code, out)
	}

	out, code = run(t, filepath.Join(writeScopedFixture(t, false), "apps", "web"), "show", "--env", "foo")
	if code != 0 || !strings.Contains(out, "web args: --env foo") {
		t.Errorf("show --env foo from apps/web exit = %d, out %q; want the script to get --env foo unvalidated", code, out)
	}
}

func TestE2E_TrailingEnvFlagReachesTool(t *testing.T) {
	root := writeScopedFixture(t, true)
	writeFixtureFile(t, filepath.Join(root, "node_modules", ".bin", "tool"), "#!/bin/sh\necho \"tool args: $*\"\n")
	out, code := run(t, root, "exec", "--", "tool", "deploy", "--env", "staging")
	if code != 0 || !strings.Contains(out, "tool args: deploy --env staging") {
		t.Errorf("exec -- tool deploy --env staging exit = %d, out %q; want --env staging passed to the tool", code, out)
	}
}

func TestE2E_MemberDirWithConfigUsesWorkspaceRoot(t *testing.T) {
	args := []string{"env"}
	environ := environWithout([]string{"AUTH0_SECRET"})
	out, code := runEnv(t, filepath.Join(writeScopedFixture(t, true), "apps", "web"), environ, args...)
	if strings.Contains(out, "every root entry needs a scope") {
		t.Fatalf("env from apps/web read the member config as the root: %q", out)
	}
	assertMissingSecret(t, args, out, code)
}

func TestE2E_MemberDirRunsMemberFolderScript(t *testing.T) {
	out, code := run(t, filepath.Join(writeScopedFixture(t, true), "apps", "web"), "show", "foo")
	if code != 0 || !strings.Contains(out, "web args: foo") || strings.Contains(out, "api args:") {
		t.Errorf("show foo from apps/web exit = %d, out %q; want only web, with foo", code, out)
	}
}

func TestE2E_MemberDirWithoutConfigRunsMemberScript(t *testing.T) {
	out, code := run(t, filepath.Join(writeScopedFixture(t, true), "apps", "api"), "show")
	if code != 0 || !strings.Contains(out, "api args: ") || strings.Contains(out, "web args:") {
		t.Errorf("show from apps/api exit = %d, out %q; want only api", code, out)
	}
}

func TestE2E_MemberDirScriptWinsOverRoot(t *testing.T) {
	root := writeScopedFixture(t, true)
	writeFixtureFile(t, filepath.Join(root, "scripts", "show.sh"), "#!/bin/sh\necho \"root args: $*\"\n")
	out, code := run(t, filepath.Join(root, "apps", "web"), "show")
	if code != 0 || !strings.Contains(out, "web args:") || strings.Contains(out, "root args:") {
		t.Errorf("show from apps/web exit = %d, out %q; want the member's script, not the root's", code, out)
	}
}

func TestE2E_MemberDirFallsBackToRootScript(t *testing.T) {
	root := writeScopedFixture(t, true)
	writeFixtureFile(t, filepath.Join(root, "scripts", "hello.sh"), "#!/bin/sh\necho \"root hello\"\n")
	out, code := run(t, filepath.Join(root, "apps", "web"), "hello")
	if code != 0 || !strings.Contains(out, "root hello") {
		t.Errorf("hello from apps/web exit = %d, out %q; want the root script", code, out)
	}
}

func TestE2E_MemberDirDevRunsMemberFolderScript(t *testing.T) {
	root := writeScopedFixture(t, true)
	writeFixtureFile(t, filepath.Join(root, "apps", "web", "scripts", "dev.sh"), "#!/bin/sh\necho \"web dev: $*\"\n")
	out, code := run(t, filepath.Join(root, "apps", "web"), "dev", "foo")
	if code != 0 || !strings.Contains(out, "web dev: foo") {
		t.Errorf("dev foo from apps/web exit = %d, out %q; want web dev: foo", code, out)
	}
}

func TestE2E_MemberDirRunRunsMemberFolderScript(t *testing.T) {
	out, code := run(t, filepath.Join(writeScopedFixture(t, true), "apps", "web"), "run", "show", "--", "foo")
	if code != 0 || !strings.Contains(out, "web args: foo") {
		t.Errorf("run show -- foo from apps/web exit = %d, out %q; want web args: foo", code, out)
	}
}

func TestE2E_MemberDirEnvFlagValidates(t *testing.T) {
	args := []string{"--env", "show"}
	environ := environWithout([]string{"AUTH0_SECRET"})
	out, code := runEnv(t, filepath.Join(writeScopedFixture(t, true), "apps", "web"), environ, args...)
	assertMissingSecret(t, args, out, code)
	if strings.Contains(out, "web args:") {
		t.Errorf("%v output %q: the script ran despite the failed validation", args, out)
	}
}

// a member package.json dev still runs through the package manager with the configured dev flags
func TestE2E_MemberDirPackageJSONDevKeepsFlags(t *testing.T) {
	root := writeScopedFixture(t, true)
	writeFixtureFile(t, filepath.Join(root, "miso.json"), `{"flags":{"dev":["--from-flags"]}}`)
	writeFixtureFile(t, filepath.Join(root, "apps", "api", "package.json"), `{"name":"api","private":true,"scripts":{"dev":"echo api-dev"}}`)
	out, code := run(t, filepath.Join(root, "apps", "api"), "dev")
	if code != 0 || !strings.Contains(out, "api-dev --from-flags") {
		t.Errorf("dev from apps/api exit = %d, out %q; want the dev flags passed", code, out)
	}
}

// an ambiguous member script fails like an ambiguous root one, never falling back to the root
func TestE2E_MemberDirAmbiguousScriptFails(t *testing.T) {
	root := writeScopedFixture(t, true)
	writeFixtureFile(t, filepath.Join(root, "apps", "web", "scripts", "show.ts"), "console.log('web ts')\n")
	writeFixtureFile(t, filepath.Join(root, "scripts", "show.sh"), "#!/bin/sh\necho \"root args: $*\"\n")
	out, code := run(t, filepath.Join(root, "apps", "web"), "show")
	if code == 0 || !strings.Contains(out, "multiple scripts") || strings.Contains(out, "root args:") {
		t.Errorf("show from apps/web exit = %d, out %q; want the ambiguity reported", code, out)
	}
}

// inside a member, dev runs like dev @web from the root: the member's companions start too
func TestE2E_MemberDirDevStartsMemberCompanions(t *testing.T) {
	root := writeScopedFixture(t, true)
	web := filepath.Join(root, "apps", "web")
	writeFixtureFile(t, filepath.Join(web, "miso.json"), `{"repo":{"tasks":{"dev":{"concurrent":["companion"]}}}}`)
	writeFixtureFile(t, filepath.Join(web, "scripts", "dev.sh"), "#!/bin/sh\necho \"web dev: $*\"\n")
	writeFixtureFile(t, filepath.Join(web, "scripts", "companion.sh"), "#!/bin/sh\necho \"web companion\"\n")
	out, code := run(t, web, "dev", "foo")
	if code != 0 || !strings.Contains(out, "web dev: foo") || !strings.Contains(out, "web companion") {
		t.Errorf("dev foo from apps/web exit = %d, out %q; want web dev: foo and the companion", code, out)
	}
}

func TestE2E_MemberDirDevUsesMemberFlags(t *testing.T) {
	root := writeScopedFixture(t, true)
	web := filepath.Join(root, "apps", "web")
	writeFixtureFile(t, filepath.Join(root, "miso.json"), `{"flags":{"dev":["--from-root"]}}`)
	writeFixtureFile(t, filepath.Join(web, "miso.json"), `{"flags":{"dev":["--from-member"]}}`)
	writeFixtureFile(t, filepath.Join(web, "package.json"), `{"name":"web","private":true,"scripts":{"dev":"echo web-dev"}}`)
	out, code := run(t, web, "dev")
	if code != 0 || !strings.Contains(out, "web-dev --from-member") {
		t.Errorf("dev from apps/web exit = %d, out %q; want the member's dev flags", code, out)
	}
}

// a root package.json dev (often "turbo dev") must not pull a member's package.json dev onto the launch path
func TestE2E_MemberDirPackageJSONDevKeepsFlagsWithRootDev(t *testing.T) {
	root := writeScopedFixture(t, true)
	web := filepath.Join(root, "apps", "web")
	writeFixtureFile(t, filepath.Join(root, "package.json"), `{"name":"fx","private":true,"workspaces":["apps/*"],"scripts":{"dev":"echo root-dev"}}`)
	writeFixtureFile(t, filepath.Join(web, "miso.json"), `{"flags":{"dev":["--from-member"]}}`)
	writeFixtureFile(t, filepath.Join(web, "package.json"), `{"name":"web","private":true,"scripts":{"dev":"echo web-dev"}}`)
	out, code := run(t, web, "dev")
	if code != 0 || !strings.Contains(out, "web-dev --from-member") || strings.Contains(out, "root-dev") {
		t.Errorf("dev from apps/web exit = %d, out %q; want the member's dev with its flags", code, out)
	}
}

// a turbo task with no root script keeps its turbo flags and the -- before the task's args
func TestE2E_DelegatedPassthroughKeepsTurboArgs(t *testing.T) {
	root := t.TempDir()
	writeFixtureFile(t, filepath.Join(root, "miso.json"), `{"repo":"turbo"}`)
	writeFixtureFile(t, filepath.Join(root, "turbo.json"), `{"tasks":{"build":{}}}`)
	writeFixtureFile(t, filepath.Join(root, "package.json"), `{"name":"fx","private":true}`)
	writeFixtureFile(t, filepath.Join(root, "package-lock.json"), `{"name":"fx","lockfileVersion":3,"packages":{}}`)
	bin := filepath.Join(root, "bin")
	writeFixtureFile(t, filepath.Join(bin, "turbo"), "#!/bin/sh\necho \"turbo args: $*\"\n")
	environ := environWithout([]string{"PATH"}, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, code := runEnv(t, root, environ, "build", "--filter=web", "--", "--watch")
	if code != 0 || !strings.Contains(out, "--filter=web -- --watch") {
		t.Errorf("build --filter=web -- --watch exit = %d, out %q; want turbo to get --filter=web -- --watch", code, out)
	}
}

func writeStandaloneMembersFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixtureFile(t, filepath.Join(root, "miso.json"), `{"packageManager":false}`)
	for _, name := range []string{"web", "api"} {
		writeFixtureFile(t, filepath.Join(root, name, "miso.json"), `{"packageManager":false}`)
		writeFixtureFile(t, filepath.Join(root, name, "scripts", "dev.sh"),
			"#!/bin/sh\ntouch \""+root+"/"+name+"-ran\"\n")
	}
	return root
}

func assertMarkers(t *testing.T, root string, want map[string]bool) {
	t.Helper()
	for name, exists := range want {
		_, err := os.Stat(filepath.Join(root, name))
		if exists && err != nil {
			t.Errorf("%s missing: %v", name, err)
		}
		if !exists && err == nil {
			t.Errorf("%s exists, want absent", name)
		}
	}
}

func TestE2E_StandaloneRootRunsMembers(t *testing.T) {
	root := writeStandaloneMembersFixture(t)
	out, code := runTimeout(t, root, 30*time.Second, "dev")
	if code != 0 {
		t.Fatalf("miso dev exit = %d, want 0 (out: %s)", code, out)
	}
	assertMarkers(t, root, map[string]bool{"web-ran": true, "api-ran": true})
}

func TestE2E_StandaloneRootScopedRun(t *testing.T) {
	root := writeStandaloneMembersFixture(t)
	out, code := runTimeout(t, root, 30*time.Second, "dev", "@api")
	if code != 0 {
		t.Fatalf("miso dev @api exit = %d, want 0 (out: %s)", code, out)
	}
	assertMarkers(t, root, map[string]bool{"api-ran": true, "web-ran": false})
}

func TestE2E_StandaloneRootFromMemberDir(t *testing.T) {
	root := writeStandaloneMembersFixture(t)
	out, code := runTimeout(t, filepath.Join(root, "api"), 30*time.Second, "dev")
	if code != 0 {
		t.Fatalf("miso dev exit = %d, want 0 (out: %s)", code, out)
	}
	assertMarkers(t, root, map[string]bool{"api-ran": true, "web-ran": false})
}

func TestE2E_StandaloneRootFromMemberDirFallsBackToRoot(t *testing.T) {
	root := writeStandaloneMembersFixture(t)
	writeFixtureFile(t, filepath.Join(root, "scripts", "lint.sh"),
		"#!/bin/sh\ntouch \""+root+"/root-lint-ran\"\n")
	out, code := runTimeout(t, filepath.Join(root, "api"), 30*time.Second, "lint")
	if code != 0 {
		t.Fatalf("miso lint exit = %d, want 0 (out: %s)", code, out)
	}
	assertMarkers(t, root, map[string]bool{"root-lint-ran": true})
}

// root and member differ in package manager; the fake pnpm and npm on PATH
// report which one miso ran. rootLockfile "" leaves the root with none
func writeMemberManagerFixture(t *testing.T, rootLockfile string) (root, web string, environ []string) {
	t.Helper()
	root = t.TempDir()
	writeFixtureFile(t, filepath.Join(root, "package.json"), `{"name":"fx","private":true,"workspaces":["apps/*"]}`)
	writeFixtureFile(t, filepath.Join(root, "miso.json"), `{}`)
	if rootLockfile != "" {
		writeFixtureFile(t, filepath.Join(root, rootLockfile), `{"name":"fx","lockfileVersion":3,"packages":{}}`)
	}
	web = filepath.Join(root, "apps", "web")
	writeFixtureFile(t, filepath.Join(web, "package.json"), `{"name":"web","private":true,"packageManager":"pnpm@9.0.0","scripts":{"hello":"echo hi"}}`)
	bin := filepath.Join(root, "bin")
	for _, name := range []string{"pnpm", "npm"} {
		path := filepath.Join(bin, name)
		writeFixtureFile(t, path, "#!/bin/sh\necho \""+name+" ran: $*\"\n")
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	environ = environWithout([]string{"PATH"}, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return root, web, environ
}

func TestE2E_MemberDirRunUsesMemberPackageManager(t *testing.T) {
	_, web, environ := writeMemberManagerFixture(t, "package-lock.json")
	out, code := runEnv(t, web, environ, "run", "hello")
	if code != 0 || !strings.HasPrefix(out, "pnpm ran:") {
		t.Errorf("run hello from apps/web exit = %d, out %q; want the member's pnpm", code, out)
	}
}

func TestE2E_MemberDirRunWithoutRootManager(t *testing.T) {
	_, web, environ := writeMemberManagerFixture(t, "")
	out, code := runEnv(t, web, environ, "run", "hello")
	if code != 0 || !strings.Contains(out, "pnpm ran:") {
		t.Errorf("run hello from apps/web exit = %d, out %q; want the member's pnpm though the root has none", code, out)
	}
}

func TestE2E_MemberDirScriptUsesMemberPackageManager(t *testing.T) {
	_, web, environ := writeMemberManagerFixture(t, "package-lock.json")
	out, code := runEnv(t, web, environ, "hello")
	if code != 0 || !strings.Contains(out, "pnpm ran:") {
		t.Errorf("hello from apps/web exit = %d, out %q; want the member's pnpm", code, out)
	}
}

// a bun-root workspace with a pnpm member that has its own miso.json; each
// fake manager records the directory and arguments it ran with
func writeInstallFixture(t *testing.T) (root, web, ranFile string, environ []string) {
	t.Helper()
	// resolved so it matches the $PWD the fake managers record; macOS temp dirs sit under a symlinked /var
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(root, "package.json"), `{"name":"fx","private":true,"workspaces":["apps/*"]}`)
	writeFixtureFile(t, filepath.Join(root, "miso.json"), `{}`)
	writeFixtureFile(t, filepath.Join(root, "bun.lock"), "")
	web = filepath.Join(root, "apps", "web")
	writeFixtureFile(t, filepath.Join(web, "package.json"), `{"name":"web","private":true,"packageManager":"pnpm@9.0.0"}`)
	writeFixtureFile(t, filepath.Join(web, "miso.json"), `{}`)
	ranFile = filepath.Join(root, "ran.txt")
	bin := filepath.Join(root, "bin")
	for _, name := range []string{"pnpm", "bun"} {
		path := filepath.Join(bin, name)
		writeFixtureFile(t, path, "#!/bin/sh\necho \""+name+" $PWD $*\" > \""+ranFile+"\"\n")
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	environ = environWithout([]string{"PATH"}, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return root, web, ranFile, environ
}

func TestE2E_InstallRunsInMemberDirWithMemberManager(t *testing.T) {
	_, web, ranFile, environ := writeInstallFixture(t)
	if out, code := runEnv(t, web, environ, "install"); code != 0 {
		t.Fatalf("install from apps/web exit = %d, out %q", code, out)
	}
	ran, err := os.ReadFile(ranFile)
	if err != nil {
		t.Fatalf("no package manager ran: %v", err)
	}
	if !strings.HasPrefix(string(ran), "pnpm "+web+" ") {
		t.Errorf("install ran %q; want pnpm in %s", ran, web)
	}
}

func TestE2E_InstallFromRootRunsAtRoot(t *testing.T) {
	root, _, ranFile, environ := writeInstallFixture(t)
	if out, code := runEnv(t, root, environ, "install"); code != 0 {
		t.Fatalf("install from root exit = %d, out %q", code, out)
	}
	ran, err := os.ReadFile(ranFile)
	if err != nil {
		t.Fatalf("no package manager ran: %v", err)
	}
	if !strings.HasPrefix(string(ran), "bun "+root+" ") {
		t.Errorf("install ran %q; want bun in %s", ran, root)
	}
}
