# Miso Agent Skills: Design Spec

**Date:** 2026-03-19
**Status:** Draft
**Branch:** feature/tui-script-execution

## Problem

Miso has a rich configuration surface — `miso.json` fields, script folder conventions, TUI modes, env validation types, injection behavior — that AI agents frequently get wrong without explicit guidance. Users who work with AI agents in miso-powered projects have no way to give their agent accurate, up-to-date knowledge about miso without pasting documentation into every conversation.

## Solution

Publish a set of four focused agent skills to the miso GitHub repository. Each skill teaches an AI agent a distinct domain of miso knowledge. Users install them once via the `vercel-labs/skills` CLI and the agent loads the relevant skill whenever it's working with miso.

## Design

### Repository Structure

A new `packages/skills/` directory is added alongside the existing `apps/` directory:

```
packages/
  skills/
    README.md
    package.json
    miso-config/
      SKILL.md
    miso-scripting/
      SKILL.md
    miso-tui/
      SKILL.md
    miso-env/
      SKILL.md
```

No build step. No dependencies beyond the `package.json` metadata. Each skill is a standalone directory containing a single `SKILL.md` file.

### Distribution

Skills are distributed via the `vercel-labs/skills` CLI, which supports 40+ AI agents including OpenCode, Claude Code, Cursor, Windsurf, and Codex.

**Install all skills at once:**
```bash
npx skills add ekkolyth/miso --all
```

**Install a specific skill:**
```bash
npx skills add ekkolyth/miso --skill miso-env
```

The CLI recursively discovers `SKILL.md` files in the repository, so pointing at the monorepo root (`ekkolyth/miso`) works without a subpath. Users can also target specific agents:

```bash
npx skills add ekkolyth/miso --all -a opencode
```

### `package.json`

`packages/skills/package.json` provides npm publishing identity and monorepo metadata:

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

No `main`, no `dependencies`, no scripts. This package is not meant to be installed via npm — it exists so the package has a canonical identity and the `license` field is explicit.

### Licensing Consistency

As part of this work, `apps/docs/package.json` gains a `"license": "MIT"` field to match `apps/miso/package.json`. The root `LICENSE` file already covers the entire monorepo; this is a consistency fix.

| File | Action |
|---|---|
| `packages/skills/package.json` | Create with `"license": "MIT"` |
| `apps/docs/package.json` | Add `"license": "MIT"` |
| `apps/miso/package.json` | Already `"license": "MIT"` — no change |
| Root `LICENSE` | No change |

### The Four Skills

Each skill is a knowledge/reference skill, not a process/workflow skill. No checklists, no flowcharts. The format is: **when to use** → **core knowledge** (config fields, types, behaviors, annotated examples) → **common mistakes**.

---

#### `miso-config`

**Trigger:** Any time an agent needs to create or modify `miso.json`.

**Covers:**
- All top-level fields with types, defaults, and valid values:
  - `$schema` — the IDE autocomplete URL (`https://misojs.dev/miso.schema.json`)
  - `packageManager` — `boolean`, default `true`; `false` enables simple mode
  - `scripts` — `string`, path to scripts folder, default `"./scripts"`
  - `shell` — `string`, fallback interpreter (`"sh"`, `"bash"`, `"zsh"`, etc.)
  - `repo` — `"single" | "mono" | "turbo" | "nx" | object`
  - `tui` — `"off" | "tabbed" | "merged" | object`
  - `flags` — object of command → string array
  - `env` — object or array; env validation config
- Annotated minimal and full examples
- Simple mode overview (one paragraph, links to `miso-env` and `miso-tui` for depth)

**Common mistakes:**
- Using `"package-manager"` (hyphen) instead of `"packageManager"` (camelCase)
- Setting `repo: "mono"` without a `workspaces` field in `package.json`
- Putting `tui` config inside `repo` instead of at the top level
- Using string flags like `"--frozen-lockfile"` instead of an array `["--frozen-lockfile"]`

---

#### `miso-scripting`

**Trigger:** Creating, organizing, naming, or debugging scripts in the `scripts/` folder.

**Covers:**
- Default scripts folder path (`./scripts`) and how to change it via `miso.json`
- Extension-based interpreter dispatch:
  - `.sh` → `sh`, `.bash` → `bash`, `.zsh` → `zsh`
  - `.js` / `.mjs` → `node`
  - `.ts` → `ts-node`
  - `.py` → `python3`
  - `.rb` → `ruby`, `.pl` → `perl`, `.lua` → `lua`, `.php` → `php`
- Shebang detection (takes precedence over extension)
- Subdirectory organization and how to invoke nested scripts (`miso build/docs`)
- How scripts override built-in miso commands (e.g., `scripts/install.sh` replaces `miso install`)
- Script resolution order: scripts folder → `package.json` scripts → passthrough to PM
- Workspace-scoped scripts in monorepos (`workspace:script` syntax)
- `miso scripts` command for listing all discovered scripts

**Common mistakes:**
- Forgetting the file extension (miso uses it for interpreter selection)
- Expecting `miso run scriptname` to work like npm — in miso, `miso scriptname` is the correct form
- Not making script files executable (shebang-based scripts need `chmod +x`)

---

#### `miso-tui`

**Trigger:** Configuring multi-process TUI display, task ordering, or concurrent tasks.

**Covers:**
- `tui` field values:
  - `"off"` — default, no TUI
  - `"tabbed"` — sidebar with per-process logs, arrow key nav, `r` to restart
  - `"merged"` — interleaved output with color-coded labels and filter bar
  - Object form: `{ "mode": "tabbed", "cleanExit": true }`
- `repo` field values and when to use each:
  - `"single"` — default, one project
  - `"mono"` — miso-native monorepo orchestration; requires `workspaces` in `package.json`
  - `"turbo"` — delegates to Turborepo, parses output into TUI tabs
  - `"nx"` — delegates to Nx (`nx run-many --target=<script>`), parses output into TUI tabs
  - Object form with `mode` and `tasks`
- `repo.tasks` configuration:
  - `dependsOn: ["^taskName"]` — run task in upstream dependencies first (topological order)
  - `concurrent: ["taskName"]` — launch additional tasks alongside in TUI
  - Tasks listed in `concurrent` override turbo/nx delegation for those tasks
- Auto-exit behavior: TUI waits 2 seconds after all processes finish, then exits; failures printed to stderr

**Common mistakes:**
- Setting `repo: "turbo"` but not having Turbo installed and in PATH
- Using `dependsOn` without the `^` prefix (without `^`, it's a same-workspace dependency, not cross-workspace)
- Expecting the TUI to launch for a single-workspace project with no `concurrent` tasks configured

---

#### `miso-env`

**Trigger:** Configuring env validation, debugging env injection, or understanding which `.env` files are loaded.

**Covers:**
- `env` config shapes: shorthand object, single entry object, and array of entries
- Entry fields: `path`, `label`, `required` (`"all"`, `"none"`, or array of keys), `variables`
- All variable type validators with examples:
  - `"string"` — with optional `min`/`max` length
  - `"port"` — integer 1–65535
  - `"int"` / `"int+"` — integer / positive integer, optional `min`/`max`
  - `"float"` — floating point
  - `"bool"` — `"true"` or `"false"`
  - `"url"` — valid URL; optional `schemes` array (e.g., `["redis", "rediss"]`)
  - `"enum"` — must include `values` array
  - `"email"` — valid email address
  - `"json"` — valid JSON string
  - `"uuid"` — valid UUID v4
  - `"pattern"` — regex; must include `pattern` field
  - Shorthand: `"PORT": "port"` is equivalent to `"PORT": { "type": "port" }`
  - Optional variables: add `"optional": true` to any variable config
- `--env` flag: triggers validation before script execution; strips the flag before passing args to the script
- Auto-discovery when no `env` block is configured: loads `.env.local` → `.env.production` → `.env.development` → `.env` in that order (first found wins per variable)
- Injection behavior: shell environment takes precedence over `.env` file values; `.env` only fills in variables not already set
- Monorepo isolation: each workspace only receives env from its own directory; no cross-workspace variable bleed
- `miso env` command: runs validation and reports errors without executing any script

**Common mistakes:**
- Using `"required": true` instead of `"required": "all"`
- Defining `"type": "enum"` without a `values` array
- Expecting `--env` to inject variables — it validates them; injection happens automatically for all scripts regardless of `--env`
- Assuming `.env` files are loaded in all circumstances — they are, but shell-exported variables always win

---

### `README.md`

The `packages/skills/README.md` covers:
- One-sentence description of what these skills are
- Install command (`npx skills add ekkolyth/miso --all`)
- Table: skill name → trigger condition → what it covers
- Link to `https://misojs.dev` for full documentation

## Out of Scope

- A miso-contributor skill (for working on the miso codebase itself) — separate effort
- A `miso-monorepo` or `miso-simple-mode` focused skill — the relevant content fits within `miso-config`, `miso-scripting`, and `miso-tui`
- Publishing to npm registry — the `package.json` identity is present but npm publishing is not part of this work
- Automated skill update notifications — handled by `npx skills check` from the vercel-labs/skills CLI
