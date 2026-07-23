# miso.json configuration

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

> **NOT an object.** Unlike `package.json`, the `scripts` field in `miso.json` is a **path string** pointing to a folder — not a map of command names to shell strings. Scripts are files inside that folder, not inline strings.

Path to the folder containing your runnable scripts. Can be relative or absolute. Can be omitted if your scripts live in `./scripts` (the default).

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
**Type:** `"miso" | "turbo" | "nx" | object`
**Default:** `"miso"`

Controls **orchestration** — who runs the processes. Independent of `tui`,
which only controls how the result renders (see `tui` below); turning `tui`
off doesn't turn orchestration off. Workspace *membership* is detected
separately and automatically (see Workspace discovery below), so this field
never gates whether workspaces are found.

- `"miso"` (default) — miso orchestrates natively. Resolves the script at the root first (its `scripts/` folder + `package.json`); found there → one root process, no fan-out. Not found at root → fans out one process per workspace member that defines it. Full tooling, including per-process restart.
- `"turbo"` — delegates to Turborepo; miso wraps the TUI + tooling. Turbo owns process lifecycle, so no per-process restart.
- `"nx"` — delegates to Nx (`nx run-many --target=<script>`); same wrap as turbo.
- Object form: `{ "mode": "turbo", "tasks": { ... } }` — use when you need `tasks` config. `mode` is optional (defaults to `"miso"`). Write `{ "tasks": { ... } }` for task config in a non-delegated project.

Recognized values are exactly `miso`, `turbo`, `nx` (empty → `miso`). The legacy `"single"` and `"mono"` values are **removed** — always-on discovery made them behave identically to `"miso"`, so both collapse into it. Setting `repo` to `"single"`, `"mono"`, or any other unrecognized value is a **load-time config error**.

See `miso-tui` for the full root-first resolution algorithm.

---

### Workspace discovery

Workspace members are auto-detected from the package manager's own config — always on, independent of `repo`:

- **pnpm** → `pnpm-workspace.yaml` (`packages:`)
- **bun / npm / yarn** → `workspaces` in `package.json`

There is no miso-specific workspace list. A member may carry its own `<member>/miso.json`, whose `scripts`, `shell`, `flags`, and `tui` fields override the root value for that member. A member's `tasks` (its own `concurrent`) is honored too — when a script fans out to that member, its `concurrent` companions resolve within it, alongside its own copy of the script. This doesn't delegate orchestration to the member — miso still drives the run from the root. A member's own `repo` is ignored (members are leaves), and `env` is handled by scope rather than the generic merge (see `miso-env`).

---

### Orchestration Override (turbo/nx mode)

When `repo.mode` is `"turbo"` or `"nx"`, miso uses a task-by-task routing rule:

| Task is listed in `repo.tasks`? | What happens when you run `miso <task>`? |
|---|---|
| Yes | Miso takes over: launches the TUI directly with miso-native orchestration |
| No | Miso delegates: runs `turbo run <task>` (or `nx run-many`) as normal |

**This is how you make miso control a specific task instead of delegating to turbo.**

Example: You use Turborepo but want `miso dev` to launch with miso's TUI (not turbo's output), while still using turbo for `build` and `lint`:

```json
{
  "repo": {
    "mode": "turbo",
    "tasks": {
      "dev": {
        "concurrent": ["studio"]
      }
    }
  },
  "tui": "tabbed"
}
```

What happens:
- `miso dev` → miso discovers workspaces, launches `dev` + `studio` in TUI tabs directly
- `miso build` → delegates to `turbo run build`
- `miso lint` → delegates to `turbo run lint`

To override a task without any extra config, an empty object is enough:

```json
"tasks": {
  "dev": {}
}
```

See `miso-tui` for the full `concurrent` and `dependsOn` reference.

---

### `tui`
**Type:** `"off" | "tabbed" | "merged" | object`
**Default:** `"tabbed"`

Controls the **renderer** only — independent of `repo`, which controls
orchestration (see above). Whatever `tui` is set to, a `miso <script>` run
still discovers workspace members, runs `concurrent` companions, and orders
`dependsOn`; `tui` just decides whether that gets drawn as chrome or
streamed as plain text.

- `"tabbed"` — default. Sidebar with per-process log panes, arrow key navigation, `r` to restart
- `"merged"` — interleaved output with color-coded process labels and filter bar
- `"off"` — no chrome. Plain `[label] line` output per process; orchestration is unaffected
- Object form: `{ "mode": "tabbed", "cleanExit": true }`

Chrome (`tabbed`/`merged`) needs an interactive terminal — with no TTY (CI, an agent, piped output) miso renders plain regardless of this setting.

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

Each **root** `env` entry requires a `scope` — the target it applies to, or the reserved `"global"`. Scoping keeps one workspace's variables out of another's. See `miso-env` for scopes, variable types, and validation.

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
      "scope": "global",
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
- **`repo: "single"` or `"mono"`** — both are removed; use `"miso"` (the default). Any legacy value is a load error.
- **Root `env` entry without `scope`** — every root `env` entry needs a `scope` (a target name or `"global"`); a missing scope is a config error. See `miso-env`.
- **`tui` inside `repo`** — `tui` is a top-level field, not nested under `repo`
- **String flags** — `"flags": { "install": "--frozen-lockfile" }` is wrong; must be an array: `["--frozen-lockfile"]`
- **`"scripts"` as an object** — `"scripts": { "build": "tsc" }` is invalid; `scripts` is a path string like `"./scripts"`. Inline script strings go in `package.json`. Miso discovers runnable files from the folder at that path.
