# Simple Mode: Package-Manager-Free Script Execution

**Date:** 2026-03-17
**Status:** Draft
**Branch:** feature/tui-script-execution

## Problem

Miso is a package manager wrapper at its core, but most of its features — script execution, TUI, env validation, task orchestration — are language-agnostic. Currently, miso requires a lockfile, `package.json`, or `node_modules` directory to even find the project root. This forces non-JS users (e.g., Go, Rust, Python projects) to create a `package.json` just to use miso's script runner and orchestration features.

## Solution

Add a `"packageManager": false` config option to `miso.json` that disables all package manager features. When set, miso operates as a pure script runner and task orchestrator — no PM detection, no built-in commands, everything the user types resolves as a script.

## Design

### Config

A new top-level `"packageManager"` boolean field in `miso.json`:

```json
{
  "$schema": "https://misojs.dev/miso.schema.json",
  "packageManager": false,
  "scripts": "./scripts",
  "shell": "bash",
  "tui": { "mode": "tabbed" },
  "env": [...]
}
```

- **Default:** `true` when absent. Existing configs are unaffected. `"packageManager": true` is accepted and equivalent to omission.
- **Schema:** Add `"packageManager"` as a boolean to `miso.schema.json` with default `true`.
- **Go implementation:** Use `*bool` in the `Config` struct so that absent (nil) is distinguishable from explicit `false`. Treat nil as `true`. The `Config.Save` function must preserve the `packageManager` field during serialization (round-trip safe).

### Project Root Detection

`FindProjectRoot` already checks for `miso.json` first. When `packageManager: false`, the lockfile and `node_modules` fallbacks are irrelevant — `miso.json` must exist and serves as the sole project root anchor.

### Command Routing

When `packageManager: false`, skip all built-in PM command matching in the router. Every CLI argument goes straight to script resolution.

```
CLI input → is packageManager false?
  YES → resolve as script (except miso meta-commands)
  NO  → current routing (match built-in commands, then fall through to scripts)
```

**Examples in simple mode:**

| Input | Behavior |
|-------|----------|
| `miso build` | Looks for `scripts/build.sh` (or matching extension) |
| `miso run build` | Looks for `scripts/run` — `run` is NOT special, it's a script name |
| `miso install` | Looks for `scripts/install.sh`, fails if not found |
| `miso dev` | Looks for `scripts/dev.sh`, TUI kicks in if configured |

**Miso meta-commands that remain built-in in simple mode:**

- `miso env` — env validation
- `miso init` — project initialization
- `miso version` — version info
- `miso upgrade` — self-update
- `miso scripts` — list discovered scripts
- `miso completion` — shell completion

These are miso's own commands, not PM commands.

**Error behavior:** If a command is not a meta-command and not a discovered script, fail with: `script '<name>' not found in ./scripts`. The `ActionPassthrough` code path (which forwards to a PM) is never reached in simple mode.

### Script Resolution in Simple Mode

`ResolveScript` must skip the `package.json` fallback when `packageManager: false`. Only folder scripts are searched. If a `package.json` happens to exist in a simple-mode project (e.g., for unrelated tooling), its `scripts` field is ignored.

### TUI in Simple Mode

The TUI `Launch` function currently requires a `manager.Manager` argument. In simple mode, there is no manager. `Launch` must be modified to accept an optional manager (nil-safe). The `package.json` script branch inside `Launch` (which calls `mgr.BuildRun()`) is unreachable in simple mode since `ResolveScript` never produces `package.json` scripts — but the nil guard prevents a crash if the code path is hit unexpectedly.

### `miso init` Flow

```
miso init
│
├── Detects JS project? (lockfile or package.json exists)
│   │
│   ├── package.json has `packageManager` field?
│   │   └── YES → Use that PM, proceed with setup
│   │
│   └── NO → Ask which PM (bun, npm, pnpm, yarn), proceed with setup
│
└── No JS project detected →
    ┌──────────────────────────────────────────────────┐
    │ Welcome to miso!                                 │
    │                                                  │
    │ miso could not detect an existing javascript     │
    │ project.                                         │
    │                                                  │
    │ 1) Create new project                            │
    │ 2) Run in simple mode                            │
    │                                                  │
    └──────────────────────────────────────────────────┘

    Option 1: "Create new project"
    → Ask which PM (bun, npm, pnpm, yarn)
    → Run `<pm> init`
    → Scaffold miso.json (packageManager omitted, defaults to true)

    Option 2: "Run in simple mode"
    → Scaffold miso.json with `"packageManager": false`
    → Create `./scripts` directory
    → Done
```

### Features by Mode

#### Available in simple mode

- Folder script discovery and execution (`./scripts`)
- TUI (tabbed/merged modes)
- Concurrent tasks and dependency graphs (`dependsOn`, `concurrent`)
- Env validation (`miso env`)
- Shell/shebang-based script execution
- `miso init`, `miso version`, `miso upgrade`, `miso scripts`, `miso completion`
- All `miso.json` config: `scripts`, `shell`, `tui`, `env`, `repo.tasks`

#### Not available in simple mode

- `install`, `add`, `remove`, and all other PM passthrough commands
- `package.json` script discovery (skipped even if `package.json` exists)
- Workspace discovery from `package.json` `workspaces` field
- `flags` config (no PM to pass flags to)
- `misox` (the PM-forwarding binary)
- `ActionPassthrough` (forwarding unknown commands to PM)

### Edge Cases

- **`repo.tasks` without workspaces:** Tasks and dependency graphs work in simple mode but operate on folder scripts in the single project root only. No workspace fan-out.
- **Switching modes:** A user in simple mode can run `miso init` again to set up a PM and switch to normal mode (remove `"packageManager": false` from config).
- **`EnsureManager` never called:** When `packageManager: false`, the code path that calls `EnsureManager` is bypassed entirely. No lockfile detection, no `package.json` `packageManager` field lookup.
- **`package.json` exists in simple mode:** Ignored for script resolution. The `package.json` `scripts` field is not searched. This prevents unexpected behavior in projects that have a `package.json` for unrelated reasons.
- **Bare `miso` with no arguments:** In simple mode, behaves the same as normal mode (shows usage/error). Consider tailoring the help text to omit PM commands.
- **Multi-script runs not supported:** Since `run` is not a keyword in simple mode, there is no multi-script invocation syntax. Use TUI with `repo.tasks` for concurrent execution.
- **`--env` flag works in simple mode:** Running `miso build --env` triggers env validation before script execution, then strips `--env` from the args passed to the script. Same behavior as normal mode.

## Approach

**Early gate in command routing (Approach A).** When `packageManager: false`, bypass `ParseCLI` entirely — do not enter the command router's switch statement. Instead, `main.go` checks the config flag and goes straight to script resolution (except meta-commands).

**Implementation specifics:**

1. **Guard in `main.go`:** After `LoadConfig`, check `cfg.PackageManager`. If `false`:
   - Check if the first arg is a meta-command (`env`, `scripts`). Note: `init`, `version`, `upgrade`, `completion` are already handled before config loading.
   - If meta-command, handle it directly.
   - Otherwise, resolve the first arg as a folder script via `scripting.ResolveScript` (with `package.json` fallback disabled).
2. **Skip `EnsureManager`:** The `EnsureManager` call (~line 154 in `main.go`) is inside the normal routing path, which is bypassed entirely. It is never reached.
3. **Skip TUI manager paths:** Any TUI code that calls `manager.GetManager` is also bypassed. Add a nil guard before `mgr.BuildRun()` in `launch.go` as a safety net (no signature change needed — Go interfaces can be nil).
4. **`ParseCLI` is NOT modified.** It is simply not called in simple mode.
5. **Multi-script runs are not supported in simple mode.** Since `run` is not a keyword, `miso run script1 script2` resolves `run` as a script name. There is no multi-script invocation syntax in simple mode. Users who need to run multiple scripts concurrently should use TUI with `repo.tasks`.
6. **`ActionDev` and `ActionRunMultiple` are never produced** in simple mode because `ParseCLI` is never called.
7. **Config round-trip:** `PackageManager *bool` with `json:"packageManager,omitempty"` — Go's `omitempty` omits nil pointers but includes `*bool` pointing to `false`, so `Save` correctly preserves `"packageManager": false`.

Alternatives considered:
- **Null manager pattern:** Over-engineered. Builds a fake PM implementation to avoid using one.
- **Two-mode dispatcher:** Bigger refactor, risk of duplicating logic, more maintenance.
