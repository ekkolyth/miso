# Env Injection Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Automatically load `.env` file values into the process environment of every script miso orchestrates.

**Architecture:** Add a `BuildProcessEnv` function that merges `.env` file values into `os.Environ()` (process env wins). Add an `environ []string` parameter to `ExecScriptFile`. Add an `Environ` field to the TUI `Process` struct. Update all miso-orchestrated spawn points to call `BuildProcessEnv` and pass the result.

**Tech Stack:** Go, `github.com/joho/godotenv`

**Spec:** `docs/superpowers/specs/2026-03-17-env-injection-design.md`

---

### Task 1: Implement `BuildProcessEnv`

**Files:**
- Modify: `apps/miso/internal/cli/env/env.go`
- Test: `apps/miso/internal/cli/env/env_test.go`

- [ ] **Step 1: Write failing tests for BuildProcessEnv**

Add to the end of `env_test.go`:

```go
func TestBuildProcessEnv_SingleRepo_InjectsEnvFile(t *testing.T) {
	dir := writeTempEnv(t, ".env.local", "DB_URL=postgres://localhost\nAPI_KEY=secret123\n")
	cfg := config.Config{
		Env: []*config.EnvEntry{
			{Path: ".env.local"},
		},
	}

	environ, err := BuildProcessEnv(dir, cfg, dir)
	if err != nil {
		t.Fatalf("BuildProcessEnv() error: %v", err)
	}
	if environ == nil {
		t.Fatal("environ is nil, want non-nil")
	}

	envMap := envSliceToMap(environ)
	if envMap["DB_URL"] != "postgres://localhost" {
		t.Errorf("DB_URL = %q, want %q", envMap["DB_URL"], "postgres://localhost")
	}
	if envMap["API_KEY"] != "secret123" {
		t.Errorf("API_KEY = %q, want %q", envMap["API_KEY"], "secret123")
	}
}

func TestBuildProcessEnv_ProcessEnvWins(t *testing.T) {
	dir := writeTempEnv(t, ".env.local", "HOME=/from/file\n")
	cfg := config.Config{
		Env: []*config.EnvEntry{
			{Path: ".env.local"},
		},
	}

	environ, err := BuildProcessEnv(dir, cfg, dir)
	if err != nil {
		t.Fatalf("BuildProcessEnv() error: %v", err)
	}

	envMap := envSliceToMap(environ)
	// HOME should be the real value from os.Environ(), not the file value
	if envMap["HOME"] == "/from/file" {
		t.Error("HOME was overwritten by .env file, want process env to win")
	}
}

func TestBuildProcessEnv_NoConfig_Discovery(t *testing.T) {
	dir := writeTempEnv(t, ".env.local", "DISCOVERED=yes\n")
	cfg := config.Config{}

	environ, err := BuildProcessEnv(dir, cfg, dir)
	if err != nil {
		t.Fatalf("BuildProcessEnv() error: %v", err)
	}

	envMap := envSliceToMap(environ)
	if envMap["DISCOVERED"] != "yes" {
		t.Errorf("DISCOVERED = %q, want %q (should auto-discover .env.local)", envMap["DISCOVERED"], "yes")
	}
}

func TestBuildProcessEnv_NoConfig_NoFile_ReturnsNil(t *testing.T) {
	dir := t.TempDir() // empty dir, no .env files
	cfg := config.Config{}

	environ, err := BuildProcessEnv(dir, cfg, dir)
	if err != nil {
		t.Fatalf("BuildProcessEnv() error: %v", err)
	}
	if environ != nil {
		t.Errorf("environ = %v, want nil (no .env file found)", environ)
	}
}

func TestBuildProcessEnv_MissingFile_SoftFail(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Env: []*config.EnvEntry{
			{Path: "nonexistent.env"},
		},
	}

	// Should not return an error — soft fail
	environ, err := BuildProcessEnv(dir, cfg, dir)
	if err != nil {
		t.Fatalf("BuildProcessEnv() should soft-fail, got error: %v", err)
	}
	// Returns nil when no files could be loaded (script inherits parent env via cmd.Env = nil)
	if environ != nil {
		t.Errorf("environ should be nil when all files missing (inherit parent env), got %d entries", len(environ))
	}
}

func TestBuildProcessEnv_Monorepo_ScopedByPath(t *testing.T) {
	dir := t.TempDir()
	// Create workspace dirs with env files
	webDir := filepath.Join(dir, "apps", "web")
	apiDir := filepath.Join(dir, "apps", "api")
	os.MkdirAll(webDir, 0o755)
	os.MkdirAll(apiDir, 0o755)
	os.WriteFile(filepath.Join(webDir, ".env.local"), []byte("WEB_PORT=3000\n"), 0o644)
	os.WriteFile(filepath.Join(apiDir, ".env"), []byte("API_PORT=4000\n"), 0o644)

	cfg := config.Config{
		Repo: "mono",
		Env: []*config.EnvEntry{
			{Label: "web", Path: "apps/web/.env.local"},
			{Label: "api", Path: "apps/api/.env"},
		},
	}

	// Build env for web workspace — should only get WEB_PORT
	environ, err := BuildProcessEnv(dir, cfg, webDir)
	if err != nil {
		t.Fatalf("BuildProcessEnv() error: %v", err)
	}
	envMap := envSliceToMap(environ)
	if envMap["WEB_PORT"] != "3000" {
		t.Errorf("WEB_PORT = %q, want %q", envMap["WEB_PORT"], "3000")
	}
	if envMap["API_PORT"] != "" {
		t.Errorf("API_PORT = %q, want empty (should not leak from api workspace)", envMap["API_PORT"])
	}
}

func TestBuildProcessEnv_Monorepo_RootGetsAll(t *testing.T) {
	dir := t.TempDir()
	webDir := filepath.Join(dir, "apps", "web")
	apiDir := filepath.Join(dir, "apps", "api")
	os.MkdirAll(webDir, 0o755)
	os.MkdirAll(apiDir, 0o755)
	os.WriteFile(filepath.Join(webDir, ".env.local"), []byte("WEB_PORT=3000\n"), 0o644)
	os.WriteFile(filepath.Join(apiDir, ".env"), []byte("API_PORT=4000\n"), 0o644)

	cfg := config.Config{
		Repo: "mono",
		Env: []*config.EnvEntry{
			{Label: "web", Path: "apps/web/.env.local"},
			{Label: "api", Path: "apps/api/.env"},
		},
	}

	// Build env for root — should get both
	environ, err := BuildProcessEnv(dir, cfg, dir)
	if err != nil {
		t.Fatalf("BuildProcessEnv() error: %v", err)
	}
	envMap := envSliceToMap(environ)
	if envMap["WEB_PORT"] != "3000" {
		t.Errorf("WEB_PORT = %q, want %q", envMap["WEB_PORT"], "3000")
	}
	if envMap["API_PORT"] != "4000" {
		t.Errorf("API_PORT = %q, want %q", envMap["API_PORT"], "4000")
	}
}

func TestBuildProcessEnv_Monorepo_NoConfig_PerWorkspaceDiscovery(t *testing.T) {
	dir := t.TempDir()
	webDir := filepath.Join(dir, "apps", "web")
	os.MkdirAll(webDir, 0o755)
	os.WriteFile(filepath.Join(webDir, ".env.local"), []byte("DISCOVERED_WEB=yes\n"), 0o644)

	cfg := config.Config{Repo: "mono"}

	environ, err := BuildProcessEnv(dir, cfg, webDir)
	if err != nil {
		t.Fatalf("BuildProcessEnv() error: %v", err)
	}
	envMap := envSliceToMap(environ)
	if envMap["DISCOVERED_WEB"] != "yes" {
		t.Errorf("DISCOVERED_WEB = %q, want %q", envMap["DISCOVERED_WEB"], "yes")
	}
}

func TestBuildProcessEnv_MultipleEntries_LaterWins(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.env"), []byte("SHARED=from_a\nONLY_A=yes\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.env"), []byte("SHARED=from_b\nONLY_B=yes\n"), 0o644)

	cfg := config.Config{
		Env: []*config.EnvEntry{
			{Path: "a.env"},
			{Path: "b.env"},
		},
	}

	environ, err := BuildProcessEnv(dir, cfg, dir)
	if err != nil {
		t.Fatalf("BuildProcessEnv() error: %v", err)
	}
	envMap := envSliceToMap(environ)
	if envMap["SHARED"] != "from_b" {
		t.Errorf("SHARED = %q, want %q (later entry should win)", envMap["SHARED"], "from_b")
	}
	if envMap["ONLY_A"] != "yes" {
		t.Errorf("ONLY_A = %q, want %q", envMap["ONLY_A"], "yes")
	}
	if envMap["ONLY_B"] != "yes" {
		t.Errorf("ONLY_B = %q, want %q", envMap["ONLY_B"], "yes")
	}
}

// envSliceToMap converts []string{"KEY=VALUE",...} to map[string]string
func envSliceToMap(environ []string) map[string]string {
	m := make(map[string]string)
	for _, e := range environ {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go test ./internal/cli/env/ -run "TestBuildProcessEnv" -v`
Expected: FAIL — `BuildProcessEnv` undefined.

- [ ] **Step 3: Implement BuildProcessEnv**

Add to `apps/miso/internal/cli/env/env.go`:

```go
// BuildProcessEnv builds a merged environment for spawning scripts.
// It loads .env files and merges them with os.Environ(). Process env wins —
// .env values only fill in vars that aren't already set.
// Returns nil if no env files are found/configured (caller should use default inherited env).
func BuildProcessEnv(projectRoot string, cfg config.Config, workspaceDir string) ([]string, error) {
	// Collect all env vars from .env files
	fileVars := make(map[string]string)
	loaded := false

	if len(cfg.Env) > 0 {
		for _, entry := range cfg.Env {
			absPath := filepath.Join(projectRoot, entry.Path)
			if entry.Path == "" {
				// No path: use discovery in the appropriate directory
				searchDir := projectRoot
				if cfg.IsMonorepo() && workspaceDir != projectRoot {
					searchDir = workspaceDir
				}
				discovered, err := discoverEnvFile(searchDir)
				if err != nil {
					continue // soft-fail: file not found
				}
				absPath = discovered
			}

			// Monorepo scoping: only load entries whose path falls under workspaceDir
			if cfg.IsMonorepo() && workspaceDir != projectRoot {
				if !strings.HasPrefix(absPath, workspaceDir+string(filepath.Separator)) && absPath != workspaceDir {
					continue
				}
			}

			envMap, err := loadEnvFile(absPath)
			if err != nil {
				continue // soft-fail: warn and skip
			}
			loaded = true
			for k, v := range envMap {
				fileVars[k] = v
			}
		}
	} else {
		// No config: discovery mode
		searchDir := projectRoot
		if cfg.IsMonorepo() && workspaceDir != projectRoot {
			searchDir = workspaceDir
		}
		path, err := discoverEnvFile(searchDir)
		if err != nil {
			return nil, nil // no .env file found, return nil
		}
		envMap, err := loadEnvFile(path)
		if err != nil {
			return nil, nil
		}
		loaded = true
		for k, v := range envMap {
			fileVars[k] = v
		}
	}

	if !loaded {
		return nil, nil
	}

	// Build merged environment: start with os.Environ(), then fill gaps from fileVars
	processEnv := os.Environ()
	existing := make(map[string]bool)
	for _, e := range processEnv {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) >= 1 {
			existing[parts[0]] = true
		}
	}

	for k, v := range fileVars {
		if !existing[k] {
			processEnv = append(processEnv, k+"="+v)
		}
	}

	return processEnv, nil
}
```

Note: add `"strings"` to imports if not already present.

- [ ] **Step 4: Run ALL env tests**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go test ./internal/cli/env/ -v`
Expected: ALL PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/mikekenway/Development/miso
git add apps/miso/internal/cli/env/env.go apps/miso/internal/cli/env/env_test.go
git commit -m "feat: add BuildProcessEnv for automatic .env injection"
```

---

### Task 2: Add `environ` parameter to `ExecScriptFile`

**Files:**
- Modify: `apps/miso/internal/cli/scripting/exec.go:14` (ExecScriptFile signature)
- Modify: `apps/miso/internal/cli/scripting/override.go:19` (RunOverride caller)

- [ ] **Step 1: Add `environ` parameter to ExecScriptFile**

In `apps/miso/internal/cli/scripting/exec.go`, change the signature:

```go
func ExecScriptFile(scriptPath string, args []string, workDir string, defaultShell string, environ []string) error {
```

And before `return cmd.Run()` (after the existing `cmd.Stderr = os.Stderr` line), add:

```go
	if environ != nil {
		cmd.Env = environ
	}
```

- [ ] **Step 2: Update RunOverride signature to accept environ**

In `apps/miso/internal/cli/scripting/override.go`, update the function to accept and pass through environ:

```go
func RunOverride(scriptName string, scriptArgs []string, root string, cfg config.Config, environ []string) error {
	resolved, err := ResolveScript(scriptName, root, cfg)
	if err != nil {
		return err
	}
	if resolved.Source == ScriptSourceNone {
		return fmt.Errorf("script %q not found", scriptName)
	}
	if resolved.Source == ScriptSourceFolder {
		return ExecScriptFile(resolved.Path, scriptArgs, root, cfg.Shell, environ)
	}
	if resolved.Source == ScriptSourcePackageJSON {
		return fmt.Errorf("script %q found in package.json, use package manager directly", scriptName)
	}
	return fmt.Errorf("unknown script source")
}
```

- [ ] **Step 3: Update all callers in main.go to pass nil (temporary)**

In `apps/miso/cmd/main.go`, find ALL `ExecScriptFile` and `RunOverride` calls and add `nil` as the last argument:

- Simple mode block (~line 175): `scripting.ExecScriptFile(resolved.Path, scriptArgs, originalWorkDir, cfg.Shell)` → add `, nil`
- `ActionWorkspaceScript` (~line 310): `scripting.ExecScriptFile(resolved.Path, parsed.ScriptArgs, workDir, cfg.Shell)` → add `, nil`
- `ActionScriptOverride` (~line 325): `scripting.RunOverride(parsed.ScriptName, parsed.ScriptArgs, projectRoot, cfg)` → add `, nil`
- `ActionScriptFolder` (~line 330): `scripting.ExecScriptFile(parsed.Command, parsed.ScriptArgs, originalWorkDir, cfg.Shell)` → add `, nil`

- [ ] **Step 4: Verify compilation**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go build ./cmd/`
Expected: Build succeeds.

- [ ] **Step 5: Run all tests**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go test ./... -v 2>&1 | tail -5`
Expected: ALL PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/mikekenway/Development/miso
git add apps/miso/internal/cli/scripting/exec.go apps/miso/internal/cli/scripting/override.go apps/miso/cmd/main.go
git commit -m "feat: add environ parameter to ExecScriptFile and RunOverride"
```

---

### Task 3: Inject env at all miso-orchestrated spawn points

**Files:**
- Modify: `apps/miso/cmd/main.go` (simple mode, ActionScriptFolder, ActionWorkspaceScript, ActionScriptOverride)

- [ ] **Step 1: Update simple mode block in main.go**

In the simple mode block, before the `ExecScriptFile` call, add:

```go
		// Build env for injection
		processEnv, envErr := env.BuildProcessEnv(projectRoot, cfg, originalWorkDir)
		if envErr != nil {
			cli.Fail(logger, envErr, false)
		}
```

Then update the `ExecScriptFile` call to pass `processEnv` instead of `nil`:

```go
		if err := scripting.ExecScriptFile(resolved.Path, scriptArgs, originalWorkDir, cfg.Shell, processEnv); err != nil {
```

- [ ] **Step 2: Update ActionScriptFolder in main.go**

In the `ActionScriptFolder` case, add env building before the exec call:

```go
	case cli.ActionScriptFolder:
		processEnv, envErr := env.BuildProcessEnv(projectRoot, cfg, originalWorkDir)
		if envErr != nil {
			cli.Fail(logger, envErr, false)
		}
		if err := scripting.ExecScriptFile(parsed.Command, parsed.ScriptArgs, originalWorkDir, cfg.Shell, processEnv); err != nil {
			cli.Fail(logger, err, false)
		}
		return
```

- [ ] **Step 3: Update ActionWorkspaceScript in main.go**

In the `ActionWorkspaceScript` case, add env building with the workspace's work dir:

```go
	case cli.ActionWorkspaceScript:
		resolved, workDir, err := scripting.ResolveWorkspaceScript(
			parsed.WorkspaceName, parsed.ScriptName, projectRoot, cfg,
		)
		if err != nil {
			cli.Fail(logger, err, false)
		}
		if resolved.Source == scripting.ScriptSourceNone {
			cli.Fail(logger, fmt.Errorf("script %q not found in workspace %q", parsed.ScriptName, parsed.WorkspaceName), false)
		}
		processEnv, envErr := env.BuildProcessEnv(projectRoot, cfg, workDir)
		if envErr != nil {
			cli.Fail(logger, envErr, false)
		}
		if err := scripting.ExecScriptFile(resolved.Path, parsed.ScriptArgs, workDir, cfg.Shell, processEnv); err != nil {
			cli.Fail(logger, err, false)
		}
		return
```

- [ ] **Step 4: Update ActionScriptOverride in main.go**

`RunOverride` already accepts `environ` (updated in Task 2). Now pass actual env instead of nil:

```go
	case cli.ActionScriptOverride:
		processEnv, envErr := env.BuildProcessEnv(projectRoot, cfg, originalWorkDir)
		if envErr != nil {
			cli.Fail(logger, envErr, false)
		}
		if err := scripting.RunOverride(parsed.ScriptName, parsed.ScriptArgs, projectRoot, cfg, processEnv); err != nil {
			cli.Fail(logger, err, false)
		}
		return
```

- [ ] **Step 5: Verify compilation**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go build ./cmd/`
Expected: Build succeeds.

- [ ] **Step 6: Run all tests**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go test ./... 2>&1 | grep -E "^(ok|FAIL)"`
Expected: ALL ok, no FAIL.

- [ ] **Step 7: Commit**

```bash
cd /Users/mikekenway/Development/miso
git add apps/miso/cmd/main.go
git commit -m "feat: inject env at all miso-orchestrated spawn points"
```

---

### Task 4: TUI process env injection

**Files:**
- Modify: `apps/miso/internal/tui/process.go:37-49` (Process struct)
- Modify: `apps/miso/internal/tui/process.go:91-102` (Start method)
- Modify: `apps/miso/internal/tui/process.go:71-87` (Add method)
- Modify: `apps/miso/internal/tui/launch.go:36-60` (Launch loop)

- [ ] **Step 1: Add Environ field to Process struct**

In `apps/miso/internal/tui/process.go`, add an `Environ` field to the `Process` struct:

```go
type Process struct {
	Entry    TuiScriptEntry
	Command  string
	Args     []string
	Dir      string   // working directory for the process
	Environ  []string // environment variables for the process (nil = inherit)
	State    ProcessState
	ExitCode int
	Buffer   *RingBuffer

	cmd  *exec.Cmd
	done chan struct{}
	mu   sync.Mutex
}
```

- [ ] **Step 2: Apply Environ in Start method**

In `ProcessManager.Start()`, after setting `p.cmd.Dir` and before `p.cmd.SysProcAttr`, add:

```go
	if p.Environ != nil {
		p.cmd.Env = p.Environ
	}
```

- [ ] **Step 3: Add environ parameter to ProcessManager.Add**

Update the `Add` method signature and use it:

```go
func (pm *ProcessManager) Add(entry TuiScriptEntry, command string, args []string, dir string, environ []string) *Process {
	p := &Process{
		Entry:   entry,
		Command: command,
		Args:    args,
		Dir:     dir,
		Environ: environ,
		State:   StateStarting,
		Buffer:  NewRingBuffer(DefaultBufferSize),
		done:    make(chan struct{}),
	}
```

- [ ] **Step 4: Update Launch to build and pass env**

In `apps/miso/internal/tui/launch.go`, add the `env` import and update the Launch function's process-building loop. After building `cmd` and `args` for each entry, build the env:

```go
	for _, entry := range entries {
		var cmd string
		var args []string
		dir := entry.WorkspaceDir

		if entry.ScriptSource == "folder" {
			shell := cfg.Shell
			if shell == "" {
				shell = "sh"
			}
			cmd = shell
			args = []string{"-e", entry.ScriptPath}
		} else {
			if mgr == nil {
				return false, fmt.Errorf("script %q requires a package manager but none is configured", entry.ScriptName)
			}
			spec := mgr.BuildRun(entry.ScriptName, nil)
			cmd = spec.Command
			args = spec.Args
		}

		// Build env for this process (scoped to workspace dir)
		processEnv, envErr := env.BuildProcessEnv(root, cfg, dir)
		if envErr != nil {
			return false, fmt.Errorf("build env for %s: %w", entry.Label, envErr)
		}

		pm.Add(entry, cmd, args, dir, processEnv)
	}
```

Add `"github.com/ekkolyth/miso/internal/cli/env"` to the imports in launch.go.

- [ ] **Step 5: Update DelegateLaunch**

In `apps/miso/internal/tui/delegate.go`, there are two `pm.Add` call sites that need the extra `nil` environ parameter (turbo/nx handles its own env):

- In `routeBasic` (~line 103): `pm.Add(entry, "", nil, root)` → `pm.Add(entry, "", nil, root, nil)`
- In `routeTurbo` (~line 119): `pm.Add(entry, "", nil, root)` → `pm.Add(entry, "", nil, root, nil)`

Update both.

- [ ] **Step 6: Verify compilation**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go build ./cmd/`
Expected: Build succeeds.

- [ ] **Step 7: Run all tests**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go test ./... 2>&1 | grep -E "^(ok|FAIL)"`
Expected: ALL ok.

- [ ] **Step 8: Commit**

```bash
cd /Users/mikekenway/Development/miso
git add apps/miso/internal/tui/process.go apps/miso/internal/tui/launch.go apps/miso/internal/tui/delegate.go
git commit -m "feat: TUI process env injection"
```

---

### Task 5: Integration verification

**Files:**
- No new files — manual verification.

- [ ] **Step 1: Run all tests**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go test ./... -v 2>&1 | grep -E "^(ok|FAIL)"`
Expected: ALL ok.

- [ ] **Step 2: Build binary**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go build -o bin/miso ./cmd/`

- [ ] **Step 3: Manual test — env injection in simple mode**

```bash
cd /tmp && rm -rf test-env && mkdir test-env && cd test-env
cat > miso.json << 'EOF'
{
  "packageManager": false,
  "scripts": "./scripts",
  "env": {
    "path": ".env.local"
  }
}
EOF
echo 'MY_SECRET=injected_value' > .env.local
mkdir scripts
printf '#!/bin/sh\necho "MY_SECRET=$MY_SECRET"' > scripts/check.sh
chmod +x scripts/check.sh

/Users/mikekenway/Development/miso/apps/miso/bin/miso check
# Expected: MY_SECRET=injected_value

cd / && rm -rf /tmp/test-env
```

- [ ] **Step 4: Manual test — process env wins**

```bash
cd /tmp && rm -rf test-env2 && mkdir test-env2 && cd test-env2
cat > miso.json << 'EOF'
{
  "packageManager": false,
  "scripts": "./scripts",
  "env": {
    "path": ".env.local"
  }
}
EOF
echo 'MY_VAR=from_file' > .env.local
mkdir scripts
printf '#!/bin/sh\necho "MY_VAR=$MY_VAR"' > scripts/check.sh
chmod +x scripts/check.sh

MY_VAR=from_shell /Users/mikekenway/Development/miso/apps/miso/bin/miso check
# Expected: MY_VAR=from_shell (process env wins)

cd / && rm -rf /tmp/test-env2
```

- [ ] **Step 5: Manual test — auto-discovery (no env config)**

```bash
cd /tmp && rm -rf test-env3 && mkdir test-env3 && cd test-env3
cat > miso.json << 'EOF'
{
  "packageManager": false,
  "scripts": "./scripts"
}
EOF
echo 'AUTO_DISCOVERED=yes' > .env.local
mkdir scripts
printf '#!/bin/sh\necho "AUTO_DISCOVERED=$AUTO_DISCOVERED"' > scripts/check.sh
chmod +x scripts/check.sh

/Users/mikekenway/Development/miso/apps/miso/bin/miso check
# Expected: AUTO_DISCOVERED=yes

cd / && rm -rf /tmp/test-env3
```

---

### Task 6: Update documentation

**Files:**
- Modify: `apps/docs/content/working-with-miso/config.mdx`
- Modify: `apps/docs/content/env-validation/index.mdx`

- [ ] **Step 1: Update config.mdx env section**

In `apps/docs/content/working-with-miso/config.mdx`, find the "### Env" section and add an explanation of env injection at the top of the section, before the config example:

```mdx
### Env

Miso automatically loads your registered `.env` files into the environment of every script it runs. You don't need to do anything special — if you've configured an `env` block, the variables are available to your scripts.

For example, if your `.env.local` has `DATABASE_URL=postgres://localhost/mydb`, any script miso runs will see that value. If the variable is already set in your shell (e.g., you exported `DATABASE_URL` yourself), your shell value takes precedence — the `.env` file only fills in variables that aren't already set.

If you don't configure an `env` block at all, miso will auto-discover `.env` files in this order: `.env.local`, `.env.production`, `.env.development`, `.env`.

In monorepo mode, each workspace only gets the env from its own directory — variables from other workspaces don't bleed across boundaries.

The `--env` flag adds **validation** before running the script. Without it, variables are loaded but not validated.
```

- [ ] **Step 2: Update env-validation index**

Read `apps/docs/content/env-validation/index.mdx` and add a note about the relationship between injection and validation:

```mdx
> **Note:** Miso automatically loads `.env` files into your scripts — you don't need `--env` for that. The `--env` flag and `miso env` are for **validation**: checking that variables exist and match their expected types before running.
```

- [ ] **Step 3: Commit documentation**

```bash
cd /Users/mikekenway/Development/miso
git add apps/docs/content/
git commit -m "docs: document automatic env injection behavior"
```
