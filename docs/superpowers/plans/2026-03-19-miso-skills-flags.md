# Miso Skills CLI Flags Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `miso skills --add` and `miso skills --rm` flags that invoke the `vercel-labs/skills` CLI with the correct subpath URL baked in, while letting `miso skills <anything-else>` fall through to the package manager.

**Architecture:** New `internal/cli/commands/skills.go` file exports three functions: `ParseSkillsFlags`, `RunSkillsAdd`, `RunSkillsRemove`. A new `case "skills":` block in `cmd/main.go` calls them before config is loaded, consistent with the existing `upgrade` pattern.

**Tech Stack:** Go, `github.com/ekkolyth/miso/internal/manager` (Exec pattern)

---

### Task 1: Write failing tests for `skills.go`

**Files:**
- Create: `apps/miso/internal/cli/commands/skills_test.go`

- [ ] **Step 1: Create the test file**

```go
package commands_test

import (
	"testing"

	"github.com/ekkolyth/miso/internal/cli/commands"
)

func TestParseSkillsFlags_AddOnly(t *testing.T) {
	add, rm := commands.ParseSkillsFlags([]string{"--add"})
	if !add {
		t.Error("expected add=true")
	}
	if rm {
		t.Error("expected rm=false")
	}
}

func TestParseSkillsFlags_RmOnly(t *testing.T) {
	add, rm := commands.ParseSkillsFlags([]string{"--rm"})
	if add {
		t.Error("expected add=false")
	}
	if !rm {
		t.Error("expected rm=true")
	}
}

func TestParseSkillsFlags_Both(t *testing.T) {
	add, rm := commands.ParseSkillsFlags([]string{"--add", "--rm"})
	if !add {
		t.Error("expected add=true")
	}
	if !rm {
		t.Error("expected rm=true")
	}
}

func TestParseSkillsFlags_Neither(t *testing.T) {
	add, rm := commands.ParseSkillsFlags([]string{"add", "lodash"})
	if add {
		t.Error("expected add=false")
	}
	if rm {
		t.Error("expected rm=false")
	}
}

func TestParseSkillsFlags_Empty(t *testing.T) {
	add, rm := commands.ParseSkillsFlags([]string{})
	if add {
		t.Error("expected add=false")
	}
	if rm {
		t.Error("expected rm=false")
	}
}

func TestParseSkillsFlags_ExtraArgs(t *testing.T) {
	add, rm := commands.ParseSkillsFlags([]string{"--add", "--verbose"})
	if !add {
		t.Error("expected add=true")
	}
	if rm {
		t.Error("expected rm=false")
	}
}
```

Save to `apps/miso/internal/cli/commands/skills_test.go`.

- [ ] **Step 2: Run the test to verify it fails (function doesn't exist yet)**

Run: `go test ./internal/cli/commands/... -run TestParseSkillsFlags -v`
Expected: compile error — `commands.ParseSkillsFlags undefined`

---

### Task 2: Implement `internal/cli/commands/skills.go`

**Files:**
- Create: `apps/miso/internal/cli/commands/skills.go`

- [ ] **Step 1: Create the implementation**

```go
package commands

import "github.com/ekkolyth/miso/internal/manager"

// ParseSkillsFlags scans args for --add and --rm flags.
// Returns (add, rm) booleans. Does not modify args.
func ParseSkillsFlags(args []string) (add bool, rm bool) {
	for _, arg := range args {
		switch arg {
		case "--add":
			add = true
		case "--rm":
			rm = true
		}
	}
	return
}

// RunSkillsAdd runs: npx skills add "<repo-url>" --all
func RunSkillsAdd() error {
	spec := manager.ExecSpec{
		Command: "npx",
		Args:    []string{"skills", "add", "https://github.com/ekkolyth/miso/tree/main/packages/skills", "--all"},
	}
	return manager.Exec(spec, "")
}

// RunSkillsRemove runs: npx skills remove miso-config miso-scripting miso-tui miso-env
func RunSkillsRemove() error {
	spec := manager.ExecSpec{
		Command: "npx",
		Args:    []string{"skills", "remove", "miso-config", "miso-scripting", "miso-tui", "miso-env"},
	}
	return manager.Exec(spec, "")
}
```

Save to `apps/miso/internal/cli/commands/skills.go`.

- [ ] **Step 2: Run the tests to verify they pass**

Run: `go test ./internal/cli/commands/... -run TestParseSkillsFlags -v`
Expected: all 6 tests PASS

- [ ] **Step 3: Commit**

```bash
git add apps/miso/internal/cli/commands/skills.go apps/miso/internal/cli/commands/skills_test.go
git commit -m "feat: add ParseSkillsFlags, RunSkillsAdd, RunSkillsRemove"
```

---

### Task 3: Add `case "skills":` to `cmd/main.go`

**Files:**
- Modify: `apps/miso/cmd/main.go:100-106`

The `case "upgrade":` block ends at line 105 (the closing `}`), before the outer `}` on line 106. Insert the new `case "skills":` block immediately after.

- [ ] **Step 1: Add the case block**

In `apps/miso/cmd/main.go`, locate:

```go
	case "upgrade":
		local, remainingArgs := cli.ParseLocalFlag(args[1:])
		if err := commands.Upgrade(local, remainingArgs); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	}
```

Replace with:

```go
	case "upgrade":
		local, remainingArgs := cli.ParseLocalFlag(args[1:])
		if err := commands.Upgrade(local, remainingArgs); err != nil {
			cli.Fail(logger, err, false)
		}
		return
	case "skills":
		add, rm := commands.ParseSkillsFlags(args[1:])
		if add && rm {
			cli.Fail(logger, fmt.Errorf("--add and --rm are mutually exclusive"), false)
		}
		if add {
			if err := commands.RunSkillsAdd(); err != nil {
				cli.Fail(logger, err, false)
			}
			return
		}
		if rm {
			if err := commands.RunSkillsRemove(); err != nil {
				cli.Fail(logger, err, false)
			}
			return
		}
		// Neither --add nor --rm: fall through to normal routing (PM passthrough)
	}
```

Note the absence of `return` or `break` at the end of the `skills` case — execution continues past the `switch` block into the project root lookup, eventually reaching `ActionPassthrough` so `miso skills add lodash` still works.

- [ ] **Step 2: Verify the file compiles**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 3: Run the full test suite**

Run: `go test ./...`
Expected: all tests pass (no new failures)

- [ ] **Step 4: Commit**

```bash
git add apps/miso/cmd/main.go
git commit -m "feat: add miso skills --add and --rm CLI flags"
```

---

### Task 4: Manual smoke test

**No files changed.** Verification only.

- [ ] **Step 1: Build the binary**

Run: `go build -o /tmp/miso-test ./cmd/miso`
(Run from `apps/miso/` directory.)

- [ ] **Step 2: Verify `--add` and `--rm` mutually exclusive error**

Run: `/tmp/miso-test skills --add --rm`
Expected: exits non-zero with an error message containing `--add and --rm are mutually exclusive`

- [ ] **Step 3: Verify neither flag falls through**

Run: `/tmp/miso-test skills` (from a directory with a valid miso project)
Expected: falls through to PM passthrough (equivalent to `bun skills` / `npm skills` etc.) — same behavior as before this change

- [ ] **Step 4: Clean up**

```bash
rm /tmp/miso-test
```
