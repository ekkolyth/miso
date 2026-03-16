# Batch Env Validation Errors Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collect and display all env validation errors at once instead of failing on the first one.

**Architecture:** Change `validateVariables()` and `runEntry()` to return `[]error` instead of `error`. `Run()` accumulates errors per label and formats a grouped error summary at the end.

**Tech Stack:** Go standard library, existing `go-playground/validator` dependency unchanged.

---

## File Structure

- **Modify:** `apps/miso/internal/cli/env/validator.go` — change `validateVariables()` return type to `[]error`
- **Modify:** `apps/miso/internal/cli/env/env.go` — change `runEntry()` return type, add error accumulation and formatting in `Run()`
- **Create:** `apps/miso/internal/cli/env/validator_test.go` — tests for `validateVariables()`
- **Create:** `apps/miso/internal/cli/env/env_test.go` — tests for `runEntry()` and `Run()` error formatting

---

## Chunk 1: Batch validation in validateVariables

### Task 1: Make validateVariables return all errors

**Files:**
- Modify: `apps/miso/internal/cli/env/validator.go:23-81`
- Create: `apps/miso/internal/cli/env/validator_test.go`

- [ ] **Step 1: Write failing test — multiple errors collected**

Create `apps/miso/internal/cli/env/validator_test.go`:

```go
package env

import (
	"testing"

	"github.com/ekkolyth/miso/internal/config"
)

func TestValidateVariables_CollectsAllErrors(t *testing.T) {
	vars := map[string]config.VarConfigOrString{
		"APP_SECRET":  {IsShorthand: true, Type: "string"},
		"DB_PORT":     {IsShorthand: true, Type: "port"},
		"API_URL":     {IsShorthand: true, Type: "url"},
	}
	required := config.EnvRequired{Mode: "all"}

	// envMap missing APP_SECRET, has invalid port and invalid url
	envMap := map[string]string{
		"DB_PORT": "not-a-number",
		"API_URL": "not-a-url",
	}

	errs := validateVariables(envMap, vars, required)
	if len(errs) != 3 {
		t.Fatalf("got %d errors, want 3: %v", len(errs), errs)
	}
}

func TestValidateVariables_NoErrors(t *testing.T) {
	vars := map[string]config.VarConfigOrString{
		"PORT": {IsShorthand: true, Type: "port"},
	}
	required := config.EnvRequired{Mode: "all"}
	envMap := map[string]string{"PORT": "8080"}

	errs := validateVariables(envMap, vars, required)
	if len(errs) != 0 {
		t.Fatalf("got %d errors, want 0: %v", len(errs), errs)
	}
}

func TestValidateVariables_MixedMissingAndInvalid(t *testing.T) {
	vars := map[string]config.VarConfigOrString{
		"REQUIRED_VAR": {IsShorthand: true, Type: "string"},
		"BAD_PORT":     {IsShorthand: true, Type: "port"},
	}
	required := config.EnvRequired{Mode: "all"}
	envMap := map[string]string{
		"BAD_PORT": "99999",
	}

	errs := validateVariables(envMap, vars, required)
	if len(errs) != 2 {
		t.Fatalf("got %d errors, want 2: %v", len(errs), errs)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/miso && go test ./internal/cli/env/ -run TestValidateVariables -v`
Expected: compilation error — `validateVariables` returns `error` not `[]error`

- [ ] **Step 3: Change validateVariables to return []error**

In `apps/miso/internal/cli/env/validator.go`, change `validateVariables` to:

```go
func validateVariables(envMap map[string]string, vars map[string]config.VarConfigOrString, required config.EnvRequired) []error {
	validate := validator.New()

	// Register custom pattern validator for dynamic regex
	validate.RegisterValidation("matches_regex", func(fl validator.FieldLevel) bool {
		pattern := fl.Param()
		if pattern == "" {
			return false
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(fl.Field().String())
	})

	names := make([]string, 0, len(vars))
	for name := range vars {
		names = append(names, name)
	}
	sort.Strings(names)

	var errs []error

	for _, name := range names {
		v := vars[name]
		var cfg config.VarConfig
		if v.IsShorthand {
			if v.Type == "pattern" || v.Type == "enum" {
				errs = append(errs, fmt.Errorf("variable %s: type %s cannot use shorthand (requires pattern/values)", name, v.Type))
				continue
			}
			if !shorthandTypes[v.Type] {
				errs = append(errs, fmt.Errorf("variable %s: unknown type %s", name, v.Type))
				continue
			}
			cfg = config.VarConfig{Type: v.Type, Optional: false}
		} else {
			cfg = v.Config
		}

		val, ok := envMap[name]
		if !ok {
			if isRequired(name, required, vars) {
				errs = append(errs, fmt.Errorf("missing required variable: %s", name))
			}
			continue
		}

		// Optional variables with empty values are allowed — skip validation.
		if cfg.Optional && val == "" {
			continue
		}

		if err := validateVar(validate, name, val, cfg); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}
```

- [ ] **Step 4: Fix compilation in env.go**

In `apps/miso/internal/cli/env/env.go`, temporarily update the call site at line 115 to keep things compiling while we work on the next task:

```go
	// Object mode — full type validation
	if errs := validateVariables(envMap, entry.Variables.Object, entry.Required); len(errs) > 0 {
		return fmt.Errorf("%s: %s", label, errs[0])
	}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd apps/miso && go test ./internal/cli/env/ -run TestValidateVariables -v`
Expected: all 3 tests PASS

- [ ] **Step 6: Commit**

```bash
git add apps/miso/internal/cli/env/validator.go apps/miso/internal/cli/env/validator_test.go apps/miso/internal/cli/env/env.go
git commit -m "feat: make validateVariables collect all errors instead of fail-fast"
```

---

## Chunk 2: Batch errors in runEntry and Run, formatted output

### Task 2: Make runEntry return []error and update Run with grouped output

**Files:**
- Modify: `apps/miso/internal/cli/env/env.go:58-119`
- Create: `apps/miso/internal/cli/env/env_test.go`

- [ ] **Step 1: Write failing test — runEntry collects multiple array-mode errors**

Create `apps/miso/internal/cli/env/env_test.go`:

```go
package env

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/log"

	"github.com/ekkolyth/miso/internal/config"
)

// writeTempEnv creates a temp .env file and returns the project root dir.
func writeTempEnv(t *testing.T, filename, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp env: %v", err)
	}
	return dir
}

func TestRunEntry_ArrayMode_CollectsAllMissing(t *testing.T) {
	dir := writeTempEnv(t, ".env", "EXISTING=value\n")
	entry := &config.EnvEntry{
		Label: "test",
		Path:  ".env",
		Variables: config.EnvVariables{
			Array: []string{"MISSING_A", "MISSING_B", "EXISTING"},
		},
	}
	logger := log.New(os.Stderr)

	errs := runEntry(dir, entry, logger)
	if len(errs) != 2 {
		t.Fatalf("got %d errors, want 2: %v", len(errs), errs)
	}
}

func TestRunEntry_ObjectMode_CollectsAllErrors(t *testing.T) {
	dir := writeTempEnv(t, ".env", "BAD_PORT=abc\n")
	entry := &config.EnvEntry{
		Label: "test",
		Path:  ".env",
		Variables: config.EnvVariables{
			Object: map[string]config.VarConfigOrString{
				"MISSING_VAR": {IsShorthand: true, Type: "string"},
				"BAD_PORT":    {IsShorthand: true, Type: "port"},
			},
		},
		Required: config.EnvRequired{Mode: "all"},
	}
	logger := log.New(os.Stderr)

	errs := runEntry(dir, entry, logger)
	if len(errs) != 2 {
		t.Fatalf("got %d errors, want 2: %v", len(errs), errs)
	}
}

func TestRunEntry_FileNotFound_SingleError(t *testing.T) {
	dir := t.TempDir()
	entry := &config.EnvEntry{
		Label: "test",
		Path:  "nonexistent.env",
	}
	logger := log.New(os.Stderr)

	errs := runEntry(dir, entry, logger)
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(errs), errs)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/miso && go test ./internal/cli/env/ -run TestRunEntry -v`
Expected: compilation error — `runEntry` returns `error` not `[]error`

- [ ] **Step 3: Change runEntry to return []error**

In `apps/miso/internal/cli/env/env.go`, replace `runEntry`:

```go
// runEntry resolves, loads, and validates a single EnvEntry.
// Returns a slice of validation errors (nil if all passed).
func runEntry(projectRoot string, entry *config.EnvEntry, logger *log.Logger) []error {
	label := entryLabel(entry)

	// Resolve path
	absPath, err := resolveEntryPath(projectRoot, entry)
	if err != nil {
		return []error{err}
	}

	// Load env file
	envMap, err := loadEnvFile(absPath)
	if err != nil {
		return []error{err}
	}

	// No variables defined: just report the load
	if len(entry.Variables.Object) == 0 && len(entry.Variables.Array) == 0 {
		logger.Info("env loaded", "label", label, "path", absPath)
		return nil
	}

	// Presence-only (array) mode
	if len(entry.Variables.Array) > 0 {
		var errs []error
		for _, key := range entry.Variables.Array {
			if _, ok := envMap[key]; !ok {
				errs = append(errs, fmt.Errorf("missing required variable: %s", key))
			}
		}
		if len(errs) > 0 {
			return errs
		}
		logger.Info("env validation passed", "label", label, "variables", len(entry.Variables.Array))
		return nil
	}

	// Object mode — full type validation
	if errs := validateVariables(envMap, entry.Variables.Object, entry.Required); len(errs) > 0 {
		return errs
	}
	logger.Info("env validation passed", "label", label, "variables", len(entry.Variables.Object))
	return nil
}
```

- [ ] **Step 4: Run runEntry tests to verify they pass**

Run: `cd apps/miso && go test ./internal/cli/env/ -run TestRunEntry -v`
Expected: all 3 tests PASS

- [ ] **Step 5: Commit**

```bash
git add apps/miso/internal/cli/env/env.go apps/miso/internal/cli/env/env_test.go
git commit -m "feat: make runEntry collect all validation errors"
```

### Task 3: Update Run() with error accumulation and grouped formatting

**Files:**
- Modify: `apps/miso/internal/cli/env/env.go:58-79`
- Modify: `apps/miso/internal/cli/env/env_test.go`

- [ ] **Step 1: Write failing test — Run collects errors from multiple entries**

Add to `apps/miso/internal/cli/env/env_test.go`:

```go
func TestRun_GroupedErrors_MultipleEntries(t *testing.T) {
	dir := writeTempEnv(t, "a.env", "")
	// write a second env file
	if err := os.WriteFile(filepath.Join(dir, "b.env"), []byte("BAD_PORT=abc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Env: []*config.EnvEntry{
			{
				Label: "alpha",
				Path:  "a.env",
				Variables: config.EnvVariables{
					Array: []string{"MISSING_ONE", "MISSING_TWO"},
				},
			},
			{
				Label: "beta",
				Path:  "b.env",
				Variables: config.EnvVariables{
					Object: map[string]config.VarConfigOrString{
						"BAD_PORT": {IsShorthand: true, Type: "port"},
					},
				},
				Required: config.EnvRequired{Mode: "none"},
			},
		},
	}
	logger := log.New(os.Stderr)

	err := Run(dir, cfg, logger)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	msg := err.Error()
	// Should mention both labels
	if !strings.Contains(msg,"alpha") {
		t.Errorf("error should mention alpha: %s", msg)
	}
	if !strings.Contains(msg,"beta") {
		t.Errorf("error should mention beta: %s", msg)
	}
	// Should mention all individual errors
	if !strings.Contains(msg,"MISSING_ONE") {
		t.Errorf("error should mention MISSING_ONE: %s", msg)
	}
	if !strings.Contains(msg,"MISSING_TWO") {
		t.Errorf("error should mention MISSING_TWO: %s", msg)
	}
	if !strings.Contains(msg,"BAD_PORT") {
		t.Errorf("error should mention BAD_PORT: %s", msg)
	}
}

func TestRun_SuccessfulEntries_StillPass(t *testing.T) {
	dir := writeTempEnv(t, "good.env", "PORT=8080\n")
	cfg := config.Config{
		Env: []*config.EnvEntry{
			{
				Label: "good",
				Path:  "good.env",
				Variables: config.EnvVariables{
					Object: map[string]config.VarConfigOrString{
						"PORT": {IsShorthand: true, Type: "port"},
					},
				},
				Required: config.EnvRequired{Mode: "all"},
			},
		},
	}
	logger := log.New(os.Stderr)

	err := Run(dir, cfg, logger)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestRun_PassingEntryLogsInfo_WhenSiblingFails(t *testing.T) {
	dir := writeTempEnv(t, "good.env", "PORT=8080\n")
	if err := os.WriteFile(filepath.Join(dir, "bad.env"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Env: []*config.EnvEntry{
			{
				Label: "good",
				Path:  "good.env",
				Variables: config.EnvVariables{
					Object: map[string]config.VarConfigOrString{
						"PORT": {IsShorthand: true, Type: "port"},
					},
				},
				Required: config.EnvRequired{Mode: "all"},
			},
			{
				Label: "bad",
				Path:  "bad.env",
				Variables: config.EnvVariables{
					Array: []string{"MISSING_VAR"},
				},
			},
		},
	}

	// Capture log output to a buffer
	var buf strings.Builder
	logger := log.NewWithOptions(&buf, log.Options{})

	err := Run(dir, cfg, logger)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// The good entry should have logged its INFO line despite bad entry failing
	logOutput := buf.String()
	if !strings.Contains(logOutput, "good") {
		t.Errorf("expected INFO log for passing entry 'good', got: %s", logOutput)
	}
}

```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/miso && go test ./internal/cli/env/ -run TestRun_ -v`
Expected: FAIL — `Run` still returns on first entry failure

- [ ] **Step 3: Update Run() with error accumulation and formatting**

In `apps/miso/internal/cli/env/env.go`, add `strings` to imports and replace `Run`:

```go
// entryErrors holds errors for a single entry, preserving order.
type entryErrors struct {
	label string
	errs  []error
}

// Run executes the miso env command: for each EnvEntry, resolve its path, load the file,
// validate variables, and report results. When no env config is present, falls back to
// discovery mode and reports which file was found.
func Run(projectRoot string, cfg config.Config, logger *log.Logger) error {
	if len(cfg.Env) == 0 {
		// No config: discovery mode
		path, err := discoverEnvFile(projectRoot)
		if err != nil {
			return err
		}
		logger.Info("env loaded", "path", path)
		return nil
	}

	var failures []entryErrors

	for _, entry := range cfg.Env {
		errs := runEntry(projectRoot, entry, logger)
		if len(errs) > 0 {
			failures = append(failures, entryErrors{
				label: entryLabel(entry),
				errs:  errs,
			})
		}
	}

	if len(failures) == 0 {
		return nil
	}

	return formatGroupedErrors(failures)
}

// formatGroupedErrors builds a single error with all failures grouped by label.
func formatGroupedErrors(failures []entryErrors) error {
	var b strings.Builder
	b.WriteString("env validation failed:")
	for _, f := range failures {
		fmt.Fprintf(&b, "\n  %s:", f.label)
		for _, e := range f.errs {
			fmt.Fprintf(&b, "\n    - %s", e.Error())
		}
	}
	return fmt.Errorf("%s", b.String())
}
```

Also add `"strings"` to the import block in `env.go`.

- [ ] **Step 4: Run all tests to verify they pass**

Run: `cd apps/miso && go test ./internal/cli/env/ -v`
Expected: all tests PASS

- [ ] **Step 5: Commit**

```bash
git add apps/miso/internal/cli/env/env.go apps/miso/internal/cli/env/env_test.go
git commit -m "feat: accumulate and display grouped env validation errors"
```

---

## Chunk 3: Clean up and final verification

### Task 4: Remove temporary shim, verify full build

**Files:**
- Modify: `apps/miso/internal/cli/env/env.go` (if any temporary compatibility code remains)

- [ ] **Step 1: Verify no temporary shims remain**

Read `apps/miso/internal/cli/env/env.go` and confirm that the temporary `errs[0]` shim from Task 1 Step 4 has been replaced by the proper implementation from Task 3.

- [ ] **Step 2: Run full test suite**

Run: `cd apps/miso && go test ./... -v`
Expected: all tests PASS, no compilation errors

- [ ] **Step 3: Run go vet**

Run: `cd apps/miso && go vet ./...`
Expected: no issues

- [ ] **Step 4: Commit if any cleanup was needed**

Only if changes were made:
```bash
git add apps/miso/internal/cli/env/env.go apps/miso/internal/cli/env/validator.go && git commit -m "chore: clean up batch env validation implementation"
```
