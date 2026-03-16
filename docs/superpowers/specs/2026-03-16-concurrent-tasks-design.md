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

Currently, `parseRepoField` in `config.go` rejects `tasks` unless `mode` is `"mono"` or `"turbo"`. Remove this restriction entirely so `tasks` is valid in all modes: `"single"`, `"mono"`, `"turbo"`, and `"nx"`.

This intentionally enables task overrides for nx mode too — the routing logic in `main.go` already handles delegated modes (`IsDelegated()` covers both turbo and nx), so nx task overrides work with no additional code.

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

### Single-repo behavior

In single-repo mode, `discoverEntries` currently returns `nil` when there's no `multi` config, causing the TUI to not launch. With `concurrent`, the single-repo path needs a new code branch:

1. Check `cfg.TaskConcurrent(scriptName)` for the current command
2. If concurrent tasks exist, resolve the main task itself against the root (scripts folder, then package.json) and add it as a TUI entry
3. Resolve each concurrent task name against the root the same way
4. If the main task doesn't resolve (no script named `dev` in the root), only concurrent entries run — this matches the old `multi` behavior where the command name was just a lookup key
5. Return all entries for the TUI

### Remove `multi`

The `multi` field is removed entirely — no deprecation period, no backwards compatibility. If a user's `miso.json` still contains a `multi` key, emit a stderr warning during config load: `"warning: 'multi' config is no longer supported — use repo.tasks with 'concurrent' instead"`.

The equivalent using `concurrent`:

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
  "repo": {
    "tasks": {
      "build": {
        "dependsOn": ["^build"],
        "concurrent": ["typecheck"]
      }
    }
  }
}
```

**Ordering behavior:** Concurrent entries are excluded from the `dependsOn` dependency graph. The dependency levels computed by `HasDependsOn` / `BuildDependencyGraph` / `TopoSort` apply only to the main task's entries. Concurrent entries always start immediately when the TUI launches — they do not participate in topological sorting and are not blocked by dependency levels.

Implementation: in `launch.go`, split the entries into two groups before computing dependency levels — main task entries (passed to the topo sort) and concurrent entries (started unconditionally). In the startup goroutine, start all concurrent entries immediately, then proceed with level-based startup for main entries.

### Discovery behavior

- **Monorepo modes** (`mono`, `turbo`, `nx` with task override): For each name in `concurrent`, call `DiscoverTuiScripts(name, workspaces, scriptsFolder)`. This finds all workspaces with a matching script using the same prefix-matching logic as the main task.
- **Single-repo mode**: For each name in `concurrent`, resolve the script against the project root using `scripting.ResolveScript`. This checks the scripts folder first, then package.json. If a script cannot be found, skip it silently (same as monorepo behavior).
- If a concurrent task name matches nothing in any mode, skip silently. The workspace may not have that script and that's fine.

### Label strategy

Labels must be unique across the merged entry list (main task + all concurrent tasks). The existing `DiscoverTuiScripts` assigns labels per-call: single match in a workspace gets the workspace name, multiple matches get `workspace:scriptName`. This works within a single call but can collide across calls — e.g., workspace "app" with only `dev` gets label `"app"`, and the same workspace with only `services` also gets label `"app"`.

**Fix:** After merging all entries (main + concurrent), do a post-merge pass. For any label that appears more than once, rewrite it to `label:scriptName` to disambiguate. This keeps labels short when there's no collision and only adds the script name suffix when needed.

In single-repo mode, concurrent entries use the script name as the label (same as `DiscoverMultiScripts` does today).

## Implementation

### `config.go`

- Add `Concurrent []string` field to `TaskConfig` struct with JSON tag `"concurrent,omitempty"`
- Remove `Multi` field from `Config` struct
- Remove `Multi` from `configLoad` struct
- Remove the mode restriction in `parseRepoField` (line 290) — delete the entire `if obj.Tasks != nil && ...` block to allow `tasks` in all modes
- Add helper method `TaskConcurrent(command string) []string` that returns the concurrent list for a given command, or nil
- In `Load()`, after parsing, check if the raw JSON contains a `"multi"` key. If so, emit a stderr warning: `"warning: 'multi' config is no longer supported — use repo.tasks with 'concurrent' instead"`

### `config_test.go`

- Add test for parsing `concurrent` field
- Add test for `tasks` in single mode (update `TestLoadRepoObjectInvalidMode` which currently asserts that `tasks` with `mode: "single"` is an error — change to expect success)
- Add test for `tasks` in nx mode
- Remove tests for `multi`
- Test that `TaskConcurrent()` returns correct values

### `launch.go`

- In `discoverEntries`:
  - **Monorepo path:** after discovering main task entries via `DiscoverTuiScripts(scriptName, ...)`, iterate `cfg.TaskConcurrent(scriptName)` and call `DiscoverTuiScripts(concName, ...)` for each. Merge results.
  - **Single-repo path:** replace the `cfg.Multi` branch with a new branch that checks `cfg.TaskConcurrent(scriptName)`. Resolve the main task and each concurrent task via `scripting.ResolveScript`, build `TuiScriptEntry` for each. Skip any that fail to resolve.
- After merging, run the label deduplication pass (see Label strategy above)
- Remove the `cfg.Multi` code path entirely
- In the startup goroutine, when `levels != nil`, start concurrent entries immediately (before level-based startup of main entries). Concurrent entries can be identified by checking if their `ScriptName` differs from the main `scriptName`.

### `discover.go`

- Remove `DiscoverMultiScripts` function

### `discover_test.go`

- Remove tests for `DiscoverMultiScripts`

### `cmd/main.go`

- In the TUI interception block (lines 179-227), the single-repo non-delegated path (lines 212-224) currently calls `tui.Launch` which falls through when `discoverEntries` returns nil. After the `launch.go` changes, this path will now find concurrent entries and launch the TUI. No routing changes needed — the existing `tui.Launch` call handles it, but verify this works end-to-end.
- For the non-monorepo, non-TUI path: when `cfg.Tasks` has a `concurrent` entry for the script but TUI is disabled, the concurrent tasks are silently ignored (they only make sense in TUI mode). No special handling needed.

### Schema files (both must be updated)

**`apps/miso/miso.schema.json`:**
- Add `concurrent` to the task config properties alongside `dependsOn`:
  ```json
  "concurrent": {
    "type": "array",
    "items": { "type": "string" },
    "description": "Additional tasks to discover and run in parallel alongside the main task."
  }
  ```
- Remove the `multi` property from the top-level properties
- Update the `tasks` description: "Per-task configuration. Available in all repo modes. Defines dependency ordering (dependsOn) and companion tasks (concurrent). In turbo/nx mode, tasks listed here use miso's direct orchestration instead of delegation."

**`apps/docs/public/miso.schema.json`:**
- Same changes as above: add `concurrent` to task config properties (and update `additionalProperties: false` to allow it), remove `multi`, update `tasks` description

### `miso.example.json`

- Add a `repo.tasks` example showing `concurrent`:
  ```json
  {
    "repo": {
      "tasks": {
        "dev": { "concurrent": ["services"] }
      }
    }
  }
  ```

### `miso.example.monorepo.json`

- Add `concurrent` to the `dev` task:
  ```json
  "dev": { "concurrent": ["services", "db:studio"] }
  ```

### Documentation

**`apps/docs/content/tui/configuration.mdx`:**
- Remove the `## multi Option (Single Repos)` section entirely
- Update the `## repo Option` section:
  - Remove "The object form is only valid when mode is mono. Turbo and Nx modes do not support tasks"
  - Replace with: "The object form works in all repo modes"
  - Add `concurrent` documentation to the task config explanation
  - Show examples for both monorepo and single-repo usage
- Update the `## Script Discovery` section:
  - Single repos: replace `multi` references with `concurrent`
  - Add: "Concurrent tasks are discovered alongside the main task and run in parallel"

**`apps/docs/content/working-with-miso/config.mdx`:**
- Remove the `### Multi` section
- Update the `### Repo` section:
  - Show `concurrent` in examples
  - Note that `tasks` works in all modes
  - Update the object-form example to include `concurrent`

### Other Go files referencing `multi`

All references to `Multi` or `multi` in Go source must be removed:
- `apps/miso/internal/tui/launch.go` — remove the `cfg.Multi` code path in `discoverEntries`, remove the monorepo+multi warning
- `apps/miso/internal/tui/discover.go` — remove `DiscoverMultiScripts`
- `apps/miso/internal/tui/discover_test.go` — remove `DiscoverMultiScripts` tests
- `apps/miso/internal/config/config.go` — remove `Multi` from structs
- `apps/miso/internal/config/config_test.go` — remove multi-related tests
- Check all files from grep for `multi` and clean up any remaining references

## Non-goals

- Ordering between concurrent tasks (they all start at the same time)
- Scoping concurrent tasks to specific workspaces (discovery runs across all workspaces)
