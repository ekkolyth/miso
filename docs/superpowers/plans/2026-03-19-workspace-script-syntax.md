# Workspace Script Syntax Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the broken `workspace:script` colon syntax with `@workspace/script`, redesign workspace lookup to match by basename/path/package-name, and extend workspace script resolution to fall through to the workspace's `package.json`.

**Architecture:** Four sequential changes — router parsing, config lookup, scripting resolver, execution handler — followed by skills/docs update. Each change is independently testable. TDD throughout: write the failing test first, then implement.

**Tech Stack:** Go 1.21+, standard library only (`os`, `path/filepath`, `encoding/json`, `strings`)

---

## File Map

| File | Change |
|---|---|
| `apps/miso/internal/cli/router.go` | Replace colon-split block with `@workspace/script` detection |
| `apps/miso/internal/cli/router_test.go` | **New** — parser tests |
| `apps/miso/internal/config/config.go` | Redesign `FindWorkspace` signature and matching logic |
| `apps/miso/internal/config/config_test.go` | Extend with `FindWorkspace` matching tests |
| `apps/miso/internal/cli/scripting/workspace.go` | Update `FindWorkspace` call; add `package.json` fallback |
| `apps/miso/internal/cli/scripting/workspace_test.go` | **New** — workspace resolution tests |
| `apps/miso/cmd/main.go` | Handle `ScriptSourcePackageJSON` in `ActionWorkspaceScript`; update CWD-scoping block |
| `packages/skills/miso-scripting/SKILL.md` | Update workspace-scoped scripts section |

---

## Task 1: Router — `@workspace/script` parsing

**Files:**
- Modify: `apps/miso/internal/cli/router.go:155-168`
- Create: `apps/miso/internal/cli/router_test.go`

### Step 1: Create router_test.go with failing tests

```go
package cli

import (
	"testing"

	"github.com/ekkolyth/miso/internal/config"
)

// minimal cfg helpers
func monoCfg() config.Config  { return config.Config{Repo: "mono"} }
func singleCfg() config.Config { return config.Config{Repo: "single"} }

func TestParseAtWorkspaceScript(t *testing.T) {
	parsed, err := ParseCLI([]string{"@api/test"}, monoCfg(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Action != ActionWorkspaceScript {
		t.Errorf("Action = %v, want ActionWorkspaceScript", parsed.Action)
	}
	if parsed.WorkspaceName != "api" {
		t.Errorf("WorkspaceName = %q, want %q", parsed.WorkspaceName, "api")
	}
	if parsed.ScriptName != "test" {
		t.Errorf("ScriptName = %q, want %q", parsed.ScriptName, "test")
	}
}

func TestParseAtWorkspaceColonScript(t *testing.T) {
	parsed, err := ParseCLI([]string{"@api/test:unit"}, monoCfg(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.WorkspaceName != "api" {
		t.Errorf("WorkspaceName = %q, want %q", parsed.WorkspaceName, "api")
	}
	if parsed.ScriptName != "test:unit" {
		t.Errorf("ScriptName = %q, want %q", parsed.ScriptName, "test:unit")
	}
}

func TestParseAtScopedWorkspace(t *testing.T) {
	parsed, err := ParseCLI([]string{"@myorg/api/build"}, monoCfg(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.WorkspaceName != "myorg/api" {
		t.Errorf("WorkspaceName = %q, want %q", parsed.WorkspaceName, "myorg/api")
	}
	if parsed.ScriptName != "build" {
		t.Errorf("ScriptName = %q, want %q", parsed.ScriptName, "build")
	}
}

func TestParseAtWorkspaceMissingSlashErrors(t *testing.T) {
	_, err := ParseCLI([]string{"@api"}, monoCfg(), t.TempDir())
	if err == nil {
		t.Error("expected error for @workspace with no slash, got nil")
	}
}

func TestParseAtPathWorkspace(t *testing.T) {
	parsed, err := ParseCLI([]string{"@packages/api/build"}, monoCfg(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.WorkspaceName != "packages/api" {
		t.Errorf("WorkspaceName = %q, want %q", parsed.WorkspaceName, "packages/api")
	}
	if parsed.ScriptName != "build" {
		t.Errorf("ScriptName = %q, want %q", parsed.ScriptName, "build")
	}
}

func TestParseColonScriptNotWorkspaceRoutedInMono(t *testing.T) {
	// test:unit in monorepo should NOT be ActionWorkspaceScript
	// (it will fall through to ResolveScript which returns ScriptSourceNone for an empty dir,
	// then passthrough — important thing is it's NOT ActionWorkspaceScript)
	parsed, err := ParseCLI([]string{"test:unit"}, monoCfg(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Action == ActionWorkspaceScript {
		t.Errorf("Action = ActionWorkspaceScript, want anything else (colon-named script should not be workspace-routed)")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd apps/miso && go test ./internal/cli/... -run "TestParseAt|TestParseColon" -v
```

Expected: FAIL — `ParseCLI` still uses colon-split logic, so `@api/test` and `test:unit` route incorrectly.

- [ ] **Step 3: Replace the colon-split block in router.go**

In `apps/miso/internal/cli/router.go`, replace lines 155–168:

```go
	// check for workspace:script syntax (only in mono mode)
	if cfg.IsMonorepo() && strings.Contains(cmd, ":") {
		parts := strings.SplitN(cmd, ":", 2)
		workspaceName := parts[0]
		scriptName := parts[1]
		if workspaceName != "" && scriptName != "" {
			return ParsedCLI{
				Action:        ActionWorkspaceScript,
				WorkspaceName: workspaceName,
				ScriptName:    scriptName,
				ScriptArgs:    parseInlineArgs(args[1:]),
			}, nil
		}
	}
```

with:

```go
	// check for @workspace/script syntax
	if strings.HasPrefix(cmd, "@") {
		inner := cmd[1:] // strip leading @
		lastSlash := strings.LastIndex(inner, "/")
		if lastSlash < 0 {
			return ParsedCLI{}, fmt.Errorf("invalid workspace command %q: usage: @<workspace>/<script>", cmd)
		}
		workspaceName := inner[:lastSlash]
		scriptName := inner[lastSlash+1:]
		if workspaceName == "" || scriptName == "" {
			return ParsedCLI{}, fmt.Errorf("invalid workspace command %q: usage: @<workspace>/<script>", cmd)
		}
		return ParsedCLI{
			Action:        ActionWorkspaceScript,
			WorkspaceName: workspaceName,
			ScriptName:    scriptName,
			ScriptArgs:    parseInlineArgs(args[1:]),
		}, nil
	}
```

Also add `"fmt"` to the import block if not already present (check — it is not currently imported in router.go; add it).

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd apps/miso && go test ./internal/cli/... -run "TestParseAt|TestParseColon" -v
```

Expected: PASS all 5 tests.

- [ ] **Step 5: Run full test suite to check for regressions**

```bash
cd apps/miso && go test ./...
```

Expected: all existing tests pass.

- [ ] **Step 6: Commit**

```bash
git add apps/miso/internal/cli/router.go apps/miso/internal/cli/router_test.go
git commit -m "feat: replace workspace:script colon syntax with @workspace/script"
```

---

## Task 2: Config — redesign `FindWorkspace`

**Files:**
- Modify: `apps/miso/internal/config/config.go:493-503`
- Modify: `apps/miso/internal/config/config_test.go` (extend)

The new `FindWorkspace` matches an identifier against each workspace using three candidates in order: directory basename, relative path from root, and `name` field from workspace's `package.json`. Returns an error (not bool) to carry "not found" and "ambiguous" messages.

- [ ] **Step 1: Add failing tests to config_test.go**

Add these test functions to the bottom of `apps/miso/internal/config/config_test.go`:

```go
func TestFindWorkspaceByBasename(t *testing.T) {
	root := t.TempDir()
	wsDir := filepath.Join(root, "packages", "api")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	workspaces := []string{wsDir}

	got, err := FindWorkspace("api", workspaces, root)
	if err != nil {
		t.Fatalf("FindWorkspace() error: %v", err)
	}
	if got != wsDir {
		t.Errorf("got %q, want %q", got, wsDir)
	}
}

func TestFindWorkspaceByRelativePath(t *testing.T) {
	root := t.TempDir()
	wsDir := filepath.Join(root, "packages", "api")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	workspaces := []string{wsDir}

	got, err := FindWorkspace("packages/api", workspaces, root)
	if err != nil {
		t.Fatalf("FindWorkspace() error: %v", err)
	}
	if got != wsDir {
		t.Errorf("got %q, want %q", got, wsDir)
	}
}

func TestFindWorkspaceByPackageName(t *testing.T) {
	root := t.TempDir()
	wsDir := filepath.Join(root, "packages", "api")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pkgJSON := `{"name": "@myorg/api"}`
	if err := os.WriteFile(filepath.Join(wsDir, "package.json"), []byte(pkgJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	workspaces := []string{wsDir}

	got, err := FindWorkspace("@myorg/api", workspaces, root)
	if err != nil {
		t.Fatalf("FindWorkspace() error: %v", err)
	}
	if got != wsDir {
		t.Errorf("got %q, want %q", got, wsDir)
	}
}

func TestFindWorkspaceAmbiguous(t *testing.T) {
	root := t.TempDir()
	ws1 := filepath.Join(root, "apps", "api")
	ws2 := filepath.Join(root, "packages", "api")
	for _, d := range []string{ws1, ws2} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	workspaces := []string{ws1, ws2}

	_, err := FindWorkspace("api", workspaces, root)
	if err == nil {
		t.Error("expected error for ambiguous match, got nil")
	}
}

func TestFindWorkspaceNotFound(t *testing.T) {
	root := t.TempDir()
	wsDir := filepath.Join(root, "packages", "api")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	workspaces := []string{wsDir}

	_, err := FindWorkspace("web", workspaces, root)
	if err == nil {
		t.Error("expected error for not found, got nil")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd apps/miso && go test ./internal/config/... -run "TestFindWorkspace" -v
```

Expected: FAIL — `FindWorkspace` has wrong signature and logic.

- [ ] **Step 3: Rewrite FindWorkspace in config.go**

Replace the existing `FindWorkspace` function (lines 493–503) with the new function plus four private helpers. All helpers are package-private (`config` package) — `joinStrings` and `joinWorkspaceNames` are NOT imported from the `scripting` package; they are defined here independently.

```go
// FindWorkspace matches a workspace identifier against discovered workspace paths.
// The identifier is matched against three candidates per workspace, in order:
//  1. Directory basename (e.g. "api" for /root/packages/api)
//  2. Relative path from root (e.g. "packages/api")
//  3. The "name" field in the workspace's package.json (e.g. "@myorg/api")
//
// If the identifier matches more than one workspace, an error is returned listing
// the conflicting paths. If no match is found, an error is returned listing available
// workspace basenames.
func FindWorkspace(name string, workspaces []string, root string) (string, error) {
	var matches []string

	for _, ws := range workspaces {
		if matchesWorkspace(name, ws, root) {
			matches = append(matches, ws)
		}
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("workspace %q not found (available: %s)", name, joinWorkspaceNames(workspaces))
	default:
		var paths []string
		for _, m := range matches {
			rel, err := filepath.Rel(root, m)
			if err != nil {
				paths = append(paths, m)
			} else {
				paths = append(paths, rel)
			}
		}
		return "", fmt.Errorf("workspace %q is ambiguous — matches multiple workspaces: %s (use a more specific identifier)", name, joinStrings(paths))
	}
}

// matchesWorkspace returns true if name matches the given workspace path
// by basename, relative path from root, or package.json name field.
func matchesWorkspace(name string, ws string, root string) bool {
	// 1. basename
	if filepath.Base(ws) == name {
		return true
	}

	// 2. relative path from root
	rel, err := filepath.Rel(root, ws)
	if err == nil && filepath.ToSlash(rel) == filepath.ToSlash(name) {
		return true
	}

	// 3. package.json name field
	pkgName := readPackageJSONName(ws)
	if pkgName != "" && pkgName == name {
		return true
	}

	return false
}

// readPackageJSONName reads the "name" field from a workspace's package.json.
// Returns empty string if the file is missing or the field is absent.
func readPackageJSONName(wsDir string) string {
	data, err := os.ReadFile(filepath.Join(wsDir, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	return pkg.Name
}

// joinWorkspaceNames returns a comma-separated list of workspace basenames.
// Defined here (config package) — also duplicated in scripting package; do NOT import across packages.
func joinWorkspaceNames(workspaces []string) string {
	names := make([]string, 0, len(workspaces))
	for _, ws := range workspaces {
		names = append(names, filepath.Base(ws))
	}
	return joinStrings(names)
}

// joinStrings joins a slice of strings with ", ".
// Defined here (config package) — also duplicated in scripting package; do NOT import across packages.
func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}
```

Also add `"fmt"` to the config.go import if not already present (check existing imports — `fmt` is likely already there).

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd apps/miso && go test ./internal/config/... -run "TestFindWorkspace" -v
```

Expected: PASS all 5 tests.

- [ ] **Step 5: Run full test suite**

```bash
cd apps/miso && go test ./...
```

Expected: compile errors in `workspace.go` because `FindWorkspace` signature changed from `(string, []string) (string, bool)` to `(string, []string, string) (string, error)`. Note these — they will be fixed in Task 3.

- [ ] **Step 6: Commit**

```bash
git add apps/miso/internal/config/config.go apps/miso/internal/config/config_test.go
git commit -m "feat: redesign FindWorkspace to match by basename, path, and package name"
```

---

## Task 3: Scripting — update workspace resolver

**Files:**
- Modify: `apps/miso/internal/cli/scripting/workspace.go`
- Create: `apps/miso/internal/cli/scripting/workspace_test.go`

- [ ] **Step 1: Create workspace_test.go with failing tests**

```go
package scripting

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ekkolyth/miso/internal/config"
)

func setupWorkspace(t *testing.T) (root string, wsDir string) {
	t.Helper()
	root = t.TempDir()
	wsDir = filepath.Join(root, "packages", "api")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// write root package.json declaring the workspace
	rootPkg := `{"workspaces": ["packages/api"]}`
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(rootPkg), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, wsDir
}

func TestResolveWorkspaceScriptFromFolder(t *testing.T) {
	root, wsDir := setupWorkspace(t)
	scriptsDir := filepath.Join(wsDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "build.sh"), []byte("#!/bin/sh\necho build"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Scripts: "./scripts", Repo: "mono"}

	resolved, workDir, err := ResolveWorkspaceScript("api", "build", root, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Source != ScriptSourceFolder {
		t.Errorf("Source = %v, want ScriptSourceFolder", resolved.Source)
	}
	if workDir != wsDir {
		t.Errorf("workDir = %q, want %q", workDir, wsDir)
	}
}

func TestResolveWorkspaceScriptFromPackageJSON(t *testing.T) {
	root, wsDir := setupWorkspace(t)
	// no scripts/ folder — only package.json
	wsPkg := `{"scripts": {"test:unit": "vitest run"}}`
	if err := os.WriteFile(filepath.Join(wsDir, "package.json"), []byte(wsPkg), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Scripts: "./scripts", Repo: "mono"}

	resolved, workDir, err := ResolveWorkspaceScript("api", "test:unit", root, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Source != ScriptSourcePackageJSON {
		t.Errorf("Source = %v, want ScriptSourcePackageJSON", resolved.Source)
	}
	if resolved.Path != "vitest run" {
		t.Errorf("Path = %q, want %q", resolved.Path, "vitest run")
	}
	if workDir != wsDir {
		t.Errorf("workDir = %q, want %q", workDir, wsDir)
	}
}

func TestResolveWorkspaceScriptNotFound(t *testing.T) {
	root, wsDir := setupWorkspace(t)
	_ = wsDir
	cfg := config.Config{Scripts: "./scripts", Repo: "mono"}

	resolved, _, err := ResolveWorkspaceScript("api", "nonexistent", root, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Source != ScriptSourceNone {
		t.Errorf("Source = %v, want ScriptSourceNone", resolved.Source)
	}
}

func TestResolveWorkspaceScriptByPackageName(t *testing.T) {
	root, wsDir := setupWorkspace(t)
	// workspace has a scoped package name
	wsPkg := `{"name": "@myorg/api", "scripts": {"build": "tsc"}}`
	if err := os.WriteFile(filepath.Join(wsDir, "package.json"), []byte(wsPkg), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Scripts: "./scripts", Repo: "mono"}

	resolved, _, err := ResolveWorkspaceScript("@myorg/api", "build", root, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Source != ScriptSourcePackageJSON {
		t.Errorf("Source = %v, want ScriptSourcePackageJSON", resolved.Source)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd apps/miso && go test ./internal/cli/scripting/... -run "TestResolveWorkspaceScript" -v
```

Expected: FAIL — compile error from `FindWorkspace` signature change, and missing `package.json` fallback.

- [ ] **Step 3: Rewrite workspace.go**

Replace the entire content of `apps/miso/internal/cli/scripting/workspace.go`:

```go
package scripting

import (
	"fmt"
	"path/filepath"

	"github.com/ekkolyth/miso/internal/config"
)

// ResolveWorkspaceScript resolves a script by name within a specific workspace.
// workspaceName is matched against workspace paths by basename, relative path,
// or package.json name field. scriptName is the script to find.
// Returns the resolved script and the workspace directory as the working directory.
func ResolveWorkspaceScript(workspaceName string, scriptName string, root string, cfg config.Config) (ResolvedScript, string, error) {
	// load workspaces from package.json
	workspaces, err := config.LoadWorkspaces(root)
	if err != nil {
		return ResolvedScript{}, "", fmt.Errorf("load workspaces: %w", err)
	}

	if len(workspaces) == 0 {
		return ResolvedScript{}, "", fmt.Errorf("no workspaces found in package.json — is this a monorepo?")
	}

	// find the workspace directory — matches by basename, relative path, or package.json name
	workspaceDir, err := config.FindWorkspace(workspaceName, workspaces, root)
	if err != nil {
		return ResolvedScript{}, "", err
	}

	// determine the scripts folder for this workspace
	scriptsPath := cfg.Scripts
	if scriptsPath == "" {
		scriptsPath = "./scripts"
	}

	// resolve scripts path relative to workspace dir (not project root)
	if !filepath.IsAbs(scriptsPath) {
		scriptsPath = filepath.Join(workspaceDir, scriptsPath)
	}

	// 1. check workspace scripts/ folder
	discovered, err := DiscoverScripts(scriptsPath)
	if err != nil {
		return ResolvedScript{}, "", fmt.Errorf("discover workspace scripts: %w", err)
	}

	scripts, ok := discovered[scriptName]
	if ok {
		if len(scripts) > 1 {
			var paths []string
			for _, s := range scripts {
				paths = append(paths, s.RelativePath)
			}
			return ResolvedScript{}, "", fmt.Errorf("multiple scripts for %q exist in workspace %q: %s",
				scriptName, workspaceName, joinStrings(paths))
		}
		return ResolvedScript{
			Source: ScriptSourceFolder,
			Path:   scripts[0].Path,
		}, workspaceDir, nil
	}

	// 2. fall back to workspace package.json scripts
	pkgScripts, err := ReadPackageJSONScripts(workspaceDir)
	if err != nil {
		return ResolvedScript{}, "", fmt.Errorf("read workspace package.json scripts: %w", err)
	}
	if command, ok := pkgScripts[scriptName]; ok {
		return ResolvedScript{
			Source: ScriptSourcePackageJSON,
			Path:   command,
		}, workspaceDir, nil
	}

	// not found in either
	return ResolvedScript{Source: ScriptSourceNone}, workspaceDir, nil
}

// WorkspaceFromCWD checks whether the current working directory is inside a
// known workspace and returns the workspace directory if so.
func WorkspaceFromCWD(cwd string, workspaces []string) (string, bool) {
	for _, ws := range workspaces {
		absWs, err := filepath.Abs(ws)
		if err != nil {
			continue
		}
		absCwd, err := filepath.Abs(cwd)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(absWs, absCwd)
		if err != nil {
			continue
		}
		if rel == "." || (len(rel) > 0 && rel[0] != '.') {
			return absWs, true
		}
	}
	return "", false
}

// joinWorkspaceNames returns a comma-separated list of workspace short names.
func joinWorkspaceNames(workspaces []string) string {
	names := make([]string, 0, len(workspaces))
	for _, ws := range workspaces {
		names = append(names, filepath.Base(ws))
	}
	return joinStrings(names)
}

// joinStrings joins a slice of strings with ", ".
func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}
```

- [ ] **Step 4: Run workspace tests to verify they pass**

```bash
cd apps/miso && go test ./internal/cli/scripting/... -run "TestResolveWorkspaceScript" -v
```

Expected: PASS all 4 tests.

- [ ] **Step 5: Run full test suite**

```bash
cd apps/miso && go test ./...
```

Expected: compile error in `cmd/main.go` — `FindWorkspace` is called at line 218 with the old `(string, []string)` signature. Note it — fixed in Task 4.

- [ ] **Step 6: Commit**

```bash
git add apps/miso/internal/cli/scripting/workspace.go apps/miso/internal/cli/scripting/workspace_test.go
git commit -m "feat: extend ResolveWorkspaceScript with package.json fallback and updated FindWorkspace"
```

---

## Task 4: Execution — update main.go

**Files:**
- Modify: `apps/miso/cmd/main.go:217-224` (CWD-scoping block)
- Modify: `apps/miso/cmd/main.go:321-340` (ActionWorkspaceScript handler)

Two sites in main.go call `FindWorkspace` or depend on the old workspace routing:

**Site 1 — CWD-scoping block (lines 212–227):** Uses `filepath.Base(wsDir)` as the workspace name and calls `ResolveWorkspaceScript`. This still works because `filepath.Base` is still a valid match candidate — no logic change needed, but the `FindWorkspace` signature change means `ResolveWorkspaceScript` now takes the updated path internally.

**Site 2 — ActionWorkspaceScript handler (lines 321–340):** Only handles `ScriptSourceFolder`. Needs to also handle `ScriptSourcePackageJSON`.

- [ ] **Step 1: Update the CWD-scoping block (lines 212-227)**

The CWD-scoping block in `apps/miso/cmd/main.go` currently only promotes to `ActionWorkspaceScript` when `resolved.Source == scripting.ScriptSourceFolder`. After Task 3, `ResolveWorkspaceScript` can also return `ScriptSourcePackageJSON` (from the workspace's `package.json`). Update line 220 to promote on either source:

Replace:
```go
			if resolveErr == nil && resolved.Source == scripting.ScriptSourceFolder {
				parsed.Action = cli.ActionWorkspaceScript
				parsed.WorkspaceName = filepath.Base(wsDir)
				parsed.Command = resolved.Path
			}
```

With:
```go
			if resolveErr == nil && (resolved.Source == scripting.ScriptSourceFolder || resolved.Source == scripting.ScriptSourcePackageJSON) {
				parsed.Action = cli.ActionWorkspaceScript
				parsed.WorkspaceName = filepath.Base(wsDir)
				parsed.Command = resolved.Path
			}
```

- [ ] **Step 2: Update the ActionWorkspaceScript handler**

Replace lines 321–340 in `apps/miso/cmd/main.go`:

```go
	case cli.ActionWorkspaceScript:
		// @workspace/script syntax — resolve and execute in the workspace directory
		resolved, workDir, err := scripting.ResolveWorkspaceScript(
			parsed.WorkspaceName, parsed.ScriptName, projectRoot, cfg,
		)
		if err != nil {
			cli.Fail(logger, err, false)
		}
		switch resolved.Source {
		case scripting.ScriptSourceFolder:
			processEnv, envErr := env.BuildProcessEnv(projectRoot, cfg, workDir)
			if envErr != nil {
				cli.Fail(logger, envErr, false)
			}
			if err := scripting.ExecScriptFile(resolved.Path, parsed.ScriptArgs, workDir, cfg.Shell, processEnv); err != nil {
				cli.Fail(logger, err, false)
			}
		case scripting.ScriptSourcePackageJSON:
			if err := commands.Run(managerName, parsed.ScriptName, parsed.ScriptArgs, workDir); err != nil {
				cli.Fail(logger, err, false)
			}
		default:
			cli.Fail(logger, fmt.Errorf("script %q not found in workspace %q", parsed.ScriptName, parsed.WorkspaceName), false)
		}
		return
```

- [ ] **Step 3: Build to verify it compiles**

```bash
cd apps/miso && go build ./...
```

Expected: clean build, no errors.

- [ ] **Step 4: Run full test suite**

```bash
cd apps/miso && go test ./...
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add apps/miso/cmd/main.go
git commit -m "feat: handle ScriptSourcePackageJSON in workspace execution and CWD-scoping block"
```

---

## Task 5: Update miso-scripting SKILL.md

**Files:**
- Modify: `packages/skills/miso-scripting/SKILL.md:83-91`

- [ ] **Step 1: Update the workspace-scoped scripts section**

Replace lines 83–91 in `packages/skills/miso-scripting/SKILL.md`:

```markdown
## Workspace-Scoped Scripts (Monorepos)

In a monorepo (`repo: "mono"`), run a script in a specific workspace using the `@workspace/script` syntax:

```bash
miso @api/build           # run "build" in the workspace identified as "api"
miso @myorg/api/test      # run "test" in the workspace with package name "@myorg/api"
miso @packages/web/dev    # run "dev" in the workspace at path "packages/web"
miso @api/test:unit       # run "test:unit" (colons are fine in script names)
```

The workspace identifier can be any of:
- **Directory basename** — `api` for a workspace at `packages/api`
- **Relative path from root** — `packages/api`
- **Package name** — the `name` field in the workspace's `package.json` (e.g. `@myorg/api`)

If the identifier matches more than one workspace, miso will error and list the conflicting paths.

Miso resolves the script from that workspace's own `scripts/` folder first, then falls back to its `package.json` scripts.
```

- [ ] **Step 2: Commit**

```bash
git add packages/skills/miso-scripting/SKILL.md
git commit -m "docs: update miso-scripting skill with @workspace/script syntax"
```

---

## Final Verification

- [ ] **Step 1: Run full test suite one last time**

```bash
cd apps/miso && go test ./... -v 2>&1 | tail -30
```

Expected: all tests pass, no failures.

- [ ] **Step 2: Build the binary**

```bash
cd apps/miso && go build -o /tmp/miso-test ./cmd/main.go
```

Expected: clean build.
