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

- **Default:** `true` when absent. Existing configs are unaffected.
- **Schema:** Add `"packageManager"` as a boolean to `miso.schema.json` with default `true`.

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
| `miso run build` | Looks for `scripts/run` — NOT `build` |
| `miso install` | Looks for `scripts/install.sh`, fails if not found |
| `miso dev` | Looks for `scripts/dev.sh`, TUI kicks in if configured |

**Miso meta-commands that remain built-in in simple mode:**

- `miso env` — env validation
- `miso init` — project initialization
- `miso version` — version info
- `miso upgrade` — self-update

These are miso's own commands, not PM commands.

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
    → If package.json exists with `packageManager` field, use that PM
    → Otherwise ask which PM (bun, npm, pnpm, yarn)
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
- `miso init`, `miso version`, `miso upgrade`
- All `miso.json` config: `scripts`, `shell`, `tui`, `env`, `repo.tasks`

#### Not available in simple mode

- `install`, `add`, `remove`, and all other PM passthrough commands
- `package.json` script discovery (no `package.json` expected)
- Workspace discovery from `package.json` `workspaces` field
- `flags` config (no PM to pass flags to)
- `misox` (the PM-forwarding binary)

### Edge Cases

- **`repo.tasks` without workspaces:** Tasks and dependency graphs work in simple mode but operate on folder scripts in the single project root only. No workspace fan-out.
- **Switching modes:** A user in simple mode can run `miso init` again to set up a PM and switch to normal mode (remove `"packageManager": false` from config).
- **`EnsureManager` never called:** When `packageManager: false`, the code path that calls `EnsureManager` is bypassed entirely. No lockfile detection, no `package.json` `packageManager` field lookup.

## Approach

**Early gate in command routing (Approach A).** A single check at the top of the routing logic in `main.go`. When `packageManager: false`, skip built-in command matching and go straight to script resolution (except meta-commands). This is the minimal change that matches the mental model: in simple mode, there are no built-in PM commands.

Alternatives considered:
- **Null manager pattern:** Over-engineered. Builds a fake PM implementation to avoid using one.
- **Two-mode dispatcher:** Bigger refactor, risk of duplicating logic, more maintenance.
