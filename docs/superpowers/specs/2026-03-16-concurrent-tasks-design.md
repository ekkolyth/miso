# Concurrent Tasks

## Problem

When miso overrides a task in turbo mode (e.g., `dev`), it only discovers scripts matching the main task name across workspaces. Companion tasks like `services` or `db:studio` — which turbo runs via its `with` field — are never discovered or started. Users migrating from turbo/nx expect one command to start everything.

Additionally, the existing `multi` config for single repos is a separate concept that solves a similar problem. It maps a command to a list of scripts to run concurrently, but uses a different code path and only works in single-repo mode.

## Design

### New `concurrent` field on `TaskConfig`

Add a `concurrent` property to the existing `TaskConfig` struct. It holds an array of task names to discover and run in parallel alongside the main task.

```json
{
  "repo": {
    "mode": "turbo",
    "tasks": {
      "dev": { "concurrent": ["services", "db:studio"] }
    }
  }
}
```

When `miso dev` runs:

1. Discover all workspaces with a `dev` script (existing behavior)
2. For each name in `concurrent`, discover all workspaces with that script
3. Merge all entries into a single list
4. Start everything concurrently in the TUI

### `tasks` works in all repo modes

Currently, `parseRepoField` in `config.go` rejects `tasks` unless `mode` is `"mono"` or `"turbo"`. Remove this restriction so `tasks` is valid in all modes including `"single"` and `"nx"`.

Single-repo example:

```json
{
  "repo": {
    "mode": "single",
    "tasks": {
      "dev": { "concurrent": ["services", "watch:css"] }
    }
  }
}
```

In single-repo mode, concurrent task discovery resolves each name against the root directory using the same resolution logic (scripts folder, then package.json).

### Remove `multi`

The `multi` field is removed entirely — no deprecation period, no backwards compatibility. The equivalent using `concurrent`:

Before:
```json
{
  "multi": {
    "dev": ["frontend", "backend"]
  }
}
```

After:
```json
{
  "repo": {
    "tasks": {
      "dev": { "concurrent": ["frontend", "backend"] }
    }
  }
}
```

### `concurrent` and `dependsOn` coexist

They are orthogonal. `dependsOn: ["^build"]` controls workspace-level ordering for the main task. `concurrent: ["services"]` adds companion tasks that run in parallel. Both can appear on the same task config:

```json
{
  "tasks": {
    "build": {
      "dependsOn": ["^build"],
      "concurrent": ["typecheck"]
    }
  }
}
```

### Discovery behavior

- **Monorepo modes** (`mono`, `turbo`, `nx` with task override): For each name in `concurrent`, call `DiscoverTuiScripts(name, workspaces, scriptsFolder)`. This finds all workspaces with a matching script using the same prefix-matching logic as the main task.
- **Single-repo mode**: For each name in `concurrent`, resolve the script against the project root using `scripting.ResolveScript`. This checks the scripts folder first, then package.json — same resolution as the former `multi` path.
- If a concurrent task name matches nothing, skip silently. The workspace may not have that script and that's fine.

### Label deduplication

Concurrent entries from different task names could theoretically collide on labels (e.g., two workspaces both named "docker" discovered from different concurrent tasks). The existing label generation in `DiscoverTuiScripts` uses `workspace:scriptName` when multiple scripts match within a workspace, which naturally prevents collisions across different script names.

## Implementation

### `config.go`

- Add `Concurrent []string` field to `TaskConfig` struct with JSON tag `"concurrent,omitempty"`
- Remove `Multi` field from `Config` struct
- Remove `Multi` from `configLoad` struct
- Remove the mode restriction in `parseRepoField` (line 290) — allow `tasks` in all modes
- Add helper method `TaskConcurrent(command string) []string` that returns the concurrent list for a given command, or nil

### `config_test.go`

- Add test for parsing `concurrent` field
- Add test for `tasks` in single mode
- Remove tests for `multi`
- Test that `TaskConcurrent()` returns correct values

### `launch.go`

- In `discoverEntries`, after discovering main task entries, iterate `cfg.TaskConcurrent(scriptName)` and discover entries for each concurrent task name
- Monorepo path: call `DiscoverTuiScripts` for each concurrent name
- Single-repo path: resolve each concurrent name via `scripting.ResolveScript` and build `TuiScriptEntry` (similar to what `DiscoverMultiScripts` does today)

### `discover.go`

- Remove `DiscoverMultiScripts` function (it handled the `multi` code path)

### `miso.schema.json`

- Add `concurrent` property to the task config object under `repo.tasks`:
  ```json
  "concurrent": {
    "type": "array",
    "items": { "type": "string" },
    "description": "Additional tasks to discover and run in parallel alongside the main task."
  }
  ```
- Remove the `multi` property from the top-level schema
- Update the `tasks` description to reflect availability in all modes

### `miso.example.json`

- Remove any `multi` usage
- Add a `repo.tasks` example with `concurrent` if appropriate

### `miso.example.monorepo.json`

- Add `concurrent` to the `dev` task example

### Documentation

**`apps/docs/content/tui/configuration.mdx`**:
- Remove the `## multi Option (Single Repos)` section
- Update the `## repo Option` section: remove "tasks is only valid when mode is mono" language
- Add `concurrent` documentation to the task config section
- Update the single-repo script discovery section to reference `concurrent` instead of `multi`

**`apps/docs/content/working-with-miso/config.mdx`**:
- Remove the `### Multi` section
- Update the `### Repo` section to show `concurrent` in examples and note that `tasks` works in all modes

### Other Go files referencing `multi`

- `apps/miso/internal/tui/launch.go` — remove the `cfg.Multi` code path in `discoverEntries`
- `apps/miso/internal/tui/discover.go` — remove `DiscoverMultiScripts`
- `apps/miso/internal/tui/discover_test.go` — remove `DiscoverMultiScripts` tests
- `apps/miso/cmd/main.go` — no changes needed (routing logic already handles task overrides)
- Any other files referencing `Multi` or `multi` (check grep results)

## Non-goals

- Ordering between concurrent tasks (they all start at the same time)
- Scoping concurrent tasks to specific workspaces (discovery runs across all workspaces)
- Nx-specific concurrent configuration (nx mode delegates; task override with concurrent follows the same pattern as turbo)
