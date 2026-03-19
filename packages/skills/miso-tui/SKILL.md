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
