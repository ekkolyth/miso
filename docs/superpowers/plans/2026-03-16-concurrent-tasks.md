# Concurrent Tasks Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `concurrent` field to `TaskConfig` so companion tasks (e.g., `services`, `db:studio`) are discovered and run alongside the main task in the TUI, replacing the old `multi` field.

**Architecture:** `concurrent` is a string array on `TaskConfig` under `repo.tasks`. During `discoverEntries`, after discovering the main task's scripts, each concurrent name is discovered using the same resolution path (monorepo: `DiscoverTuiScripts`; single-repo: `scripting.ResolveScript`). A post-merge label dedup pass prevents collisions. Concurrent entries start immediately in the TUI, bypassing any `dependsOn` ordering.

**Tech Stack:** Go, bubbletea TUI framework, JSON schema

**Spec:** `docs/superpowers/specs/2026-03-16-concurrent-tasks-design.md`

---

## Chunk 1: Config Layer

### Task 1: Add `Concurrent` to `TaskConfig` and `TaskConcurrent()` helper

**Files:**
- Modify: `apps/miso/internal/config/config.go:20-23` (TaskConfig struct)
- Modify: `apps/miso/internal/config/config.go:142-158` (add new method after HasDependsOn)

- [ ] **Step 1: Write the failing test for `TaskConcurrent`**

Add to `apps/miso/internal/config/config_test.go`:

```go
func TestTaskConcurrent(t *testing.T) {
	cfg := Config{
		Tasks: map[string]TaskConfig{
			"dev": {Concurrent: []string{"services", "db:studio"}},
			"build": {},
		},
	}

	got := cfg.TaskConcurrent("dev")
	if len(got) != 2 || got[0] != "services" || got[1] != "db:studio" {
		t.Errorf("TaskConcurrent(\"dev\") = %v, want [services db:studio]", got)
	}

	if got := cfg.TaskConcurrent("build"); got != nil {
		t.Errorf("TaskConcurrent(\"build\") = %v, want nil", got)
	}

	if got := cfg.TaskConcurrent("unknown"); got != nil {
		t.Errorf("TaskConcurrent(\"unknown\") = %v, want nil", got)
	}

	nilCfg := Config{}
	if got := nilCfg.TaskConcurrent("dev"); got != nil {
		t.Errorf("TaskConcurrent on nil Tasks = %v, want nil", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/miso && go test ./internal/config/ -run TestTaskConcurrent -v`
Expected: FAIL — `Concurrent` field and `TaskConcurrent` method don't exist yet.

- [ ] **Step 3: Add `Concurrent` field to `TaskConfig` and implement `TaskConcurrent`**

In `apps/miso/internal/config/config.go`, update `TaskConfig` (line 20-23):

```go
// TaskConfig holds per-task configuration for dependency ordering and concurrent companions.
type TaskConfig struct {
	DependsOn  []string `json:"dependsOn,omitempty"`
	Concurrent []string `json:"concurrent,omitempty"`
}
```

Add after `HasDependsOn` method (after line 158):

```go
// TaskConcurrent returns the concurrent task names for the given command, or nil.
func (c Config) TaskConcurrent(command string) []string {
	if c.Tasks == nil {
		return nil
	}
	task, ok := c.Tasks[command]
	if !ok {
		return nil
	}
	if len(task.Concurrent) == 0 {
		return nil
	}
	return task.Concurrent
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/miso && go test ./internal/config/ -run TestTaskConcurrent -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add apps/miso/internal/config/config.go apps/miso/internal/config/config_test.go
git commit -m "feat: add Concurrent field to TaskConfig and TaskConcurrent helper"
```

### Task 2: Remove `Multi` from config and allow `tasks` in all modes

**Files:**
- Modify: `apps/miso/internal/config/config.go:26-37` (Config struct)
- Modify: `apps/miso/internal/config/config.go:170-180` (configLoad struct)
- Modify: `apps/miso/internal/config/config.go:208-216` (Load function)
- Modify: `apps/miso/internal/config/config.go:276-295` (parseRepoField)
- Modify: `apps/miso/internal/config/config_test.go`

- [ ] **Step 1: Write tests for concurrent parsing and tasks-in-all-modes**

Add to `apps/miso/internal/config/config_test.go`:

```go
func TestLoadRepoObjectWithConcurrent(t *testing.T) {
	dir := writeTempConfig(t, `{
		"repo": {
			"mode": "turbo",
			"tasks": {
				"dev": { "concurrent": ["services", "db:studio"] }
			}
		}
	}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Tasks == nil {
		t.Fatal("Tasks is nil, want populated map")
	}
	dev, ok := cfg.Tasks["dev"]
	if !ok {
		t.Fatal("Tasks[\"dev\"] missing")
	}
	if len(dev.Concurrent) != 2 || dev.Concurrent[0] != "services" || dev.Concurrent[1] != "db:studio" {
		t.Errorf("Tasks[\"dev\"].Concurrent = %v, want [services db:studio]", dev.Concurrent)
	}
}

func TestLoadRepoObjectSingleWithTasks(t *testing.T) {
	dir := writeTempConfig(t, `{
		"repo": {
			"mode": "single",
			"tasks": {
				"dev": { "concurrent": ["frontend", "backend"] }
			}
		}
	}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v, want success (tasks valid in all modes)", err)
	}
	if cfg.Repo != "single" {
		t.Errorf("Repo = %q, want %q", cfg.Repo, "single")
	}
	if cfg.Tasks == nil {
		t.Fatal("Tasks is nil, want populated map")
	}
}

func TestLoadRepoObjectNxWithTasks(t *testing.T) {
	dir := writeTempConfig(t, `{
		"repo": {
			"mode": "nx",
			"tasks": { "dev": { "concurrent": ["api:serve"] } }
		}
	}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v, want success (tasks valid in nx mode)", err)
	}
	if cfg.Repo != "nx" {
		t.Errorf("Repo = %q, want %q", cfg.Repo, "nx")
	}
}
```

- [ ] **Step 2: Run new tests plus the existing invalid-mode test to see what fails**

Run: `cd apps/miso && go test ./internal/config/ -run "TestLoadRepoObject" -v`
Expected: `TestLoadRepoObjectWithConcurrent` PASS (concurrent parses via JSON), `TestLoadRepoObjectSingleWithTasks` FAIL (mode restriction), `TestLoadRepoObjectNxWithTasks` FAIL (mode restriction), `TestLoadRepoObjectInvalidMode` PASS (still expects error).

- [ ] **Step 3: Remove `Multi` from structs, remove mode restriction, add `multi` deprecation warning**

In `apps/miso/internal/config/config.go`:

**Config struct (line 36):** Delete the `Multi` line:
```go
Multi          map[string][]string   `json:"multi,omitempty"`
```

**configLoad struct (line 179):** Delete the `Multi` line:
```go
Multi          map[string][]string `json:"multi,omitempty"`
```

**Load function (line 215):** Delete `Multi: load.Multi,` from the cfg assignment.

After the `json.Unmarshal(data, &load)` call (after line 196), add the `multi` deprecation check:

```go
	// Warn if deprecated 'multi' key is present in the raw JSON.
	var rawKeys map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawKeys); err == nil {
		if _, hasMulti := rawKeys["multi"]; hasMulti {
			fmt.Fprintf(os.Stderr, "warning: 'multi' config is no longer supported — use repo.tasks with 'concurrent' instead\n")
		}
	}
```

**parseRepoField (lines 290-292):** Delete the entire mode restriction block:
```go
	if obj.Tasks != nil && obj.Mode != "mono" && obj.Mode != "turbo" {
		return "", nil, fmt.Errorf("repo.tasks is only valid when mode is \"mono\" or \"turbo\", got %q", obj.Mode)
	}
```

- [ ] **Step 4: Update `TestLoadRepoObjectInvalidMode` to expect success**

Replace the entire `TestLoadRepoObjectInvalidMode` function in `config_test.go`:

```go
func TestLoadRepoObjectSingleWithDependsOn(t *testing.T) {
	dir := writeTempConfig(t, `{
		"repo": {
			"mode": "single",
			"tasks": { "build": { "dependsOn": ["^build"] } }
		}
	}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v, want success (tasks valid in all modes)", err)
	}
	if cfg.Tasks == nil {
		t.Fatal("Tasks is nil")
	}
}
```

- [ ] **Step 5: Update `TestLoadTuiConfig` to remove `multi` assertions**

Replace `TestLoadTuiConfig` in `config_test.go`:

```go
func TestLoadTuiConfig(t *testing.T) {
	dir := writeTempConfig(t, `{
		"tui": "tabbed"
	}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.TuiMode != "tabbed" {
		t.Errorf("Tui = %q, want %q", cfg.TuiMode, "tabbed")
	}
}
```

- [ ] **Step 6: Update `TestLoadTuiConfigDefaults` to remove `multi` assertion**

Remove lines 71-73 from `TestLoadTuiConfigDefaults` (the `cfg.Multi` check).

- [ ] **Step 7: Run all config tests**

Run: `cd apps/miso && go test ./internal/config/ -v`
Expected: All PASS

- [ ] **Step 8: Commit**

```bash
git add apps/miso/internal/config/config.go apps/miso/internal/config/config_test.go
git commit -m "feat: remove multi config, allow tasks in all repo modes"
```

---

## Chunk 2: Discovery and Launch Layer

### Task 3: Add label deduplication and single-repo concurrent discovery

**Files:**
- Modify: `apps/miso/internal/tui/discover.go:144-174` (replace DiscoverMultiScripts)
- Modify: `apps/miso/internal/tui/discover_test.go`

- [ ] **Step 1: Write failing test for `DeduplicateLabels`**

Add to `apps/miso/internal/tui/discover_test.go`:

```go
func TestDeduplicateLabels(t *testing.T) {
	entries := []TuiScriptEntry{
		{Label: "app", ScriptName: "dev", WorkspaceDir: "/ws/app"},
		{Label: "app", ScriptName: "services", WorkspaceDir: "/ws/app"},
		{Label: "docker", ScriptName: "services", WorkspaceDir: "/ws/docker"},
	}

	result := DeduplicateLabels(entries)

	labels := labelsOf(result)
	// "app" appears twice with different scripts → disambiguate both
	// "docker" is unique → stays as-is
	expected := map[string]bool{
		"app:dev":      true,
		"app:services": true,
		"docker":       true,
	}
	if len(labels) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(labels), labels)
	}
	for _, l := range labels {
		if !expected[l] {
			t.Errorf("unexpected label %q, expected one of %v", l, expected)
		}
	}
}

func TestDeduplicateLabels_NoDuplicates(t *testing.T) {
	entries := []TuiScriptEntry{
		{Label: "app", ScriptName: "dev", WorkspaceDir: "/ws/app"},
		{Label: "api", ScriptName: "dev", WorkspaceDir: "/ws/api"},
	}

	result := DeduplicateLabels(entries)
	labels := labelsOf(result)
	if labels[0] != "app" || labels[1] != "api" {
		t.Errorf("labels changed unexpectedly: %v", labels)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/miso && go test ./internal/tui/ -run TestDeduplicateLabels -v`
Expected: FAIL — `DeduplicateLabels` doesn't exist yet.

- [ ] **Step 3: Implement `DeduplicateLabels` and `ResolveSingleRepoScripts`**

Replace `DiscoverMultiScripts` in `apps/miso/internal/tui/discover.go` (lines 144-174) with:

```go
// DeduplicateLabels ensures all labels in the merged entry list are unique.
// For any label that appears more than once, it rewrites to "label:scriptName".
func DeduplicateLabels(entries []TuiScriptEntry) []TuiScriptEntry {
	// Count label occurrences
	counts := make(map[string]int)
	for _, e := range entries {
		counts[e.Label]++
	}

	// Rewrite duplicates
	for i := range entries {
		if counts[entries[i].Label] > 1 {
			entries[i].Label = entries[i].Label + ":" + entries[i].ScriptName
		}
	}

	return entries
}

// ResolveSingleRepoScripts resolves a list of script names against the project
// root for single-repo concurrent discovery. Scripts that cannot be found are
// silently skipped. Labels are the script names.
func ResolveSingleRepoScripts(scripts []string, root string, cfg config.Config) ([]TuiScriptEntry, error) {
	var entries []TuiScriptEntry

	for _, name := range scripts {
		resolved, err := scripting.ResolveScript(name, root, cfg)
		if err != nil {
			return nil, err
		}
		if resolved.Source == scripting.ScriptSourceNone {
			continue // silently skip unresolvable scripts
		}

		source := ""
		switch resolved.Source {
		case scripting.ScriptSourceFolder:
			source = "folder"
		case scripting.ScriptSourcePackageJSON:
			source = "packagejson"
		}

		entries = append(entries, TuiScriptEntry{
			Label:        name,
			ScriptName:   name,
			WorkspaceDir: root,
			ScriptSource: source,
			ScriptPath:   resolved.Path,
		})
	}

	return entries, nil
}
```

- [ ] **Step 4: Write test for `ResolveSingleRepoScripts`**

Replace `TestDiscoverTuiScripts_MultiConfig` in `discover_test.go`:

```go
func TestResolveSingleRepoScripts(t *testing.T) {
	root := t.TempDir()
	writePackageJSON(t, root, map[string]string{
		"dev":   "vite",
		"build": "tsc",
	})

	cfg := config.Config{
		Scripts: "./scripts",
	}

	entries, err := ResolveSingleRepoScripts([]string{"dev", "build"}, root, cfg)
	if err != nil {
		t.Fatalf("ResolveSingleRepoScripts: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	for _, e := range entries {
		if e.Label != e.ScriptName {
			t.Errorf("label %q != script name %q", e.Label, e.ScriptName)
		}
		if e.ScriptSource != "packagejson" {
			t.Errorf("%q: expected source 'packagejson', got %q", e.Label, e.ScriptSource)
		}
	}
}

func TestResolveSingleRepoScripts_SkipsMissing(t *testing.T) {
	root := t.TempDir()
	writePackageJSON(t, root, map[string]string{
		"dev": "vite",
	})

	cfg := config.Config{
		Scripts: "./scripts",
	}

	entries, err := ResolveSingleRepoScripts([]string{"dev", "nonexistent"}, root, cfg)
	if err != nil {
		t.Fatalf("ResolveSingleRepoScripts: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (missing script skipped), got %d", len(entries))
	}
	if entries[0].ScriptName != "dev" {
		t.Errorf("expected 'dev', got %q", entries[0].ScriptName)
	}
}
```

- [ ] **Step 5: Run all discover tests**

Run: `cd apps/miso && go test ./internal/tui/ -run "TestDeduplicateLabels|TestResolveSingleRepo" -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add apps/miso/internal/tui/discover.go apps/miso/internal/tui/discover_test.go
git commit -m "feat: add DeduplicateLabels and ResolveSingleRepoScripts, remove DiscoverMultiScripts"
```

### Task 4: Update `discoverEntries` to use concurrent discovery

**Files:**
- Modify: `apps/miso/internal/tui/launch.go:147-176` (discoverEntries)

- [ ] **Step 1: Rewrite `discoverEntries` to support concurrent tasks**

Replace the `discoverEntries` function in `apps/miso/internal/tui/launch.go` (lines 147-176):

```go
func discoverEntries(cfg config.Config, scriptName string, root string) ([]TuiScriptEntry, error) {
	if cfg.IsMonorepo() {
		wsDirs, err := config.LoadWorkspaces(root)
		if err != nil {
			return nil, fmt.Errorf("failed to load workspaces: %w", err)
		}

		var wsInfos []WorkspaceInfo
		for _, dir := range wsDirs {
			name := filepath.Base(dir)
			wsInfos = append(wsInfos, WorkspaceInfo{
				Name: name,
				Dir:  dir,
			})
		}

		entries, err := DiscoverTuiScripts(scriptName, wsInfos, cfg.Scripts)
		if err != nil {
			return nil, err
		}

		// Discover concurrent companion tasks
		for _, concName := range cfg.TaskConcurrent(scriptName) {
			concEntries, err := DiscoverTuiScripts(concName, wsInfos, cfg.Scripts)
			if err != nil {
				return nil, fmt.Errorf("discover concurrent %q: %w", concName, err)
			}
			entries = append(entries, concEntries...)
		}

		return DeduplicateLabels(entries), nil
	}

	// Single repo with concurrent config
	concurrent := cfg.TaskConcurrent(scriptName)
	if len(concurrent) > 0 {
		// Resolve the main task itself
		mainEntries, err := ResolveSingleRepoScripts([]string{scriptName}, root, cfg)
		if err != nil {
			return nil, err
		}

		// Resolve each concurrent task
		concEntries, err := ResolveSingleRepoScripts(concurrent, root, cfg)
		if err != nil {
			return nil, err
		}

		return DeduplicateLabels(append(mainEntries, concEntries...)), nil
	}

	return nil, nil
}
```

- [ ] **Step 2: Remove unused `cfg.Multi` import usage — verify compile**

Run: `cd apps/miso && go build ./cmd/main.go`
Expected: Should compile. If `config.Multi` is still referenced elsewhere in `launch.go`, those references were removed in step 1.

- [ ] **Step 3: Update `Launch` to handle concurrent entries with `dependsOn` ordering**

In `apps/miso/internal/tui/launch.go`, add `"strings"` to the import block. Then replace the startup goroutine (lines 58-126) with concurrent-aware logic:

```go
	// Pre-compute dependency levels if this command has dependsOn config.
	// Only main task entries participate in dependency ordering — concurrent
	// entries are started immediately.
	var levels [][]TuiScriptEntry
	var concurrentProcs []*Process
	if cfg.HasDependsOn(scriptName) {
		// Split entries: main task entries vs concurrent entries.
		// Use prefix matching (same as DiscoverTuiScripts) to identify
		// concurrent entries — e.g., concurrent: ["services"] should match
		// entries with ScriptName "services" AND "services:worker".
		var mainEntries []TuiScriptEntry
		concurrentPrefixes := cfg.TaskConcurrent(scriptName)
		for _, e := range entries {
			isConcurrent := false
			for _, prefix := range concurrentPrefixes {
				if e.ScriptName == prefix || strings.HasPrefix(e.ScriptName, prefix+":") || strings.HasPrefix(e.ScriptName, prefix+"/") {
					isConcurrent = true
					break
				}
			}
			if isConcurrent {
				proc := pm.findProc(e.Label)
				if proc != nil {
					concurrentProcs = append(concurrentProcs, proc)
				}
			} else {
				mainEntries = append(mainEntries, e)
			}
		}

		wsInfos := buildWSInfos(mainEntries)
		graph, err := BuildDependencyGraph(wsInfos)
		if err != nil {
			return false, fmt.Errorf("build dependency graph: %w", err)
		}
		var sortErr error
		levels, sortErr = TopoSort(mainEntries, graph)
		if sortErr != nil {
			return false, sortErr
		}
	}

	var model tea.Model
	switch cfg.TuiMode {
	case "tabbed":
		model = NewTabbedModel(pm, scriptName, false)
	case "merged":
		model = NewMergedModel(pm, scriptName, false)
	default:
		return false, fmt.Errorf("unknown tui mode: %s", cfg.TuiMode)
	}

	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	pm.SetProgram(p)

	// Catch OS signals — tell bubbletea to quit cleanly so it restores
	// the alt screen. Process cleanup happens after p.Run() returns below.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		p.Quit()
	}()
	defer signal.Stop(sigCh)

	// Start all processes in a goroutine — prog.Send() blocks until the
	// bubbletea event loop is running, so we can't call Start before p.Run().
	go func() {
		// Start concurrent companions immediately
		for _, proc := range concurrentProcs {
			if err := pm.Start(proc); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to start %s: %v\n", proc.Entry.Label, err)
			}
		}

		if levels != nil {
			for _, level := range levels {
				var levelProcs []*Process
				for _, entry := range level {
					proc := pm.findProc(entry.Label)
					if proc == nil {
						continue
					}
					if err := pm.Start(proc); err != nil {
						fmt.Fprintf(os.Stderr, "warning: failed to start %s: %v\n", proc.Entry.Label, err)
					}
					levelProcs = append(levelProcs, proc)
				}
				pm.WaitAllExited(levelProcs)
				for _, proc := range levelProcs {
					if proc.ExitCode != 0 {
						return
					}
				}
			}
		} else {
			for _, proc := range pm.Processes {
				if err := pm.Start(proc); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to start %s: %v\n", proc.Entry.Label, err)
				}
			}
		}
	}()
```

- [ ] **Step 4: Verify the build compiles**

Run: `cd apps/miso && go build ./cmd/main.go`
Expected: Success

- [ ] **Step 5: Run all TUI tests**

Run: `cd apps/miso && go test ./internal/tui/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add apps/miso/internal/tui/launch.go
git commit -m "feat: concurrent task discovery and launch in TUI"
```

### Task 5: Full build verification

**Files:** None (verification only)

- [ ] **Step 1: Run all tests across the project**

Run: `cd apps/miso && go test ./... -v`
Expected: All PASS. Watch for compilation errors from any remaining `Multi` references.

- [ ] **Step 2: Fix any remaining `Multi` references**

If any tests fail due to `cfg.Multi` references, remove them. Key places to check:
- `apps/miso/internal/tui/launch.go` — the monorepo+multi warning (line 148-149) should already be gone
- `apps/miso/cmd/main.go` — no `Multi` references expected

- [ ] **Step 3: Commit if any fixes were needed**

```bash
git add -A
git commit -m "fix: remove remaining multi references"
```

---

## Chunk 3: Schema, Examples, and Documentation

### Task 6: Update JSON schema files

**Files:**
- Modify: `apps/miso/miso.schema.json`
- Modify: `apps/docs/public/miso.schema.json`

- [ ] **Step 1: Update `apps/miso/miso.schema.json`**

Add `concurrent` to the task config properties object (after the `dependsOn` property, inside `additionalProperties` under `tasks`). The task object at lines 48-56 should become:

```json
"additionalProperties": {
    "type": "object",
    "properties": {
        "dependsOn": {
            "type": "array",
            "items": { "type": "string" },
            "description": "Task dependencies. Use '^taskName' prefix to run upstream workspace dependencies first."
        },
        "concurrent": {
            "type": "array",
            "items": { "type": "string" },
            "description": "Additional tasks to discover and run in parallel alongside the main task."
        }
    },
    "additionalProperties": false
}
```

Update the `tasks` description (line 58) to:
```
"description": "Per-task configuration. Available in all repo modes. Defines dependency ordering (dependsOn) and companion tasks (concurrent). In turbo/nx mode, tasks listed here use miso's direct orchestration instead of delegation."
```

Remove the `multi` property (lines 96-103).

- [ ] **Step 2: Update `apps/docs/public/miso.schema.json`**

Apply the same three changes:
1. Add `concurrent` to the task config properties (lines 48-56)
2. Update `tasks` description (line 58)
3. Remove `multi` property (lines 74-81)

- [ ] **Step 3: Validate both schemas are valid JSON**

Run: `python3 -c "import json; json.load(open('apps/miso/miso.schema.json')); json.load(open('apps/docs/public/miso.schema.json')); print('OK')"` from the miso root.
Expected: `OK`

- [ ] **Step 4: Commit**

```bash
git add apps/miso/miso.schema.json apps/docs/public/miso.schema.json
git commit -m "feat: update JSON schemas with concurrent field, remove multi"
```

### Task 7: Update example configs

**Files:**
- Modify: `apps/miso/miso.example.json`
- Modify: `apps/miso/miso.example.monorepo.json`

- [ ] **Step 1: Update `miso.example.json`**

Add `repo` with `tasks` and `concurrent` to the single-repo example. The full file should be:

```json
{
	"$schema": "https://misojs.dev/miso.schema.json",
	"scripts": "./scripts",
	"tui": "tabbed",
	"repo": {
		"tasks": {
			"dev": { "concurrent": ["services"] }
		}
	},
	"flags": {
		"install": ["--frozen-lockfile"],
		"dev": ["--env"]
	},
	"env": {
		"path": ".env.local",
		"variables": {
			"PORT": "port",
			"DATABASE_URL": "url",
			"API_KEY": "string"
		}
	}
}
```

- [ ] **Step 2: Update `miso.example.monorepo.json`**

Update the `dev` task to include `concurrent`:

```json
"tasks": {
    "build": { "dependsOn": ["^build"] },
    "dev": { "concurrent": ["services", "db:studio"] }
}
```

- [ ] **Step 3: Commit**

```bash
git add apps/miso/miso.example.json apps/miso/miso.example.monorepo.json
git commit -m "docs: update example configs with concurrent field"
```

### Task 8: Update documentation

**Files:**
- Modify: `apps/docs/content/tui/configuration.mdx`
- Modify: `apps/docs/content/working-with-miso/config.mdx`

- [ ] **Step 1: Update `apps/docs/content/tui/configuration.mdx`**

**Remove** the entire `## multi Option (Single Repos)` section (lines 97-128).

**Update** the `## repo Option` section:

Replace the "String Form" table row for `"single"` (line 50):
```
| `"single"` (default) | Standard project. Uses `concurrent` in task config for companion scripts. |
```

Replace the "Object Form" section (lines 61-83) with:

```markdown
### Object Form (with task configuration)

For concurrent tasks, dependency ordering, or turbo/nx task overrides, use the object form with `tasks`:

\`\`\`json
{
  "repo": {
    "mode": "mono",
    "tasks": {
      "build": { "dependsOn": ["^build"] },
      "dev": { "concurrent": ["services", "db:studio"] }
    }
  }
}
\`\`\`

The `tasks` key maps command names to task configuration. Each task supports:

- **`dependsOn`** — array of strings. The `^` prefix (e.g., `"^build"`) means "run this task in upstream workspace dependencies first," reading the dependency graph from each workspace's `package.json`.
- **`concurrent`** — array of strings. Additional task names to discover and run in parallel alongside the main task. Each name is discovered across all workspaces (monorepo) or resolved from the project root (single repo).

The object form works in all repo modes. In turbo/nx mode, tasks listed in `tasks` use miso's direct orchestration instead of delegation to the underlying tool.

Commands not listed in `tasks`, or listed without `dependsOn`, run all workspaces in parallel.
```

**Update** the "Script Discovery" section (lines 113-128):

Replace the "Single Repos" subsection with:

```markdown
### Single Repos

Uses the `concurrent` config in `repo.tasks` to determine which companion scripts to run alongside the main task. Scripts are resolved through the scripts folder first, then `package.json`.

If no `concurrent` entry exists for the command, the script runs normally without the TUI.
```

- [ ] **Step 2: Update `apps/docs/content/working-with-miso/config.mdx`**

**Remove** the `### Multi` section (lines 140-152).

**Update** the `### Repo` section: replace the object form example (lines 77-87) with:

```markdown
You can also use the object form for task configuration in any mode:

\`\`\`json
{
  "repo": {
    "mode": "mono",
    "tasks": {
      "build": { "dependsOn": ["^build"] },
      "dev": { "concurrent": ["services", "db:studio"] }
    }
  }
}
\`\`\`

- `dependsOn` with the `^` prefix means "run this task in upstream dependencies first."
- `concurrent` lists additional tasks to discover and run in parallel alongside the main task.
- In turbo/nx mode, tasks listed here use miso's direct orchestration instead of delegation.

See [TUI Configuration](/tui/configuration) for details.
```

- [ ] **Step 3: Commit**

```bash
git add apps/docs/content/tui/configuration.mdx apps/docs/content/working-with-miso/config.mdx
git commit -m "docs: document concurrent tasks, remove multi references"
```

### Task 9: Final verification

- [ ] **Step 1: Run full test suite**

Run: `cd apps/miso && go test ./... -v`
Expected: All PASS

- [ ] **Step 2: Search for any remaining `multi` config references**

Run: `grep -rn "\.Multi\b\|\"multi\"" apps/miso/internal/ apps/miso/cmd/ --include="*.go" | grep -v "multiple\|ActionRunMultiple\|RunMultiple\|splitMultiple"`

Expected: No output. Any hits need to be cleaned up (except `ActionRunMultiple`/`RunMultiple`/`splitMultipleScripts` which are unrelated — they handle running multiple scripts sequentially).

- [ ] **Step 3: Build the binary**

Run: `cd apps/miso && go build -o /dev/null ./cmd/main.go`
Expected: Success

- [ ] **Step 4: Commit any final fixes**

If any cleanup was needed:
```bash
git add -A
git commit -m "chore: final cleanup of multi references"
```
