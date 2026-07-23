# TUI

`repo` and `tui` are independent. `repo` controls **orchestration** — whether
miso fans out to workspace members, runs `concurrent` companions, and orders
`dependsOn` (see `repo` Field below). `tui` controls only the **renderer** —
whether that orchestration draws chrome (tabs / merged stream) or prints
plain `[label] line` output. Turning `tui` off does not turn orchestration
off.

## `tui` Field

Controls the renderer. Set at the top level of `miso.json`.

| Value | Behavior |
|---|---|
| `"tabbed"` | Default. Sidebar with per-process log panes. Click or use arrow keys to navigate, `r` to restart, `C` to copy full log buffer. |
| `"merged"` | Interleaved output with color-coded process labels and a filter bar. |
| `"off"` | No chrome. Orchestration is unaffected — fan-out, `concurrent`, `dependsOn` still run — output streams as plain `[label] line` per process. |
| `{ "mode": "tabbed", "cleanExit": true }` | Object form. `cleanExit: true` suppresses the log dump to stdout when the TUI exits. By default, all buffered process output is printed to stdout after the TUI closes. |

Chrome (`tabbed`/`merged`) needs an interactive terminal. With no TTY — CI,
piped output, an agent — miso renders plain regardless of `tui` (see No TTY
below).

**Auto-exit behavior:** After all processes finish, the TUI waits 2 seconds then exits. By default, all buffered output is also dumped to stdout after the TUI closes. Set `cleanExit: true` to suppress that dump. Plain rendering has no such wait — it exits as soon as every process does.

---

## Interactive Mode

Press `i` to forward keyboard input to the focused task — every keystroke goes to that process's stdin (reload Metro with `r`, trigger bundler shortcuts, answer prompts). Press `Ctrl+Z` to return to miso's controls. Each task runs in its own pseudo-terminal, so tools that gate color or prompts on a TTY behave as if run directly, and long-lived dev servers stay alive instead of shutting down on a closed stdin. Interactive mode is unavailable in delegated (`turbo`/`nx`) mode — the delegate owns the child processes.

## Selection and Copy

Click-drag to select log text; `c` copies the selection, `C` copies the full buffer. Selection is anchored to buffer lines, so it survives terminal resizes. Hold a modifier (⌘/Alt/Ctrl) while clicking to let the terminal handle the click natively (native select, open links).

## No TTY

When no terminal is attached — CI, piped output, `docker` without `-t`, an
agent — miso automatically renders plain instead of chrome, whatever `tui`
is set to. Orchestration is unaffected: fan-out, `concurrent`, and
`dependsOn` all still run; only the tabs/merged-stream chrome is skipped in
favor of `[label] line` output per process. Nothing hangs or crashes.

## ANSI Colors

In `miso` mode each child runs in its own real pseudo-terminal, so its ANSI color output renders correctly whether miso draws chrome or streams plain — including slog-style loggers that emit color only when they detect a TTY. Cursor and erase control sequences are stripped; color (SGR) sequences are preserved.

In delegated (`turbo`/`nx`) mode there is no per-child pty — the delegate pipes each task's stdout — so miso sets `FORCE_COLOR=1` on the delegate (respecting `NO_COLOR`), matching what `turbo run`/`nx` emit attached to a terminal. Tools that honor `FORCE_COLOR` (most Node tooling) color; tools that only key off an attached TTY or `CLICOLOR_FORCE` (e.g. some Go `termenv`/slog loggers) may still not, since the delegate forwards `FORCE_COLOR` but not `CLICOLOR_FORCE`.

---

## `repo` Field

Controls **orchestration** — who runs the processes. Workspace membership is detected automatically from the package manager (`pnpm-workspace.yaml` or `package.json` `workspaces`), independent of this field.

| Value | Behavior |
|---|---|
| `"miso"` | Default. Miso orchestrates natively — resolves the script at the root first (`scripts/` folder + `package.json`); found there → one root process. Not found at root → fans out one process per workspace member that defines it. |
| `"turbo"` | Delegates to Turborepo. Parses Turborepo output into TUI tabs. |
| `"nx"` | Delegates to Nx (`nx run-many --target=<script>`). Parses output into TUI tabs. |
| Object form | Use when you need `tasks` config: `{ "mode": "turbo", "tasks": { ... } }` (`mode` defaults to `"miso"`). |

`"single"` and `"mono"` are removed — both are now `"miso"`; using either is a load-time config error.

---

## Script Resolution

`miso <script>` resolves the **root scope first** — its `scripts/` folder and
its `package.json`, same both-sources rule as a single-repo project (defined
in both → error, not a silent pick; see `miso-scripting`). Resolves at root
→ that's **one** process, and miso does not also fan out to members. Only
when the root has no matching script does miso resolve across workspace
members — one process per member that defines it, each member subject to the
same both-sources error within its own scope. This applies to every script
(`dev`, `build`, `test`, …), not just `dev`. Simple mode (`packageManager:
false`) never fans out — there's no workspace concept without a package
manager.

A root script always wins over fan-out. To fan `miso dev` out across
members, remove the root `dev` (folder or `package.json`) so resolution
falls through to them.

Positional args (`miso dev --inspect`) reach the spawned process only when
the run resolves to that single root entry — a member fan-out drops them,
since which member should receive them is ambiguous.

Whatever the resolved set — one root process, or N member processes — is
what gets rendered: chrome on a TTY with `tui != off`, plain `[label] line`
otherwise (see `tui` Field above).

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

Launch additional tasks alongside this one:

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

Running `miso dev` will also launch `studio` alongside it. `concurrent` tasks are always run by miso directly — they are **not** passed to turbo/nx even when `mode` is `"turbo"`.

**Scoping — bare name vs `@member/script`:**

- A bare name is **local scope**. Declared in the root's `tasks.<script>.concurrent`, it resolves against the root's `scripts/` folder + `package.json`. Declared in a **member's own** `miso.json` `tasks.<script>.concurrent`, it resolves within that member instead — member-declared companions are first-class, not root-only.
- `@member/script` is **cross-scope**. It resolves `script` inside the named member regardless of where the `concurrent` entry lives, matched by the same tiered resolver as an explicit CLI `@scope` — exact package name, then scoped short-name, then relative path, then directory basename (see `miso-scripting`).

In `"miso"` mode, root-scope `concurrent` runs alongside the root's own script; member-scope `concurrent` runs alongside that member's fan-out entry.
In `"turbo"`/`"nx"` mode, `concurrent` still runs through miso's native process management, alongside whichever task miso is directly orchestrating for that command.

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
- **Assuming no TTY means no orchestration** — an agent or CI run still fans out, runs `concurrent`, and orders `dependsOn`; it just renders plain `[label] line` instead of chrome
- **Defining a root `dev` when you wanted fan-out** — a root `scripts/dev.sh` or `package.json` `"dev"` always wins over workspace members; remove it if you want `miso dev` to fan out across them
- **Expecting chrome to need 2+ processes** — `tabbed`/`merged` render for a single process too; there's no minimum. Use `tui: "off"` for plain output on a lone script
- **`tui` nested inside `repo`** — `tui` is a top-level field, not under `repo`
- **`"repo": "turbo"` when you want miso to run dev** — string shorthand `"turbo"` fully delegates all tasks to turbo; to make miso handle specific tasks, use the object form with `tasks`: `{ "mode": "turbo", "tasks": { "dev": {} } }`
- **`concurrent` tasks going to turbo** — `concurrent` tasks are always run by miso directly, even in `"turbo"` mode; they are not passed to `turbo run`
