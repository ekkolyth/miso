# Miso Agent Skills Content Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create the `packages/skills/` directory with four SKILL.md files, a package.json, and a README.md, and add `"license": "MIT"` to `apps/docs/package.json`.

**Architecture:** Pure content work — no build step, no tests, no compilation. Each task creates or edits one file. Changes are committed per task for clean history.

**Tech Stack:** Markdown, JSON

---

### Task 1: Create `packages/skills/package.json`

**Files:**
- Create: `packages/skills/package.json`

- [ ] **Step 1: Create the file**

```json
{
  "name": "@ekkolyth/miso-skills",
  "version": "0.1.0",
  "description": "AI agent skills for working with miso",
  "license": "MIT",
  "repository": {
    "type": "git",
    "url": "https://github.com/ekkolyth/miso",
    "directory": "packages/skills"
  }
}
```

Save to `packages/skills/package.json`.

- [ ] **Step 2: Verify file contents look correct**

Run: `cat packages/skills/package.json`
Expected: JSON printed, `"name": "@ekkolyth/miso-skills"`, `"license": "MIT"` present.

- [ ] **Step 3: Commit**

```bash
git add packages/skills/package.json
git commit -m "feat: add packages/skills package.json"
```

---

### Task 2: Add `"license": "MIT"` to `apps/docs/package.json`

**Files:**
- Modify: `apps/docs/package.json`

- [ ] **Step 1: Add the license field**

In `apps/docs/package.json`, add `"license": "MIT"` immediately after the `"private": true` line. The result should look like:

```json
{
    "name": "miso-docs",
    "homepage": "https://misojs.dev",
    "private": true,
    "license": "MIT",
    "scripts": {
```

- [ ] **Step 2: Verify**

Run: `cat apps/docs/package.json | head -6`
Expected: `"license": "MIT"` visible on line 4 or 5.

- [ ] **Step 3: Commit**

```bash
git add apps/docs/package.json
git commit -m "chore: add license field to docs package.json"
```

---

### Task 3: Create `packages/skills/miso-config/SKILL.md`

**Files:**
- Create: `packages/skills/miso-config/SKILL.md`

- [ ] **Step 1: Create the file with the content below**

```markdown
# miso-config

**Use this skill** any time you need to create or modify `miso.json`.

---

## `miso.json` Field Reference

### `$schema`
**Type:** `string`
**Default:** not set

IDE autocomplete URL. Always use:
```json
"$schema": "https://misojs.dev/miso.schema.json"
```

---

### `packageManager`
**Type:** `boolean`
**Default:** `true`

`true` — miso wraps your package manager (npm, bun, pnpm, yarn). Detects lockfile automatically.
`false` — simple mode. No package manager. `miso run <script>` resolves only from the `scripts/` folder. See `miso-scripting` for script conventions and `miso-env` for env injection.

---

### `scripts`
**Type:** `string`
**Default:** `"./scripts"`

Path to the folder containing your runnable scripts. Can be relative or absolute.

```json
"scripts": "./tasks"
```

---

### `shell`
**Type:** `string`
**Default:** `"sh"`

Fallback interpreter used when a script has no shebang and no recognized extension.

```json
"shell": "bash"
```

---

### `repo`
**Type:** `"single" | "mono" | "turbo" | "nx" | object`
**Default:** `"single"`

Controls monorepo orchestration:
- `"single"` — one project, no workspace awareness
- `"mono"` — miso-native orchestration; requires `workspaces` array in root `package.json`
- `"turbo"` — delegates to Turborepo; parses output into TUI tabs
- `"nx"` — delegates to Nx (`nx run-many --target=<script>`); parses output into TUI tabs
- Object form: `{ "mode": "turbo", "tasks": { ... } }` — use when you need `tasks` config alongside delegation

---

### `tui`
**Type:** `"off" | "tabbed" | "merged" | object`
**Default:** `"off"`

Multi-process TUI display mode:
- `"off"` — standard output, no TUI
- `"tabbed"` — sidebar with per-process log panes, arrow key navigation, `r` to restart
- `"merged"` — interleaved output with color-coded process labels and filter bar
- Object form: `{ "mode": "tabbed", "cleanExit": true }`

See `miso-tui` for full TUI configuration including concurrent tasks and task ordering.

---

### `flags`
**Type:** `object` — keys are command names, values are string arrays
**Default:** `{}`

Persistent flags injected into specific commands. Useful for CI or project-wide defaults.

```json
"flags": {
  "install": ["--frozen-lockfile"],
  "dev": ["--turbo"]
}
```

---

### `env`
**Type:** `object | array`
**Default:** not set (auto-discovery mode)

Env file paths and validation rules. When not set, miso auto-discovers `.env.local` → `.env.production` → `.env.development` → `.env` (first found per variable).

See `miso-env` for full env configuration including variable types and validation.

---

## Annotated Examples

### Minimal config (simple mode project)

```json
{
  "$schema": "https://misojs.dev/miso.schema.json",
  "packageManager": false
}
```

### Full config (monorepo with TUI and env validation)

```json
{
  "$schema": "https://misojs.dev/miso.schema.json",
  "packageManager": true,
  "scripts": "./scripts",
  "shell": "bash",
  "repo": {
    "mode": "turbo",
    "tasks": {
      "dev": {
        "concurrent": ["studio"]
      }
    }
  },
  "tui": "tabbed",
  "flags": {
    "install": ["--frozen-lockfile"]
  },
  "env": [
    {
      "path": ".env.local",
      "required": "all",
      "variables": {
        "DATABASE_URL": "url",
        "PORT": "port",
        "NODE_ENV": { "type": "enum", "values": ["development", "production", "test"] }
      }
    }
  ]
}
```

---

## Common Mistakes

- **`"package-manager"` (hyphenated)** — the field is `"packageManager"` (camelCase)
- **`repo: "mono"` without `workspaces`** — miso reads `workspaces` from root `package.json` to discover workspace directories; if it's missing, mono mode won't find any workspaces
- **`tui` inside `repo`** — `tui` is a top-level field, not nested under `repo`
- **String flags** — `"flags": { "install": "--frozen-lockfile" }` is wrong; must be an array: `["--frozen-lockfile"]`
```

Save to `packages/skills/miso-config/SKILL.md`.

- [ ] **Step 2: Commit**

```bash
git add packages/skills/miso-config/SKILL.md
git commit -m "feat: add miso-config agent skill"
```

---

### Task 4: Create `packages/skills/miso-scripting/SKILL.md`

**Files:**
- Create: `packages/skills/miso-scripting/SKILL.md`

- [ ] **Step 1: Create the file with the content below**

```markdown
# miso-scripting

**Use this skill** any time you need to create, organize, name, or debug scripts in a miso `scripts/` folder.

---

## Scripts Folder

By default, miso looks for scripts in `./scripts` relative to `miso.json`. Override with the `scripts` field:

```json
{ "scripts": "./tasks" }
```

Run `miso scripts` to list all discovered scripts in the current project.

---

## Extension-Based Interpreter Dispatch

Miso selects the interpreter based on file extension:

| Extension | Interpreter |
|---|---|
| `.sh` | `sh` |
| `.bash` | `bash` |
| `.zsh` | `zsh` |
| `.js`, `.mjs` | `node` |
| `.ts` | `ts-node` |
| `.py` | `python3` |
| `.rb` | `ruby` |
| `.pl` | `perl` |
| `.lua` | `lua` |
| `.php` | `php` |

**Shebang takes precedence.** If a script starts with `#!/usr/bin/env python3`, miso uses that interpreter regardless of extension.

The `shell` field in `miso.json` sets the fallback interpreter for scripts with no recognized extension and no shebang.

---

## Invocation

```bash
miso <scriptname>       # runs scripts/<scriptname>.<ext>
miso build/docs         # runs scripts/build/docs.<ext>  (subdirectory)
miso <scriptname> --arg # passes --arg to the script
```

**Do not use `miso run`** — unlike npm, miso resolves scripts via `miso <name>` directly.

---

## Resolution Order

When you run `miso <command>`, miso checks in this order:
1. `scripts/` folder (script file match)
2. `package.json` `scripts` block
3. Passthrough to the package manager

A script file in the `scripts/` folder overrides a same-named `package.json` script. A `scripts/install.sh` file, for example, replaces `miso install`.

---

## Subdirectory Organization

Scripts can be organized in subdirectories:

```
scripts/
  build/
    docs.sh
    api.sh
  test/
    unit.sh
    e2e.sh
```

Invoke with path syntax: `miso build/docs`, `miso test/e2e`.

---

## Workspace-Scoped Scripts (Monorepos)

In a monorepo (`repo: "mono"`), run a script in a specific workspace:

```bash
miso workspace:script    # e.g. miso api:build
```

Miso resolves the script from that workspace's own `scripts/` folder first, then falls back to its `package.json` scripts.

---

## Common Mistakes

- **Missing file extension** — `scripts/build` won't be discovered; must be `scripts/build.sh` (or another supported extension)
- **Using `miso run scriptname`** — in miso, the correct form is `miso scriptname` (no `run` subcommand needed)
- **Shebang scripts not executable** — scripts invoked via shebang need `chmod +x scripts/myscript.sh`
```

Save to `packages/skills/miso-scripting/SKILL.md`.

- [ ] **Step 2: Commit**

```bash
git add packages/skills/miso-scripting/SKILL.md
git commit -m "feat: add miso-scripting agent skill"
```

---

### Task 5: Create `packages/skills/miso-tui/SKILL.md`

**Files:**
- Create: `packages/skills/miso-tui/SKILL.md`

- [ ] **Step 1: Create the file with the content below**

```markdown
# miso-tui

**Use this skill** when configuring multi-process TUI display, task ordering, or concurrent tasks.

---

## `tui` Field

Controls the terminal UI mode. Set at the top level of `miso.json`.

| Value | Behavior |
|---|---|
| `"off"` | Default. No TUI. Normal stdout. |
| `"tabbed"` | Sidebar with per-process log panes. Arrow keys to navigate, `r` to restart a process. |
| `"merged"` | Interleaved output with color-coded process labels and a filter bar. |
| `{ "mode": "tabbed", "cleanExit": true }` | Object form. `cleanExit: true` exits immediately when all processes finish (no 2-second wait). |

**Auto-exit behavior:** After all processes finish, the TUI waits 2 seconds then exits. Failures are printed to stderr. Set `cleanExit: true` to skip the wait.

---

## `repo` Field

Controls how miso discovers and orchestrates workspaces or pipeline tasks.

| Value | Behavior |
|---|---|
| `"single"` | Default. One project, no workspace awareness. |
| `"mono"` | Miso-native monorepo orchestration. Discovers workspaces from `workspaces` in root `package.json`. |
| `"turbo"` | Delegates to Turborepo. Parses Turborepo output into TUI tabs. |
| `"nx"` | Delegates to Nx (`nx run-many --target=<script>`). Parses output into TUI tabs. |
| Object form | Use when you need `tasks` config: `{ "mode": "turbo", "tasks": { ... } }` |

---

## `repo.tasks` — Concurrent and Dependent Tasks

`tasks` is an object where each key is a script name. Configure ordering and concurrency:

### `dependsOn`

Run upstream tasks before this one (topological order):

```json
"repo": {
  "mode": "mono",
  "tasks": {
    "build": {
      "dependsOn": ["^build"]
    }
  }
}
```

`"^build"` means: run `build` in all upstream dependencies first.
Without `^`, it's a same-workspace dependency (rarely needed).

### `concurrent`

Launch additional tasks alongside this one in the TUI:

```json
"repo": {
  "mode": "turbo",
  "tasks": {
    "dev": {
      "concurrent": ["studio"]
    }
  }
}
```

Running `miso dev` will also launch `studio` as a TUI tab. Tasks listed in `concurrent` are handled directly by miso's TUI — they are **not** delegated to turbo/nx.

---

## Full Example

```json
{
  "$schema": "https://misojs.dev/miso.schema.json",
  "tui": "tabbed",
  "repo": {
    "mode": "turbo",
    "tasks": {
      "dev": {
        "concurrent": ["studio", "worker"]
      },
      "build": {
        "dependsOn": ["^build"]
      }
    }
  }
}
```

---

## Common Mistakes

- **`repo: "turbo"` without Turbo installed** — Turbo must be in PATH; miso shells out to `turbo` directly
- **`dependsOn` without `^`** — `"dependsOn": ["build"]` means same-workspace dependency; use `"dependsOn": ["^build"]` for cross-workspace topological ordering
- **Expecting TUI for a single process** — the TUI only launches when multiple processes are involved (either monorepo workspaces or `concurrent` tasks configured)
- **`tui` nested inside `repo`** — `tui` is a top-level field, not under `repo`
```

Save to `packages/skills/miso-tui/SKILL.md`.

- [ ] **Step 2: Commit**

```bash
git add packages/skills/miso-tui/SKILL.md
git commit -m "feat: add miso-tui agent skill"
```

---

### Task 6: Create `packages/skills/miso-env/SKILL.md`

**Files:**
- Create: `packages/skills/miso-env/SKILL.md`

- [ ] **Step 1: Create the file with the content below**

```markdown
# miso-env

**Use this skill** when configuring env validation, debugging env injection, or understanding which `.env` files miso loads.

---

## Auto-Discovery (No `env` Config)

When no `env` block is set in `miso.json`, miso auto-discovers `.env` files in this order:

```
.env.local → .env.production → .env.development → .env
```

First found per variable wins. Shell-exported variables always take precedence over `.env` file values.

---

## `env` Config Block

### Shorthand (single file, no validation)

```json
"env": {
  ".env.local": "required"
}
```

### Single entry object

```json
"env": {
  "path": ".env.local",
  "required": "all",
  "variables": {
    "DATABASE_URL": "url",
    "PORT": "port"
  }
}
```

### Array of entries (multiple files)

```json
"env": [
  {
    "path": ".env.local",
    "label": "Local",
    "required": "all",
    "variables": {
      "DATABASE_URL": "url",
      "PORT": "port"
    }
  },
  {
    "path": ".env.secrets",
    "required": ["API_KEY"],
    "variables": {
      "API_KEY": "string"
    }
  }
]
```

---

## Entry Fields

| Field | Type | Description |
|---|---|---|
| `path` | `string` | Path to the `.env` file, relative to `miso.json` |
| `label` | `string` | Optional. Display name used in validation output |
| `required` | `"all" \| "none" \| string[]` | Which variables must be present. `"all"` = all defined variables. Array = specific keys only. |
| `variables` | `object` | Variable name → type validator |

---

## Variable Type Validators

### Shorthand

```json
"PORT": "port"
```

Equivalent to `"PORT": { "type": "port" }`.

### All Types

| Type | Validates |
|---|---|
| `"string"` | Any string. Optional `min`/`max` for length. |
| `"port"` | Integer 1–65535 |
| `"int"` | Integer. Optional `min`/`max`. |
| `"int+"` | Positive integer (> 0). Optional `min`/`max`. |
| `"float"` | Floating point number |
| `"bool"` | `"true"` or `"false"` (string) |
| `"url"` | Valid URL. Optional `schemes` array (e.g. `["redis", "rediss"]`). |
| `"enum"` | Must be one of `values`. Requires `values` array. |
| `"email"` | Valid email address |
| `"json"` | Valid JSON string |
| `"uuid"` | Valid UUID v4 |
| `"pattern"` | Matches regex. Requires `pattern` string. |

### Examples

```json
"variables": {
  "PORT": "port",
  "NODE_ENV": { "type": "enum", "values": ["development", "production", "test"] },
  "DATABASE_URL": { "type": "url", "schemes": ["postgres", "postgresql"] },
  "DESCRIPTION": { "type": "string", "min": 1, "max": 255 },
  "RETRY_COUNT": { "type": "int+", "max": 10 },
  "FEATURE_FLAG": "bool",
  "API_SECRET": { "type": "string", "optional": true }
}
```

Add `"optional": true` to any variable config to allow it to be absent without failing validation.

---

## `--env` Flag

Triggers env validation **before** script execution. The flag is stripped before args reach the script:

```bash
miso dev --env        # validates env, then runs dev
miso build --env      # validates env, then runs build
```

Run `miso env` to validate without executing any script.

**Note:** `--env` triggers validation only. Env file injection happens automatically for all scripts regardless of whether `--env` is passed.

---

## Injection Behavior

- Shell environment variables take precedence over `.env` file values
- `.env` files only fill in variables not already set in the shell environment
- In monorepos, each workspace receives env from its own directory only — no cross-workspace variable bleed

---

## Common Mistakes

- **`"required": true`** — invalid; use `"required": "all"` or `"required": ["KEY1", "KEY2"]`
- **`"type": "enum"` without `values`** — will fail validation; `values` array is required for enum type
- **Expecting `--env` to inject variables** — `--env` only triggers validation; injection is automatic for all scripts
- **Assuming `.env` wins over shell** — shell-exported variables always win; `.env` only fills gaps
```

Save to `packages/skills/miso-env/SKILL.md`.

- [ ] **Step 2: Commit**

```bash
git add packages/skills/miso-env/SKILL.md
git commit -m "feat: add miso-env agent skill"
```

---

### Task 7: Create `packages/skills/README.md`

**Files:**
- Create: `packages/skills/README.md`

- [ ] **Step 1: Create the file with the content below**

```markdown
# miso agent skills

AI agent skills for working with [miso](https://misojs.dev) — the smart package manager wrapper and script runner.

Install all skills at once:

```bash
npx skills add ekkolyth/miso --all
```

Install for a specific agent (e.g. OpenCode):

```bash
npx skills add ekkolyth/miso --all -a opencode
```

Install a specific skill:

```bash
npx skills add ekkolyth/miso --skill miso-env
```

---

## Available Skills

| Skill | When to use | What it covers |
|---|---|---|
| `miso-config` | Creating or modifying `miso.json` | All top-level fields, types, defaults, annotated examples, common mistakes |
| `miso-scripting` | Creating, organizing, or debugging scripts | Scripts folder, extension dispatch, shebang, subdirectories, resolution order |
| `miso-tui` | Configuring multi-process TUI, concurrent tasks, task ordering | `tui` field, `repo` modes, `tasks.concurrent`, `tasks.dependsOn` |
| `miso-env` | Configuring env validation or debugging env injection | `env` config, variable types, `--env` flag, injection behavior |

---

Full documentation: [misojs.dev](https://misojs.dev)
```

Save to `packages/skills/README.md`.

- [ ] **Step 2: Commit**

```bash
git add packages/skills/README.md
git commit -m "feat: add packages/skills README"
```
