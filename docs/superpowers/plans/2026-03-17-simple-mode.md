# Simple Mode Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow miso to operate as a pure script runner without requiring a JavaScript package manager, via a `"packageManager": false` config option.

**Architecture:** Add a `PackageManager *bool` field to the Config struct. In `main.go`, after loading config, check if simple mode is active. If so, bypass `ParseCLI` and `EnsureManager` entirely — resolve the first arg directly as a folder script (except meta-commands). Update `miso init` to offer a "simple mode" option when no JS project is detected.

**Tech Stack:** Go, `charmbracelet/huh` (TUI prompts), `encoding/json`

**Spec:** `docs/superpowers/specs/2026-03-17-simple-mode-design.md`

---

### Task 1: Add `PackageManager` field to Config

**Files:**
- Modify: `apps/miso/internal/config/config.go:27-37` (Config struct)
- Modify: `apps/miso/internal/config/config.go:186-194` (configLoad struct)
- Modify: `apps/miso/internal/config/config.go:222-229` (Load function, cfg assignment)
- Modify: `apps/miso/internal/config/config.go:426-437` (Save function)
- Test: `apps/miso/internal/config/config_test.go`

- [ ] **Step 1: Write failing tests for PackageManager parsing**

```go
func TestLoadPackageManagerFalse(t *testing.T) {
	dir := writeTempConfig(t, `{
		"packageManager": false
	}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.PackageManager == nil {
		t.Fatal("PackageManager is nil, want *false")
	}
	if *cfg.PackageManager != false {
		t.Errorf("PackageManager = %v, want false", *cfg.PackageManager)
	}
}

func TestLoadPackageManagerTrue(t *testing.T) {
	dir := writeTempConfig(t, `{
		"packageManager": true
	}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.PackageManager == nil {
		t.Fatal("PackageManager is nil, want *true")
	}
	if *cfg.PackageManager != true {
		t.Errorf("PackageManager = %v, want true", *cfg.PackageManager)
	}
}

func TestLoadPackageManagerAbsent(t *testing.T) {
	dir := writeTempConfig(t, `{}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.PackageManager != nil {
		t.Errorf("PackageManager = %v, want nil (absent)", *cfg.PackageManager)
	}
}

func TestSimpleMode(t *testing.T) {
	f := false
	cfg := Config{PackageManager: &f}
	if !cfg.SimpleMode() {
		t.Error("SimpleMode() = false, want true")
	}

	tr := true
	cfg2 := Config{PackageManager: &tr}
	if cfg2.SimpleMode() {
		t.Error("SimpleMode() = true, want false (explicit true)")
	}

	cfg3 := Config{}
	if cfg3.SimpleMode() {
		t.Error("SimpleMode() = true, want false (nil/absent)")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go test ./internal/config/ -run "TestLoadPackageManager|TestSimpleMode" -v`
Expected: FAIL — `PackageManager` field and `SimpleMode()` method don't exist.

- [ ] **Step 3: Add PackageManager field to Config and configLoad**

In `apps/miso/internal/config/config.go`, add to the `Config` struct:

```go
type Config struct {
	Schema         string                `json:"$schema,omitempty"`
	PackageManager *bool                 `json:"packageManager,omitempty"`
	Scripts        string                `json:"scripts"`
	Shell          string                `json:"shell,omitempty"`
	Flags          map[string][]string   `json:"flags,omitempty"`
	Env            []*EnvEntry           `json:"env,omitempty"`
	Repo           string                `json:"repo,omitempty"`
	Tasks          map[string]TaskConfig `json:"-"`
	TuiMode        string                `json:"tui,omitempty"`
	TuiCleanExit   bool                  `json:"-"`
}
```

Add to the `configLoad` struct:

```go
type configLoad struct {
	Schema         string              `json:"$schema,omitempty"`
	PackageManager *bool               `json:"packageManager,omitempty"`
	Scripts        string              `json:"scripts"`
	Shell          string              `json:"shell,omitempty"`
	Flags          map[string][]string `json:"flags,omitempty"`
	EnvRaw         json.RawMessage     `json:"env,omitempty"`
	RepoRaw        json.RawMessage     `json:"repo,omitempty"`
	TuiRaw         json.RawMessage     `json:"tui,omitempty"`
}
```

In the `Load` function, pass `PackageManager` through in the cfg assignment:

```go
cfg := Config{
	Schema:         load.Schema,
	PackageManager: load.PackageManager,
	Scripts:        load.Scripts,
	Shell:          load.Shell,
	Flags:          load.Flags,
	TuiMode:        tuiMode,
	TuiCleanExit:   tuiCleanExit,
}
```

Add the `SimpleMode` helper method:

```go
// SimpleMode returns true when package manager features are disabled.
func (c Config) SimpleMode() bool {
	return c.PackageManager != nil && !*c.PackageManager
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go test ./internal/config/ -v`
Expected: ALL PASS (including existing tests — no regressions).

- [ ] **Step 5: Write round-trip test for Save**

```go
func TestSaveRoundTripsPackageManagerFalse(t *testing.T) {
	dir := t.TempDir()
	f := false
	cfg := Config{
		Schema:         SchemaURL,
		PackageManager: &f,
		Scripts:        "./scripts",
	}

	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() after Save() error: %v", err)
	}

	if loaded.PackageManager == nil {
		t.Fatal("PackageManager is nil after round-trip, want *false")
	}
	if *loaded.PackageManager != false {
		t.Errorf("PackageManager = %v after round-trip, want false", *loaded.PackageManager)
	}
}

func TestSaveOmitsPackageManagerWhenNil(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Schema:  SchemaURL,
		Scripts: "./scripts",
	}

	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}

	if strings.Contains(string(data), "packageManager") {
		t.Error("saved config contains packageManager, want it omitted when nil")
	}
}
```

Note: add `"strings"` to the test file imports.

- [ ] **Step 6: Run round-trip tests**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go test ./internal/config/ -run "TestSave" -v`
Expected: PASS — `omitempty` on `*bool` omits nil but includes `false`.

- [ ] **Step 7: Commit**

```bash
cd /Users/mikekenway/Development/miso
git add apps/miso/internal/config/config.go apps/miso/internal/config/config_test.go
git commit -m "feat: add PackageManager field to Config for simple mode"
```

---

### Task 2: Update JSON schema

**Files:**
- Modify: `apps/miso/miso.schema.json:7-119` (properties section)

- [ ] **Step 1: Add packageManager property to schema**

In `apps/miso/miso.schema.json`, add the `packageManager` property inside the `"properties"` object, after `"$schema"`:

```json
"packageManager": {
	"type": "boolean",
	"default": true,
	"description": "Enable or disable package manager features. When false, miso operates as a pure script runner — no install/add/remove commands, no package.json script discovery. Default: true."
},
```

- [ ] **Step 2: Verify schema is valid JSON**

Run: `cd /Users/mikekenway/Development/miso && python3 -c "import json; json.load(open('apps/miso/miso.schema.json'))"`
Expected: No output (valid JSON).

- [ ] **Step 3: Commit**

```bash
cd /Users/mikekenway/Development/miso
git add apps/miso/miso.schema.json
git commit -m "feat: add packageManager to JSON schema"
```

---

### Task 3: Add `ResolveScriptFolderOnly` function

**Files:**
- Modify: `apps/miso/internal/cli/scripting/resolve.go`
- Test: `apps/miso/internal/cli/scripting/scripts_test.go`

In simple mode, `ResolveScript` must skip the `package.json` fallback. Rather than adding a flag to the existing function (which would require updating all 12+ callers), add a new `ResolveScriptFolderOnly` function that only checks the scripts folder.

- [ ] **Step 1: Write failing test**

```go
func TestResolveScriptFolderOnlyIgnoresPackageJSON(t *testing.T) {
	dir := t.TempDir()

	// Create scripts dir (empty)
	scriptsDir := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create package.json with a "build" script
	pkgJSON := `{"scripts": {"build": "echo building"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{Scripts: "./scripts"}
	cfg.EnsureDefaults()

	// ResolveScriptFolderOnly should NOT find "build" from package.json
	resolved, err := ResolveScriptFolderOnly("build", dir, cfg)
	if err != nil {
		t.Fatalf("ResolveScriptFolderOnly() error: %v", err)
	}
	if resolved.Source != ScriptSourceNone {
		t.Errorf("Source = %v, want ScriptSourceNone (package.json should be ignored)", resolved.Source)
	}
}

func TestResolveScriptFolderOnlyFindsFolderScript(t *testing.T) {
	dir := t.TempDir()

	// Create scripts dir with a build.sh
	scriptsDir := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "build.sh"), []byte("#!/bin/sh\necho build"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{Scripts: "./scripts"}
	cfg.EnsureDefaults()

	resolved, err := ResolveScriptFolderOnly("build", dir, cfg)
	if err != nil {
		t.Fatalf("ResolveScriptFolderOnly() error: %v", err)
	}
	if resolved.Source != ScriptSourceFolder {
		t.Errorf("Source = %v, want ScriptSourceFolder", resolved.Source)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go test ./internal/cli/scripting/ -run "TestResolveScriptFolderOnly" -v`
Expected: FAIL — `ResolveScriptFolderOnly` undefined.

- [ ] **Step 3: Implement ResolveScriptFolderOnly**

Add to `apps/miso/internal/cli/scripting/resolve.go`:

```go
// ResolveScriptFolderOnly resolves a script by name from the folder only.
// Unlike ResolveScript, it does NOT fall back to package.json.
// Used in simple mode where package.json scripts are ignored.
func ResolveScriptFolderOnly(name string, root string, cfg config.Config) (ResolvedScript, error) {
	scriptsPath := cfg.Scripts
	if scriptsPath == "" {
		scriptsPath = "./scripts"
	}

	if !filepath.IsAbs(scriptsPath) {
		scriptsPath = filepath.Join(root, scriptsPath)
	}

	discovered, err := DiscoverScripts(scriptsPath)
	if err != nil {
		return ResolvedScript{}, fmt.Errorf("discover scripts: %w", err)
	}

	name = filepath.ToSlash(name)

	ext := filepath.Ext(name)
	if ext != "" {
		key := strings.TrimSuffix(name, ext)
		if scripts, ok := discovered[key]; ok {
			for _, script := range scripts {
				if script.Extension == ext {
					if len(scripts) > 1 {
						var paths []string
						for _, s := range scripts {
							paths = append(paths, s.RelativePath)
						}
						return ResolvedScript{}, fmt.Errorf("multiple scripts for %q exist, exiting: %s",
							key, strings.Join(paths, ", "))
					}
					return ResolvedScript{
						Source: ScriptSourceFolder,
						Path:   script.Path,
					}, nil
				}
			}
		}
	}

	key := strings.TrimSuffix(name, ext)
	if scripts, ok := discovered[key]; ok {
		if len(scripts) > 1 {
			var paths []string
			for _, script := range scripts {
				paths = append(paths, script.RelativePath)
			}
			return ResolvedScript{}, fmt.Errorf("multiple scripts for %q exist, exiting: %s",
				key, strings.Join(paths, ", "))
		}
		return ResolvedScript{
			Source: ScriptSourceFolder,
			Path:   scripts[0].Path,
		}, nil
	}

	return ResolvedScript{Source: ScriptSourceNone}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go test ./internal/cli/scripting/ -v`
Expected: ALL PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/mikekenway/Development/miso
git add apps/miso/internal/cli/scripting/resolve.go apps/miso/internal/cli/scripting/scripts_test.go
git commit -m "feat: add ResolveScriptFolderOnly for simple mode"
```

---

### Task 4: Simple mode routing in main.go

**Files:**
- Modify: `apps/miso/cmd/main.go:108-157` (after config load, before ParseCLI)

This is the core change. After loading config, if `cfg.SimpleMode()`, bypass `ParseCLI` and `EnsureManager` entirely. Handle meta-commands (`env`, `scripts`), then resolve everything else as a folder script.

- [ ] **Step 1: Add simple mode routing block**

In `apps/miso/cmd/main.go`, after the config load block (line 118) and before the `ParseCLI` call (line 120), add the simple mode routing:

```go
	cfg, err := cli.LoadConfig(projectRoot)
	if err != nil {
		cli.Fail(logger, err, false)
	}

	// Simple mode: bypass ParseCLI and EnsureManager entirely.
	// Only meta-commands are handled; everything else is folder script resolution.
	if cfg.SimpleMode() {
		if len(args) == 0 {
			cli.Fail(logger, fmt.Errorf("missing command"), true)
		}

		cmd := args[0]

		// Meta-commands that remain in simple mode
		// (init, version, upgrade, completion already handled above)
		switch cmd {
		case "env":
			if err := env.Run(projectRoot, cfg, logger); err != nil {
				os.Exit(1)
			}
			return
		case "scripts":
			if err := scripting.List(cfg, projectRoot, styles, logger); err != nil {
				cli.Fail(logger, err, false)
			}
			return
		}

		// Resolve as folder script only (no package.json fallback)
		resolved, err := scripting.ResolveScriptFolderOnly(cmd, projectRoot, cfg)
		if err != nil {
			cli.Fail(logger, err, false)
		}

		if resolved.Source == scripting.ScriptSourceNone {
			cli.Fail(logger, fmt.Errorf("script '%s' not found in %s", cmd, cfg.Scripts), false)
		}

		scriptArgs := args[1:]

		// Handle --env flag: run env validation before script execution
		if env.HasEnvFlag(scriptArgs) {
			if err := env.Run(projectRoot, cfg, logger); err != nil {
				os.Exit(1)
			}
			scriptArgs = env.StripEnvFlag(scriptArgs)
		}

		// TUI interception for simple mode
		if cfg.TuiEnabled() {
			ran, err := tui.Launch(cfg, cmd, projectRoot, nil)
			if err != nil {
				cli.Fail(logger, err, false)
			}
			if ran {
				return
			}
		}

		if err := scripting.ExecScriptFile(resolved.Path, scriptArgs, originalWorkDir, cfg.Shell); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	}

	parsed, err := cli.ParseCLI(args, cfg, projectRoot)
	// ... rest of existing code
```

- [ ] **Step 2: Verify compilation**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go build ./cmd/`
Expected: Build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
cd /Users/mikekenway/Development/miso
git add apps/miso/cmd/main.go
git commit -m "feat: add simple mode routing bypass in main.go"
```

---

### Task 5: TUI simple mode support

**Files:**
- Modify: `apps/miso/internal/tui/launch.go:49-53` (mgr.BuildRun branch)
- Modify: `apps/miso/internal/tui/discover.go:163-196` (ResolveSingleRepoScripts)
- Modify: `apps/miso/internal/tui/discover.go:57-133` (discoverWorkspaceScripts)

The TUI discovery path uses `ResolveScript` and `ReadPackageJSONScripts`, which include `package.json` fallback. In simple mode, these must use folder-only resolution to prevent discovering `package.json` scripts that can't be executed without a manager.

- [ ] **Step 1: Add nil guard before mgr.BuildRun()**

In `apps/miso/internal/tui/launch.go`, replace the package manager branch (lines 49-53):

```go
		} else {
			// Run via package manager
			if mgr == nil {
				return false, fmt.Errorf("script %q requires a package manager but none is configured", entry.ScriptName)
			}
			spec := mgr.BuildRun(entry.ScriptName, nil)
			cmd = spec.Command
			args = spec.Args
		}
```

- [ ] **Step 2: Add ResolveSingleRepoScriptsFolderOnly**

In `apps/miso/internal/tui/discover.go`, add after `ResolveSingleRepoScripts`:

```go
// ResolveSingleRepoScriptsFolderOnly is like ResolveSingleRepoScripts but only
// checks the scripts folder, ignoring package.json. Used in simple mode.
func ResolveSingleRepoScriptsFolderOnly(scripts []string, root string, cfg config.Config) ([]TuiScriptEntry, error) {
	var entries []TuiScriptEntry

	for _, name := range scripts {
		resolved, err := scripting.ResolveScriptFolderOnly(name, root, cfg)
		if err != nil {
			return nil, err
		}
		if resolved.Source == scripting.ScriptSourceNone {
			continue
		}

		entries = append(entries, TuiScriptEntry{
			Label:        name,
			ScriptName:   name,
			WorkspaceDir: root,
			ScriptSource: "folder",
			ScriptPath:   resolved.Path,
		})
	}

	return entries, nil
}
```

- [ ] **Step 3: Update discoverEntries to use folder-only resolution in simple mode**

In `apps/miso/internal/tui/launch.go`, the `discoverEntries` function (line 182) handles single-repo concurrent discovery. Update it to use the folder-only variant when `cfg.SimpleMode()`:

Replace the single-repo concurrent block in `discoverEntries` (lines 216-231):

```go
	// Single repo with concurrent config
	concurrent := cfg.TaskConcurrent(scriptName)
	if len(concurrent) > 0 {
		var mainEntries, concEntries []TuiScriptEntry
		var err error

		if cfg.SimpleMode() {
			mainEntries, err = ResolveSingleRepoScriptsFolderOnly([]string{scriptName}, root, cfg)
			if err != nil {
				return nil, err
			}
			concEntries, err = ResolveSingleRepoScriptsFolderOnly(concurrent, root, cfg)
		} else {
			mainEntries, err = ResolveSingleRepoScripts([]string{scriptName}, root, cfg)
			if err != nil {
				return nil, err
			}
			concEntries, err = ResolveSingleRepoScripts(concurrent, root, cfg)
		}
		if err != nil {
			return nil, err
		}

		return DeduplicateLabels(append(mainEntries, concEntries...)), nil
	}
```

Also update the monorepo path in `discoverEntries` (line 183). In simple mode, monorepo workspaces are not supported (spec says no workspace discovery from `package.json`). Add a guard at the top of `discoverEntries`:

```go
func discoverEntries(cfg config.Config, scriptName string, root string) ([]TuiScriptEntry, error) {
	// Simple mode does not support monorepo workspace discovery
	if cfg.IsMonorepo() && cfg.SimpleMode() {
		return nil, nil
	}

	if cfg.IsMonorepo() {
		// ... existing monorepo code unchanged
```

- [ ] **Step 4: Verify compilation**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go build ./cmd/`
Expected: Build succeeds.

- [ ] **Step 5: Run existing TUI tests**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go test ./internal/tui/ -v`
Expected: ALL PASS — no regressions.

- [ ] **Step 6: Commit**

```bash
cd /Users/mikekenway/Development/miso
git add apps/miso/internal/tui/launch.go apps/miso/internal/tui/discover.go
git commit -m "feat: TUI simple mode support with folder-only script resolution"
```

---

### Task 6: Update `miso init` for simple mode

**Files:**
- Modify: `apps/miso/internal/cli/commands/init.go:107-295` (RunInit function)

- [ ] **Step 1: Add simple mode init branch**

In `apps/miso/internal/cli/commands/init.go`, the current `default:` case (line 222) handles "no package.json — new project". Replace it with the new two-option flow from the spec.

**Important restructuring:** The current code asks `askRepoType` and `askWorkspacePatterns` (lines 127-138) **before** the switch statement. These must be moved **into** each case that needs them (cases 1, 2, and the "new project" sub-branch of case 3). The simple mode branch must not ask about repo type — it's irrelevant without a package manager.

**Step-by-step:**

1. **Delete lines 126-138** (the `askRepoType` block and `askWorkspacePatterns` block before the `switch`):
```go
// DELETE THIS BLOCK:
repoType, err := askRepoType(styles)
if err != nil {
    return err
}

var workspacePatterns []string
if repoType == "mono" {
    workspacePatterns, err = askWorkspacePatterns(styles)
    if err != nil {
        return err
    }
}
```

2. **In Case 1** (line 141, `case pkg != nil && packageManagerFromPackageJSON(pkg) != ""`), add the repo type prompt at the start of the case, right after the `managerName` assignment and logger line:
```go
	case pkg != nil && packageManagerFromPackageJSON(pkg) != "":
		managerName := packageManagerFromPackageJSON(pkg)
		// ... existing validation ...
		logger.Info("using package manager from package.json", "manager", managerName)

		// Ask repo type (moved here from before switch)
		repoType, err := askRepoType(styles)
		if err != nil {
			return err
		}
		var workspacePatterns []string
		if repoType == "mono" {
			workspacePatterns, err = askWorkspacePatterns(styles)
			if err != nil {
				return err
			}
		}

		// ... rest of case 1 unchanged (uses repoType and workspacePatterns)
```

3. **In Case 2** (line 180, `case pkg != nil:`), add the same block right after `detected, _ := manager.DetectManager(root)`:
```go
	case pkg != nil:
		detected, _ := manager.DetectManager(root)

		// Ask repo type (moved here from before switch)
		repoType, err := askRepoType(styles)
		if err != nil {
			return err
		}
		var workspacePatterns []string
		if repoType == "mono" {
			workspacePatterns, err = askWorkspacePatterns(styles)
			if err != nil {
				return err
			}
		}

		managerName, err := selectManager(managerList, detected, styles)
		// ... rest of case 2 unchanged
```

4. **Replace the `default:` case** starting at line 222 with:

```go
	// ── Case 3: no package.json — offer new project or simple mode ───────
	default:
		var initChoice string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(styles.Heading.Render("miso could not detect an existing javascript project.")).
					Description(styles.Muted.Render("Press ↑/↓ to move, Enter to confirm • ctrl+c to bail")).
					Options(
						huh.NewOption("Create new project", "new"),
						huh.NewOption("Run in simple mode (scripts only, no package manager)", "simple"),
					).
					Value(&initChoice),
			),
		).WithTheme(huh.ThemeCharm())

		if err := form.Run(); err != nil {
			return err
		}

		if initChoice == "simple" {
			// Simple mode: scaffold miso.json with packageManager: false and create scripts dir
			f := false
			cfg := config.Config{
				Schema:         config.SchemaURL,
				PackageManager: &f,
				Scripts:        "./scripts",
			}
			if err := config.Save(root, cfg); err != nil {
				return err
			}
			logger.Info("created miso.json (simple mode)")

			scriptsDir := filepath.Join(root, "scripts")
			if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
				return fmt.Errorf("create scripts directory: %w", err)
			}
			logger.Info("created scripts directory")
			return nil
		}

		// "new" — create a new JS project
		// Now ask repo type (only relevant for JS projects)
		repoType, err := askRepoType(styles)
		if err != nil {
			return err
		}

		var workspacePatterns []string
		if repoType == "mono" {
			workspacePatterns, err = askWorkspacePatterns(styles)
			if err != nil {
				return err
			}
		}

		projectName, err := askProjectName(filepath.Base(root), styles)
		if err != nil {
			return err
		}

		managerName, err := selectManager(managerList, "", styles)
		if err != nil {
			return err
		}

		logger.Info("scaffolding new project", "name", projectName, "manager", managerName)

		if _, ok := manager.GetManager(managerName); !ok {
			return fmt.Errorf("unsupported manager: %s", managerName)
		}

		spec := manager.ExecSpec{
			Command: managerName,
			Args:    []string{"init"},
		}
		if err := manager.Exec(spec, root); err != nil {
			return err
		}

		freshPkg, err := readPackageJSON(root)
		if err != nil {
			return err
		}
		if freshPkg == nil {
			freshPkg = map[string]interface{}{
				"name":    projectName,
				"version": "0.0.1",
			}
		} else {
			freshPkg["name"] = projectName
		}
		freshPkg["packageManager"] = manager.ResolveManagerVersion(managerName)

		if repoType == "mono" {
			freshPkg["workspaces"] = workspacePatterns
		}

		if err := writePackageJSON(root, freshPkg); err != nil {
			return err
		}

		if repoType == "mono" {
			logger.Info("added workspaces to package.json")
			if err := scaffoldWorkspaceDirs(root, workspacePatterns, logger); err != nil {
				return err
			}
		}

		cfg := config.Config{
			Schema:  config.SchemaURL,
			Scripts: "./scripts",
			Flags:   make(map[string][]string),
			Repo:    repoType,
		}
		if err := config.Save(root, cfg); err != nil {
			return err
		}
		logger.Info("created miso.json")
	}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go build ./cmd/`
Expected: Build succeeds.

- [ ] **Step 3: Commit**

```bash
cd /Users/mikekenway/Development/miso
git add apps/miso/internal/cli/commands/init.go
git commit -m "feat: add simple mode option to miso init"
```

---

### Task 7: Full integration verification

**Files:**
- No new files — manual verification against the spec.

- [ ] **Step 1: Run all tests**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go test ./... -v`
Expected: ALL PASS.

- [ ] **Step 2: Build binaries**

Run: `cd /Users/mikekenway/Development/miso/apps/miso && go build -o bin/miso ./cmd/`
Expected: Binary builds successfully.

- [ ] **Step 3: Manual test — simple mode script execution**

```bash
cd /tmp && mkdir test-simple && cd test-simple
cat > miso.json << 'EOF'
{
  "$schema": "https://misojs.dev/miso.schema.json",
  "packageManager": false,
  "scripts": "./scripts"
}
EOF
mkdir scripts
echo '#!/bin/sh
echo "hello from build"' > scripts/build.sh
chmod +x scripts/build.sh

# Test: miso build should find and run scripts/build.sh
/Users/mikekenway/Development/miso/apps/miso/bin/miso build
# Expected output: "hello from build"

# Test: miso install should fail with "script 'install' not found"
/Users/mikekenway/Development/miso/apps/miso/bin/miso install
# Expected: error

# Test: miso scripts should list discovered scripts
/Users/mikekenway/Development/miso/apps/miso/bin/miso scripts
# Expected: shows "build"

# Test: miso env should work (even if no env config)
/Users/mikekenway/Development/miso/apps/miso/bin/miso env

# Cleanup
cd / && rm -rf /tmp/test-simple
```

- [ ] **Step 4: Manual test — miso init simple mode**

```bash
cd /tmp && mkdir test-init && cd test-init

# Run miso init — should detect no JS project and offer two options
/Users/mikekenway/Development/miso/apps/miso/bin/miso init
# Select "Run in simple mode"
# Expected: creates miso.json with packageManager: false and scripts/ directory

# Verify
cat miso.json
ls scripts/

# Cleanup
cd / && rm -rf /tmp/test-init
```

- [ ] **Step 5: Commit (if any fixes were needed)**

```bash
cd /Users/mikekenway/Development/miso
git add -A
git commit -m "fix: address integration test findings for simple mode"
```

---

### Task 8: Update documentation

**Files:**
- Modify: `apps/docs/content/index.mdx`
- Modify: `apps/docs/content/working-with-miso/getting-started.mdx`
- Modify: `apps/docs/content/working-with-miso/config.mdx`
- Modify: `apps/docs/content/working-with-miso/basic-commands.mdx`
- Create: `apps/docs/content/working-with-miso/simple-mode.mdx`
- Create: `apps/miso/miso.example.simple.json`
- Modify: `README.md`

- [ ] **Step 1: Create simple mode example config**

Create `apps/miso/miso.example.simple.json`:

```json
{
	"$schema": "https://misojs.dev/miso.schema.json",
	"packageManager": false,
	"scripts": "./scripts",
	"shell": "bash",
	"tui": "tabbed",
	"repo": {
		"tasks": {
			"dev": { "concurrent": ["services"] }
		}
	},
	"env": {
		"path": ".env.local",
		"variables": {
			"PORT": "port",
			"DATABASE_URL": "url"
		}
	}
}
```

- [ ] **Step 2: Create simple mode docs page**

Create `apps/docs/content/working-with-miso/simple-mode.mdx`:

```mdx
---
title: Simple Mode
---

# Simple Mode

Miso can run without a JavaScript package manager. If you like miso's script execution, TUI, env validation, and task orchestration but your project isn't JavaScript — simple mode is for you.

## Setup

Run `miso init` from any directory. If no JavaScript project is detected, miso will offer two options:

```
? miso could not detect an existing javascript project.
  ❯ Create new project
    Run in simple mode (scripts only, no package manager)
```

Select **"Run in simple mode"** and miso will create a `miso.json` with package management disabled, plus a `scripts/` directory:

```json
{
  "$schema": "https://misojs.dev/miso.schema.json",
  "packageManager": false,
  "scripts": "./scripts"
}
```

You can also create this file manually. The key is `"packageManager": false`.

## How It Works

In simple mode, there are no built-in commands. Every argument you pass to miso is resolved as a script name from the `scripts/` folder:

```bash
miso build       # runs scripts/build.sh
miso deploy      # runs scripts/deploy.sh
miso test        # runs scripts/test.sh
```

If a script isn't found, miso tells you:

```
ERROR miso: script 'build' not found in ./scripts
```

## What's Available

Everything except package management:

- **Script execution** — folder scripts with shebang detection and extension-based interpreters
- **TUI** — tabbed and merged modes for concurrent task display
- **Task orchestration** — `dependsOn` and `concurrent` in `repo.tasks`
- **Env validation** — `miso env` works the same as always
- **Meta-commands** — `miso env`, `miso scripts`, `miso init`, `miso version`, `miso upgrade`, `miso completion`

## What's Not Available

- `install`, `add`, `remove`, and all other package manager commands
- `package.json` script discovery (even if a `package.json` exists, its `scripts` are ignored)
- Workspace discovery from `package.json`
- `flags` config (no package manager to pass flags to)
- `misox`

## Switching to Full Mode

Run `miso init` again from your project root. If you choose to create a new project, miso will set up a package manager and remove the `"packageManager": false` setting.

Or just edit `miso.json` manually — remove the `"packageManager": false` line and miso will default back to normal package manager mode.
```

- [ ] **Step 3: Update docs homepage**

In `apps/docs/content/index.mdx`, update the description to mention simple mode. After the line "Miso is a universal package manager wrapper for JavaScript projects." add:

```
It also works as a standalone script runner for any language — see [Simple Mode](/working-with-miso/simple-mode).
```

- [ ] **Step 4: Update getting-started.mdx**

In `apps/docs/content/working-with-miso/getting-started.mdx`, after the "Choosing Your Repo Type" section (after the Monorepo subsection, before "Running Commands from Anywhere"), add a new section:

```mdx
### Simple Mode (No Package Manager)

If `miso init` doesn't detect an existing JavaScript project (no `package.json` or lockfile), it offers an additional option:

```
? miso could not detect an existing javascript project.
  ❯ Create new project
    Run in simple mode (scripts only, no package manager)
```

Selecting **simple mode** creates a `miso.json` with `"packageManager": false` and a `scripts/` directory. This lets you use miso's script runner, TUI, env validation, and task orchestration in any project — Go, Python, Rust, or anything else.

See [Simple Mode](/working-with-miso/simple-mode) for the full guide.
```

- [ ] **Step 5: Update config.mdx**

In `apps/docs/content/working-with-miso/config.mdx`, add a new section after "### JSON Schema (IDE Support)" and before "### Scripts":

```mdx
### Package Manager

Controls whether miso operates as a package manager wrapper or a standalone script runner:

<Terminal
    copy={false}
    className="mt-4 border border-neutral-700 rounded-sm"
    title="miso.json"
    command='"packageManager": false'
/>

<DocProp prop="boolean (default: true)" />

When set to `false`, miso runs in **simple mode** — all package manager features are disabled. Every command resolves as a folder script. See [Simple Mode](/working-with-miso/simple-mode) for details.

When `true` (or omitted), miso operates normally as a package manager wrapper.
```

- [ ] **Step 6: Update basic-commands.mdx**

In `apps/docs/content/working-with-miso/basic-commands.mdx`, add a note at the top of the file after the "Resolution Order" link paragraph:

```mdx
<Callout type="info">
In [simple mode](/working-with-miso/simple-mode) (`"packageManager": false`), there are no built-in commands — everything resolves as a script. The commands below (install, add, remove, dev, misox) are only available when a package manager is configured.
</Callout>
```

- [ ] **Step 7: Update README.md**

In `README.md`, after the "Why Miso?" section, add:

```markdown
## Simple Mode

Don't use JavaScript? Miso also works as a standalone script runner. Set `"packageManager": false` in your `miso.json` and use miso for script execution, TUI, env validation, and task orchestration in any project.

```json
{
    "$schema": "https://misojs.dev/miso.schema.json",
    "packageManager": false,
    "scripts": "./scripts"
}
```
```

- [ ] **Step 8: Commit documentation**

```bash
cd /Users/mikekenway/Development/miso
git add README.md apps/docs/content/ apps/miso/miso.example.simple.json
git commit -m "docs: add simple mode documentation and example config"
```
