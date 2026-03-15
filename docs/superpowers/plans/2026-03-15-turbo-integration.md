# Turbo Integration Layer Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a dedicated `internal/turbo` package that handles turbo.json parsing, output parsing with per-workspace exit codes, and transparent flag passthrough — then wire it into miso's delegate launch flow and `miso scripts`.

**Architecture:** New `internal/turbo` package with three focused files (config, output, flags). `delegate.go` becomes a thin orchestrator calling into this package. CLI passes unknown flags through transparently. `miso scripts` merges turbo.json tasks.

**Tech Stack:** Go, regex for output parsing, JSON unmarshaling for turbo.json

**Spec:** `docs/superpowers/specs/2026-03-15-turbo-integration-design.md`

---

## Chunk 1: `turbo` package — output parsing and flags

### Task 1: Create `turbo.ParseLine` with tests

**Files:**
- Create: `apps/miso/internal/turbo/output.go`
- Create: `apps/miso/internal/turbo/output_test.go`

- [ ] **Step 1: Write failing tests for `ParseLine`**

In `apps/miso/internal/turbo/output_test.go`:

```go
package turbo

import "testing"

func TestParseLineWorkspaceOutput(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		label string
		text  string
		skip  bool
	}{
		{"workspace output", "web:build: compiling...", "web:build", "compiling...", false},
		{"shared output", "shared:build: done in 1.2s", "shared:build", "done in 1.2s", false},
		{"boilerplate", "• Packages in scope: web, api", "", "", true},
		{"empty no-prefix", "no prefix here", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := ParseLine(tt.line)
			if meta.Label != tt.label {
				t.Errorf("Label = %q, want %q", meta.Label, tt.label)
			}
			if meta.Text != tt.text {
				t.Errorf("Text = %q, want %q", meta.Text, tt.text)
			}
			if meta.Skip != tt.skip {
				t.Errorf("Skip = %v, want %v", meta.Skip, tt.skip)
			}
		})
	}
}

func TestParseLineExitCode(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		isExit   bool
		exitCode int
		label    string
	}{
		{"exit success", "web:build: exited with code 0", true, 0, "web:build"},
		{"exit failure", "api:build: exited with code 1", true, 1, "api:build"},
		{"not exit", "web:build: compiling...", false, 0, "web:build"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := ParseLine(tt.line)
			if meta.IsExit != tt.isExit {
				t.Errorf("IsExit = %v, want %v", meta.IsExit, tt.isExit)
			}
			if meta.ExitCode != tt.exitCode {
				t.Errorf("ExitCode = %d, want %d", meta.ExitCode, tt.exitCode)
			}
			if meta.Label != tt.label {
				t.Errorf("Label = %q, want %q", meta.Label, tt.label)
			}
		})
	}
}

func TestParseLineCacheStatus(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		cacheHit *bool
		label    string
	}{
		{"cache hit", "web:build: cache hit, replaying output", boolPtr(true), "web:build"},
		{"cache miss", "web:build: cache miss, executing", boolPtr(false), "web:build"},
		{"not cache", "web:build: compiling...", nil, "web:build"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := ParseLine(tt.line)
			if tt.cacheHit == nil && meta.CacheHit != nil {
				t.Errorf("CacheHit = %v, want nil", *meta.CacheHit)
			}
			if tt.cacheHit != nil {
				if meta.CacheHit == nil {
					t.Fatalf("CacheHit = nil, want %v", *tt.cacheHit)
				}
				if *meta.CacheHit != *tt.cacheHit {
					t.Errorf("CacheHit = %v, want %v", *meta.CacheHit, *tt.cacheHit)
				}
			}
			if meta.Label != tt.label {
				t.Errorf("Label = %q, want %q", meta.Label, tt.label)
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/miso && go test ./internal/turbo/ -v`
Expected: FAIL — `ParseLine` not defined

- [ ] **Step 3: Implement `ParseLine`**

In `apps/miso/internal/turbo/output.go`:

```go
package turbo

import (
	"regexp"
	"strconv"
	"strings"
)

// LineMeta holds parsed metadata from a single line of turbo output.
type LineMeta struct {
	Label    string
	Text     string
	Skip     bool  // boilerplate line, discard
	IsExit   bool
	ExitCode int
	CacheHit *bool // nil = not a cache line, true/false = hit/miss
}

// turboLineRe matches "workspace:task: output text" where task may contain
// colons (e.g. "db:studio"), so the label is everything up to the last ": ".
var turboLineRe = regexp.MustCompile(`^([a-zA-Z0-9@/_.-]+:[a-zA-Z0-9:_.-]+): (.*)$`)

// exitCodeRe matches "exited with code N" in the text portion of a turbo line.
var exitCodeRe = regexp.MustCompile(`^exited with code (\d+)$`)

// ParseLine extracts workspace label, text, and metadata from a single line
// of turbo's streamed output. Stateless — call per line.
func ParseLine(line string) LineMeta {
	m := turboLineRe.FindStringSubmatch(line)
	if m == nil {
		// Unmatched line — turbo boilerplate
		return LineMeta{Skip: true}
	}

	label := m[1]
	text := m[2]

	meta := LineMeta{Label: label, Text: text}

	// Check for exit code
	if em := exitCodeRe.FindStringSubmatch(text); em != nil {
		code, _ := strconv.Atoi(em[1])
		meta.IsExit = true
		meta.ExitCode = code
	}

	// Check for cache status
	if strings.HasPrefix(text, "cache hit") {
		hit := true
		meta.CacheHit = &hit
	} else if strings.HasPrefix(text, "cache miss") {
		miss := false
		meta.CacheHit = &miss
	}

	return meta
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd apps/miso && go test ./internal/turbo/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add apps/miso/internal/turbo/output.go apps/miso/internal/turbo/output_test.go
git commit -m "feat(turbo): add ParseLine for turbo output parsing with exit codes and cache status"
```

---

### Task 2: Create `turbo.SplitFlags` with tests

**Files:**
- Create: `apps/miso/internal/turbo/flags.go`
- Create: `apps/miso/internal/turbo/flags_test.go`

- [ ] **Step 1: Write failing tests for `SplitFlags`**

In `apps/miso/internal/turbo/flags_test.go`:

```go
package turbo

import (
	"reflect"
	"testing"
)

func TestSplitFlags(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		tuiActive bool
		miso      []string
		turbo     []string
	}{
		{
			"all passthrough",
			[]string{"--filter=web", "--concurrency=2"},
			true,
			nil,
			[]string{"--filter=web", "--concurrency=2"},
		},
		{
			"miso --env stripped",
			[]string{"--env", "--filter=web"},
			true,
			[]string{"--env"},
			[]string{"--filter=web"},
		},
		{
			"--log-order=value stripped with TUI",
			[]string{"--log-order=grouped", "--filter=web"},
			true,
			nil,
			[]string{"--filter=web"},
		},
		{
			"--log-order value (space) stripped with TUI",
			[]string{"--log-order", "grouped", "--filter=web"},
			true,
			nil,
			[]string{"--filter=web"},
		},
		{
			"--log-order kept without TUI",
			[]string{"--log-order=grouped", "--filter=web"},
			false,
			nil,
			[]string{"--log-order=grouped", "--filter=web"},
		},
		{
			"--log-order value (space) kept without TUI",
			[]string{"--log-order", "grouped", "--filter=web"},
			false,
			nil,
			[]string{"--log-order", "grouped", "--filter=web"},
		},
		{
			"empty args",
			nil,
			true,
			nil,
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			miso, turbo := SplitFlags(tt.args, tt.tuiActive)
			if !reflect.DeepEqual(miso, tt.miso) {
				t.Errorf("misoFlags = %v, want %v", miso, tt.miso)
			}
			if !reflect.DeepEqual(turbo, tt.turbo) {
				t.Errorf("turboFlags = %v, want %v", turbo, tt.turbo)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/miso && go test ./internal/turbo/ -v -run TestSplitFlags`
Expected: FAIL — `SplitFlags` not defined

- [ ] **Step 3: Implement `SplitFlags`**

In `apps/miso/internal/turbo/flags.go`:

```go
package turbo

import "strings"

// misoFlags is the canonical set of flags owned by miso.
// Everything else is passed through to turbo.
var misoFlags = map[string]bool{
	"--env": true,
}

// SplitFlags separates miso-owned flags from turbo passthrough flags.
// When tuiActive is true, --log-order is stripped (miso controls it for output parsing).
// Handles both --flag=value and --flag value forms.
func SplitFlags(args []string, tuiActive bool) (miso []string, turbo []string) {
	skip := false
	for i, arg := range args {
		if skip {
			skip = false
			continue
		}

		flag := arg
		hasEquals := strings.Contains(arg, "=")
		if hasEquals {
			flag = arg[:strings.Index(arg, "=")]
		}

		if misoFlags[flag] {
			miso = append(miso, arg)
			continue
		}

		// Strip --log-order when TUI is active (both --log-order=val and --log-order val)
		if tuiActive && flag == "--log-order" {
			if !hasEquals && i+1 < len(args) {
				skip = true // consume next arg too
			}
			continue
		}

		turbo = append(turbo, arg)
	}
	return miso, turbo
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd apps/miso && go test ./internal/turbo/ -v -run TestSplitFlags`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add apps/miso/internal/turbo/flags.go apps/miso/internal/turbo/flags_test.go
git commit -m "feat(turbo): add SplitFlags for transparent flag passthrough"
```

---

## Chunk 2: `turbo` package — config parsing

### Task 3: Create `turbo.LoadConfig` with tests

**Files:**
- Create: `apps/miso/internal/turbo/config.go`
- Create: `apps/miso/internal/turbo/config_test.go`

- [ ] **Step 1: Write failing tests for `LoadConfig`**

In `apps/miso/internal/turbo/config_test.go`:

```go
package turbo

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTurboJSON(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "turbo.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write turbo.json: %v", err)
	}
	return dir
}

func TestLoadConfigV2(t *testing.T) {
	dir := writeTurboJSON(t, `{
		"tasks": {
			"build": { "dependsOn": ["^build"], "outputs": ["dist/**"] },
			"dev": { "cache": false, "persistent": true },
			"lint": {}
		}
	}`)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if cfg.Version != 2 {
		t.Errorf("Version = %d, want 2", cfg.Version)
	}
	if len(cfg.Tasks) != 3 {
		t.Fatalf("len(Tasks) = %d, want 3", len(cfg.Tasks))
	}

	build := cfg.Tasks["build"]
	if len(build.DependsOn) != 1 || build.DependsOn[0] != "^build" {
		t.Errorf("build.DependsOn = %v, want [^build]", build.DependsOn)
	}
	if len(build.Outputs) != 1 || build.Outputs[0] != "dist/**" {
		t.Errorf("build.Outputs = %v, want [dist/**]", build.Outputs)
	}

	dev := cfg.Tasks["dev"]
	if dev.Cache == nil || *dev.Cache != false {
		t.Errorf("dev.Cache = %v, want false", dev.Cache)
	}
	if !dev.Persistent {
		t.Error("dev.Persistent = false, want true")
	}

	names := cfg.TaskNames()
	if len(names) != 3 {
		t.Errorf("TaskNames() len = %d, want 3", len(names))
	}
}

func TestLoadConfigV1(t *testing.T) {
	dir := writeTurboJSON(t, `{
		"pipeline": {
			"build": { "dependsOn": ["^build"] },
			"dev": { "cache": false }
		}
	}`)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("Version = %d, want 1", cfg.Version)
	}
	if len(cfg.Tasks) != 2 {
		t.Fatalf("len(Tasks) = %d, want 2", len(cfg.Tasks))
	}
}

func TestLoadConfigNotFound(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v, want nil for missing file", err)
	}
	if len(cfg.Tasks) != 0 {
		t.Errorf("Tasks = %v, want empty", cfg.Tasks)
	}
}

func TestLoadConfigMalformed(t *testing.T) {
	dir := writeTurboJSON(t, `{not valid json}`)
	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("LoadConfig() expected error for malformed JSON, got nil")
	}
}

func TestLoadConfigNoTasksOrPipeline(t *testing.T) {
	dir := writeTurboJSON(t, `{ "globalDependencies": [".env"] }`)
	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("LoadConfig() expected error for missing tasks/pipeline, got nil")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/miso && go test ./internal/turbo/ -v -run TestLoadConfig`
Expected: FAIL — `LoadConfig` not defined

- [ ] **Step 3: Implement `LoadConfig`**

In `apps/miso/internal/turbo/config.go`:

```go
package turbo

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// TurboConfig holds parsed turbo.json configuration.
type TurboConfig struct {
	Version int                  // 2 or 1
	Tasks   map[string]TurboTask
}

// TurboTask holds per-task configuration from turbo.json.
type TurboTask struct {
	DependsOn  []string
	Cache      *bool // nil = default (true), explicit true/false
	Persistent bool
	Outputs    []string
	Env        []string
}

// TaskNames returns a sorted list of all task names defined in turbo.json.
func (c TurboConfig) TaskNames() []string {
	names := make([]string, 0, len(c.Tasks))
	for name := range c.Tasks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// turboJSON is the raw shape of turbo.json for unmarshaling.
type turboJSON struct {
	Tasks    map[string]turboTaskJSON `json:"tasks"`
	Pipeline map[string]turboTaskJSON `json:"pipeline"`
}

type turboTaskJSON struct {
	DependsOn  []string `json:"dependsOn,omitempty"`
	Cache      *bool    `json:"cache,omitempty"`
	Persistent bool     `json:"persistent,omitempty"`
	Outputs    []string `json:"outputs,omitempty"`
	Env        []string `json:"env,omitempty"`
}

// LoadConfig reads turbo.json from the project root.
// Returns an empty TurboConfig if turbo.json does not exist.
// Returns an error if the file exists but is malformed or has neither tasks nor pipeline.
func LoadConfig(root string) (TurboConfig, error) {
	path := filepath.Join(root, "turbo.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return TurboConfig{Tasks: map[string]TurboTask{}}, nil
	}
	if err != nil {
		return TurboConfig{}, fmt.Errorf("read turbo.json: %w", err)
	}

	var raw turboJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return TurboConfig{}, fmt.Errorf("parse turbo.json: %w", err)
	}

	var version int
	var rawTasks map[string]turboTaskJSON

	if raw.Tasks != nil {
		version = 2
		rawTasks = raw.Tasks
	} else if raw.Pipeline != nil {
		version = 1
		rawTasks = raw.Pipeline
	} else {
		return TurboConfig{}, fmt.Errorf("turbo.json: neither 'tasks' nor 'pipeline' key found")
	}

	tasks := make(map[string]TurboTask, len(rawTasks))
	for name, rt := range rawTasks {
		tasks[name] = TurboTask{
			DependsOn:  rt.DependsOn,
			Cache:      rt.Cache,
			Persistent: rt.Persistent,
			Outputs:    rt.Outputs,
			Env:        rt.Env,
		}
	}

	return TurboConfig{Version: version, Tasks: tasks}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd apps/miso && go test ./internal/turbo/ -v -run TestLoadConfig`
Expected: PASS

- [ ] **Step 5: Run all turbo package tests**

Run: `cd apps/miso && go test ./internal/turbo/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add apps/miso/internal/turbo/config.go apps/miso/internal/turbo/config_test.go
git commit -m "feat(turbo): add LoadConfig for turbo.json parsing (v1 + v2)"
```

---

## Chunk 3: Wire up delegate.go and main.go

### Task 4: Refactor `delegate.go` to use `turbo` package

**Files:**
- Modify: `apps/miso/internal/tui/delegate.go`
- Modify: `apps/miso/internal/tui/delegate_test.go`

This task replaces inline parsing logic with calls to `turbo.ParseLine`, adds `extraArgs` parameter, per-task exit codes, and fixes the scanner drain race.

- [ ] **Step 1: Update `DelegateLaunch` signature and imports**

In `apps/miso/internal/tui/delegate.go`:

Change the function signature from:
```go
func DelegateLaunch(cfg config.Config, scriptName string, root string) (bool, error) {
```
to:
```go
func DelegateLaunch(cfg config.Config, scriptName string, root string, extraArgs []string) (bool, error) {
```

Add `"sync"` and `"github.com/ekkolyth/miso/internal/turbo"` to imports.

Remove the `turboLineRe` and `nxHeaderRe` regex vars, and the `parseTurboLine` and `parseNxHeader` functions (they now live in the `turbo` package).

- [ ] **Step 2: Append `extraArgs` to turbo command**

Change the turbo args construction from:
```go
delegateArgs = []string{"run", scriptName, "--log-order=stream"}
```
to:
```go
delegateArgs = append([]string{"run", scriptName, "--log-order=stream"}, extraArgs...)
```

- [ ] **Step 3: Replace inline parsing with `turbo.ParseLine`**

Replace the `parseLine` closure and `routeLine` closure with new versions. The `parseLine` closure for turbo mode should call `turbo.ParseLine(line)` and return the `LineMeta`. The `routeLine` function should handle `IsExit` by updating per-process exit codes:

```go
// routeBasic handles the simple (label, text) routing used by nx and as a fallback.
routeBasic := func(label, text string) {
    if label == "" {
        return
    }
    proc := pm.findProc(label)
    if proc == nil {
        entry := TuiScriptEntry{Label: label, ScriptName: scriptName, WorkspaceDir: root}
        proc = pm.Add(entry, "", nil, root)
        proc.State = StateRunning
        pm.sendState(proc, StateRunning, 0)
    }
    proc.Buffer.Write(text)
    pm.sendOutput(proc, text)
}

// routeTurbo handles turbo output with per-task exit codes and cache metadata.
routeTurbo := func(meta turbo.LineMeta) {
    if meta.Skip || meta.Label == "" {
        return
    }
    proc := pm.findProc(meta.Label)
    if proc == nil {
        entry := TuiScriptEntry{Label: meta.Label, ScriptName: scriptName, WorkspaceDir: root}
        proc = pm.Add(entry, "", nil, root)
        proc.State = StateRunning
        pm.sendState(proc, StateRunning, 0)
    }
    if meta.IsExit {
        proc.mu.Lock()
        proc.State = StateExited
        proc.ExitCode = meta.ExitCode
        proc.mu.Unlock()
        pm.sendState(proc, StateExited, meta.ExitCode)
        return
    }
    proc.Buffer.Write(meta.Text)
    pm.sendOutput(proc, meta.Text)
}
```

- [ ] **Step 4: Fix scanner drain race with WaitGroup**

In the goroutine that starts the delegated process, add a `sync.WaitGroup` for the stdout/stderr scanner goroutines. Wait for them to drain before calling `cmd.Wait()`:

```go
go func() {
    if err := cmd.Start(); err != nil {
        fmt.Fprintf(os.Stderr, "miso: failed to start %s: %v\n", mode, err)
        p.Quit()
        return
    }

    var scanWg sync.WaitGroup
    scanWg.Add(2)

    scanPipe := func(r interface{ Read([]byte) (int, error) }) {
        defer scanWg.Done()
        scanner := bufio.NewScanner(r)
        currentNxLabel := ""
        for scanner.Scan() {
            line := stripNonColorANSI(scanner.Text())
            switch mode {
            case "turbo":
                routeTurbo(turbo.ParseLine(line))
            case "nx":
                if hdrLabel, isHdr := parseNxHeader(line); isHdr {
                    currentNxLabel = hdrLabel
                    continue
                }
                if currentNxLabel != "" {
                    routeBasic(currentNxLabel, line)
                }
            }
        }
    }

    go scanPipe(stdout)
    go scanPipe(stderr)

    // Drain scanners before waiting for process exit
    scanWg.Wait()

    exitErr := cmd.Wait()
    code := 0
    if exitErr != nil {
        if exitError, ok := exitErr.(*exec.ExitError); ok {
            code = exitError.ExitCode()
        } else {
            code = -1
        }
    }

    // Fallback: assign exit code only to processes that didn't get individual codes
    pm.mu.Lock()
    for _, proc := range pm.Processes {
        if proc.State != StateExited {
            proc.State = StateExited
            proc.ExitCode = code
        }
    }
    pm.mu.Unlock()
    for _, proc := range pm.Processes {
        pm.sendState(proc, StateExited, proc.ExitCode)
    }
}()
```

- [ ] **Step 5: Update delegate_test.go**

Remove `TestParseTurboLine` and `TestParseNxHeader` from `apps/miso/internal/tui/delegate_test.go` since the turbo parsing tests now live in `apps/miso/internal/turbo/output_test.go`. Keep the nx header test in `delegate_test.go` since nx parsing stays inline for now. If the file becomes empty (only turbo tests existed), remove it.

- [ ] **Step 6: Verify tui package compiles**

Run: `cd apps/miso && go build ./internal/tui/...`
Expected: Compiles (main.go will fail due to changed `DelegateLaunch` signature — fixed in Task 5)

- [ ] **Step 7: Commit**

```bash
git add apps/miso/internal/tui/delegate.go apps/miso/internal/tui/delegate_test.go
git commit -m "refactor: delegate.go uses turbo package, per-task exit codes, scanner drain fix"
```

---

### Task 5: Wire up flag passthrough in `main.go`

**Files:**
- Modify: `apps/miso/cmd/main.go`

- [ ] **Step 1: Add turbo import and flag splitting**

In `apps/miso/cmd/main.go`, add `"github.com/ekkolyth/miso/internal/turbo"` to imports.

Change the delegated TUI interception block (around line 186) from:
```go
if cfg.IsDelegated() {
    ran, err := tui.DelegateLaunch(cfg, scriptName, projectRoot)
```
to:
```go
if cfg.IsDelegated() {
    _, turboFlags := turbo.SplitFlags(parsed.ScriptArgs, cfg.TuiEnabled())
    ran, err := tui.DelegateLaunch(cfg, scriptName, projectRoot, turboFlags)
```

- [ ] **Step 2: Verify build and all tests pass**

Run: `cd apps/miso && go build ./... && go test ./...`
Expected: Compiles and all tests PASS

- [ ] **Step 3: Commit**

```bash
git add apps/miso/cmd/main.go
git commit -m "feat: wire turbo flag passthrough through CLI to DelegateLaunch"
```

---

## Chunk 4: `miso scripts` turbo.json integration

### Task 6: Add turbo.json tasks to `miso scripts` and tab-completion

**Files:**
- Modify: `apps/miso/internal/cli/scripting/scripts.go`
- Create: `apps/miso/internal/cli/scripting/scripts_test.go`

- [ ] **Step 1: Write failing test for `ListNames` with turbo.json**

In `apps/miso/internal/cli/scripting/scripts_test.go`:

```go
package scripting

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ekkolyth/miso/internal/config"
)

func TestListNamesIncludesTurboTasks(t *testing.T) {
	dir := t.TempDir()

	// Write a turbo.json with tasks
	turboJSON := `{"tasks": {"build": {}, "lint": {}, "test": {}}}`
	if err := os.WriteFile(filepath.Join(dir, "turbo.json"), []byte(turboJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write a package.json with one overlapping script
	pkgJSON := `{"scripts": {"dev": "turbo dev", "build": "turbo run build"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{Repo: "turbo"}

	names, err := ListNames(dir, cfg)
	if err != nil {
		t.Fatalf("ListNames() error: %v", err)
	}

	// Should include: build (from pkg, not duplicated from turbo), dev (from pkg), lint (from turbo), test (from turbo)
	want := map[string]bool{"build": true, "dev": true, "lint": true, "test": true}
	got := make(map[string]bool)
	for _, n := range names {
		got[n] = true
	}
	for name := range want {
		if !got[name] {
			t.Errorf("missing expected name %q in ListNames result: %v", name, names)
		}
	}
	// Verify no duplicates
	if len(names) != len(got) {
		t.Errorf("ListNames has duplicates: %v", names)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/miso && go test ./internal/cli/scripting/ -v -run TestListNamesIncludesTurboTasks`
Expected: FAIL — turbo tasks not included yet

- [ ] **Step 3: Update `List()` to show turbo.json tasks**

In `apps/miso/internal/cli/scripting/scripts.go`, add `"github.com/ekkolyth/miso/internal/turbo"` to imports.

Replace the existing "no scripts found" check (line 43-46):
```go
if !hasFolderScripts && !hasPkgScripts {
    logger.Info("no scripts found in scripts folder or package.json")
    return nil
}
```

With turbo-aware version:
```go
hasTurboTasks := false
if cfg.IsDelegated() && cfg.RepoMode() == "turbo" {
    turboCfg, turboErr := turbo.LoadConfig(root)
    if turboErr == nil && len(turboCfg.Tasks) > 0 {
        hasTurboTasks = true
    }
}

if !hasFolderScripts && !hasPkgScripts && !hasTurboTasks {
    logger.Info("no scripts found in scripts folder, package.json, or turbo.json")
    return nil
}
```

Then insert the following **after** the package.json display block (after line 79's closing `}`), **before** `return nil`:
```go
// show turbo.json tasks (only in turbo mode, deduplicated)
if cfg.IsDelegated() && cfg.RepoMode() == "turbo" {
    turboCfg, turboErr := turbo.LoadConfig(root)
    if turboErr == nil && len(turboCfg.Tasks) > 0 {
        existingNames := make(map[string]bool)
        for name := range folderScripts {
            existingNames[name] = true
        }
        for name := range pkgScripts {
            existingNames[name] = true
        }

        var turboNames []string
        for _, name := range turboCfg.TaskNames() {
            if !existingNames[name] {
                turboNames = append(turboNames, name)
            }
        }

        if len(turboNames) > 0 {
            fmt.Fprintf(os.Stdout, "\nturbo.json:\n")
            for _, name := range turboNames {
                fmt.Fprintf(os.Stdout, "  %s\n", name)
            }
        }
    }
}
```

- [ ] **Step 4: Update `ListNames()` to include turbo.json tasks**

In `ListNames()`, insert the following **after** the `for name := range pkgScripts` loop (after line 119's closing `}`), **before** `sort.Strings(names)`:

```go
if cfg.IsDelegated() && cfg.RepoMode() == "turbo" {
    turboCfg, turboErr := turbo.LoadConfig(root)
    if turboErr == nil {
        for _, name := range turboCfg.TaskNames() {
            if !seen[name] {
                seen[name] = true
                names = append(names, name)
            }
        }
    }
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd apps/miso && go test ./internal/cli/scripting/ -v -run TestListNamesIncludesTurboTasks`
Expected: PASS

- [ ] **Step 6: Run full build and test suite**

Run: `cd apps/miso && go build ./... && go test ./...`
Expected: Compiles and all tests PASS

- [ ] **Step 7: Commit**

```bash
git add apps/miso/internal/cli/scripting/scripts.go apps/miso/internal/cli/scripting/scripts_test.go
git commit -m "feat: show turbo.json tasks in miso scripts and tab-completion"
```

---

## Chunk 5: Documentation

### Task 7: Write turbo documentation pages

**Files:**
- Create: `apps/docs/content/turbo/index.mdx`
- Create: `apps/docs/content/turbo/flag-passthrough.mdx`
- Create: `apps/docs/content/turbo/task-discovery.mdx`
- Modify: `apps/docs/content/tui/configuration.mdx`

- [ ] **Step 1: Check docs meta config for navigation**

Read `apps/docs/content/_meta.tsx` to understand how to add a new section to the docs navigation. The turbo section needs to be registered there.

- [ ] **Step 2: Create `turbo/index.mdx`**

```mdx
---
title: Turborepo Integration
description: Using miso with Turborepo for monorepo orchestration
---

# Turborepo Integration

Miso integrates with Turborepo by delegating orchestration while providing an enhanced terminal UI.

## Setup

Set `repo` to `"turbo"` in your `miso.json`:

```json
{
  "repo": "turbo",
  "tui": "tabbed"
}
```

**Requirements:** The `turbo` binary must be available in your PATH.

## How It Works

When you run `miso dev` or `miso build` with turbo mode:

1. Miso spawns `turbo run <task>` as a single process
2. Turbo handles dependency ordering, caching, and parallelism
3. Miso parses the streamed output and displays it in the TUI with per-workspace tabs
4. Each workspace gets its own tab with independent scroll, exit code, and cache status

### With TUI enabled

Miso wraps turbo's output in the tabbed or merged view. Workspaces are discovered dynamically as turbo's output arrives. Per-workspace exit codes are parsed from turbo's output — if workspace A succeeds and workspace B fails, each tab shows the correct status.

### With TUI disabled

Miso stays invisible. Your command falls through to the package manager, which runs the root `package.json` script (e.g., `"dev": "turbo dev"`). There is no difference from running without miso.

## What Miso Delegates vs Handles

| Concern | Owner |
|---------|-------|
| Task dependency ordering | Turbo |
| Caching | Turbo |
| Parallelism | Turbo |
| Output display (TUI) | Miso |
| Per-workspace exit codes | Miso (parsed from turbo output) |
| Flag passthrough | Miso (transparent) |
| Task discovery (`miso scripts`) | Miso (reads turbo.json) |
```

- [ ] **Step 3: Create `turbo/flag-passthrough.mdx`**

```mdx
---
title: Flag Passthrough
description: How miso forwards flags to turbo transparently
---

# Flag Passthrough

Any flag that miso doesn't own is forwarded to turbo transparently. No `--` separator needed.

## Examples

```bash
# Filter to a specific workspace
miso dev --filter=web

# Limit concurrency
miso build --concurrency=2

# Force re-execution (skip cache)
miso build --force

# Combine flags
miso build --filter=web --force --concurrency=1
```

These all translate directly to turbo flags:
```bash
turbo run dev --filter=web
turbo run build --concurrency=2
# etc.
```

## Miso-Owned Flags

Miso only claims the following flags — everything else passes through:

| Flag | Purpose |
|------|---------|
| `--env` | Run env validation before the command |

## `--log-order` Behavior

When the TUI is active, miso overrides `--log-order` to `stream`. This is required for miso to parse turbo's output and route it to the correct workspace tabs.

When the TUI is disabled (`tui: "off"`), any `--log-order` value you pass is forwarded to turbo as-is.

If you pass `--log-order=grouped` while the TUI is active, miso silently strips it. This is intentional — grouped output cannot be parsed into per-workspace tabs.
```

- [ ] **Step 4: Create `turbo/task-discovery.mdx`**

```mdx
---
title: Task Discovery
description: How miso discovers tasks from turbo.json
---

# Task Discovery

When using turbo mode, `miso scripts` shows tasks from three sources with clear precedence:

1. **Scripts folder** (highest priority) — files in your `scripts/` directory
2. **package.json** — scripts defined in root `package.json`
3. **turbo.json** — tasks defined in `turbo.json` (only shown if not already covered by the above)

## Example

Given this turbo.json:
```json
{
  "tasks": {
    "build": { "dependsOn": ["^build"] },
    "dev": { "cache": false, "persistent": true },
    "lint": {},
    "test": {}
  }
}
```

And a root package.json with `"dev": "turbo dev"`, and a `scripts/setup.sh` file:

```
$ miso scripts

scripts folder:
  setup (scripts/setup.sh)

package.json:
  dev: turbo dev

turbo.json:
  build
  lint
  test
```

Note that `dev` appears under package.json (higher precedence), not turbo.json.

## Version Support

Miso supports both turbo.json formats:

- **v2** (current): reads from the `tasks` key
- **v1** (legacy): reads from the `pipeline` key

Version is detected automatically.
```

- [ ] **Step 5: Update `tui/configuration.mdx` with `cleanExit` option**

In `apps/docs/content/tui/configuration.mdx`, update the `## \`tui\` Option` section. After the existing string form example, add:

```mdx
### Object Form

For additional options, use the object form:

```json
{
  "tui": {
    "mode": "tabbed",
    "cleanExit": true
  }
}
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `mode` | string | `"off"` | TUI display mode: `"off"`, `"tabbed"`, or `"merged"` |
| `cleanExit` | boolean | `false` | When `true`, suppress log dump on TUI exit. By default, all buffered output is printed to stdout when the TUI exits. |
```

- [ ] **Step 6: Update `_meta.tsx` to add turbo section**

In `apps/docs/content/_meta.tsx`, add the turbo entry after the tui entry:

```tsx
export default {
    index: 'Welcome',
    install: 'Installation',
    'working-with-miso': 'Working with Miso',
    'env-validation': 'Env Validation',
    scripting: 'Scripting',
    tui: 'Terminal UI (TUI)',
    turbo: 'Turborepo Integration',
    contributing: 'Contributing to Miso',
}
```

Also create `apps/docs/content/turbo/_meta.ts`:

```ts
export default {
    index: 'Overview',
    'flag-passthrough': 'Flag Passthrough',
    'task-discovery': 'Task Discovery',
}
```

- [ ] **Step 7: Commit**

```bash
git add apps/docs/content/turbo/ apps/docs/content/tui/configuration.mdx apps/docs/content/_meta.tsx
git commit -m "docs: add turbo integration docs and cleanExit config documentation"
```

---

## Chunk 6: Final verification

### Task 8: Full build, test, and manual smoke test

- [ ] **Step 1: Run full build**

Run: `cd apps/miso && go build ./...`
Expected: Clean compile

- [ ] **Step 2: Run full test suite**

Run: `cd apps/miso && go test ./...`
Expected: All PASS

- [ ] **Step 3: Verify no regressions in existing turbo parsing**

Run: `cd apps/miso && go test ./internal/turbo/ -v`
Expected: All turbo package tests PASS

- [ ] **Step 4: Verify config tests still pass**

Run: `cd apps/miso && go test ./internal/config/ -v`
Expected: All PASS (including the new tui object form tests from earlier)
