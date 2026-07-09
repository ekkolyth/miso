# TUI

## `tui` Field

Controls the terminal UI mode. Set at the top level of `miso.json`.

| Value | Behavior |
|---|---|
| `"off"` | Default. No TUI. Normal stdout. |
| `"tabbed"` | Sidebar with per-process log panes. Click or use arrow keys to navigate, `r` to restart, `C` to copy full log buffer. |
| `"merged"` | Interleaved output with color-coded process labels and a filter bar. |
| `{ "mode": "tabbed", "cleanExit": true }` | Object form. `cleanExit: true` suppresses the log dump to stdout when the TUI exits. By default, all buffered process output is printed to stdout after the TUI closes. |

**Auto-exit behavior:** After all processes finish, the TUI waits 2 seconds then exits. By default, all buffered output is also dumped to stdout after the TUI closes. Set `cleanExit: true` to suppress that dump.

---

## Interactive Mode

Press `i` to forward keyboard input to the focused task — every keystroke goes to that process's stdin (reload Metro with `r`, trigger bundler shortcuts, answer prompts). Press `Ctrl+Z` to return to miso's controls. Each task runs in its own pseudo-terminal, so tools that gate color or prompts on a TTY behave as if run directly, and long-lived dev servers stay alive instead of shutting down on a closed stdin. Interactive mode is unavailable in delegated (`turbo`/`nx`) mode — the delegate owns the child processes.

## Selection and Copy

Click-drag to select log text; `c` copies the selection, `C` copies the full buffer. Selection is anchored to buffer lines, so it survives terminal resizes. Hold a modifier (⌘/Alt/Ctrl) while clicking to let the terminal handle the click natively (native select, open links).

## No TTY

When no terminal is attached — CI, piped output, `docker` without `-t`, an agent — miso prints `miso: no interactive terminal — running plain` and runs the script with normal inherited stdio instead of the TUI. Applies to both `tabbed` and `merged`; nothing hangs or crashes.

## ANSI Colors

Because each child runs in a real pseudo-terminal, its ANSI color output renders in the TUI — including slog-style loggers that emit color only when they detect a TTY. Cursor and erase control sequences are stripped; color (SGR) sequences are preserved.

---

## `repo` Field

Controls **orchestration** — who runs the processes. Workspace membership is detected automatically from the package manager (`pnpm-workspace.yaml` or `package.json` `workspaces`), independent of this field.

| Value | Behavior |
|---|---|
| `"miso"` | Default. Miso orchestrates natively — auto-discovers workspace members and fans out one process each, or runs a single process if there are none. |
| `"turbo"` | Delegates to Turborepo. Parses Turborepo output into TUI tabs. |
| `"nx"` | Delegates to Nx (`nx run-many --target=<script>`). Parses output into TUI tabs. |
| Object form | Use when you need `tasks` config: `{ "mode": "turbo", "tasks": { ... } }` (`mode` defaults to `"miso"`). |

`"single"` and `"mono"` are removed — both are now `"miso"`; using either is a load-time config error.

---

## `repo.tasks` — Concurrent and Dependent Tasks

`tasks` is an object where each key is a script name. Configure ordering and concurrency:

### `dependsOn`

Run upstream tasks before this one (topological order):

```json
"repo": {
  "mode": "miso",
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

Running `miso dev` will also launch `studio` as a TUI tab alongside it. `concurrent` tasks are always run by miso directly — they are **not** passed to turbo/nx even when `mode` is `"turbo"`.

In `"miso"` mode, `concurrent` launches extra scripts from the root `scripts/` folder alongside the workspace-distributed task.
In `"turbo"` mode, `concurrent` launches extra scripts from the root `scripts/` folder alongside miso's native `dev` orchestration (not turbo's).

---

### Overriding a Turbo (or Nx) Task

When `repo.mode` is `"turbo"` or `"nx"`, miso routes each command individually:

- **Task name is in `repo.tasks`** → miso runs it directly with its own TUI orchestration
- **Task name is NOT in `repo.tasks`** → miso delegates to `turbo run <task>` (or `nx run-many`)

This lets you pick exactly which tasks miso controls and which turbo handles.

**Example: take over `dev`, keep turbo for everything else**

Before (turbo handles dev — string shorthand):
```json
{
  "repo": "turbo",
  "tui": "tabbed"
}
```
`miso dev` → delegates to `turbo run dev`; turbo parses output into TUI tabs.

After (miso handles dev — object form with `tasks`):
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
`miso dev` → miso discovers workspaces from `package.json`, launches `dev` + `studio` in parallel in TUI tabs directly. `miso build` and `miso lint` still delegate to turbo.

**Minimum override (no extra config needed):**

```json
"tasks": {
  "dev": {}
}
```

An empty task entry is enough to make miso take over `dev`.

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
- **`"repo": "turbo"` when you want miso to run dev** — string shorthand `"turbo"` fully delegates all tasks to turbo; to make miso handle specific tasks, use the object form with `tasks`: `{ "mode": "turbo", "tasks": { "dev": {} } }`
- **`concurrent` tasks going to turbo** — `concurrent` tasks are always run by miso directly, even in `"turbo"` mode; they are not passed to `turbo run`
