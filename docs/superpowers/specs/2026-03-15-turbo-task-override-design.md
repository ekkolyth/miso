# Per-Task Orchestration Override in Turbo Mode

## Problem

With `repo: "turbo"`, all commands delegate to turbo. Users can't selectively use miso's direct orchestration for specific tasks (e.g., `dev`) while letting everything else fall through to turbo.

## Design

### Config: allow `tasks` in turbo mode

The `tasks` object in the repo config currently only works with `mode: "mono"`. Extend it to work with `mode: "turbo"` — tasks listed here get miso's direct orchestration instead of turbo delegation.

```json
{
  "repo": {
    "mode": "turbo",
    "tasks": {
      "dev": {},
      "dev:db": { "dependsOn": ["^dev"] }
    }
  },
  "tui": "tabbed"
}
```

- `miso dev` → miso discovers workspaces, runs dev directly, displays in TUI
- `miso build` → delegates to `turbo run build` as normal
- `miso lint` → delegates to turbo as normal

### Implementation

**`config.go` — relax validation (line 283)**

Change:
```go
if obj.Tasks != nil && obj.Mode != "mono" {
```
To:
```go
if obj.Tasks != nil && obj.Mode != "mono" && obj.Mode != "turbo" {
```

**`main.go` — routing logic**

In the TUI interception block, replace the simple `cfg.IsDelegated()` check with task-aware routing:

```go
if cfg.IsDelegated() {
    // Check if this specific task is overridden by miso orchestration
    _, taskOverridden := cfg.Tasks[scriptName]
    if taskOverridden {
        // Use miso's direct orchestration (same as mono mode)
        mgr, ok := manager.GetManager(managerName)
        if !ok {
            cli.Fail(logger, fmt.Errorf("unknown manager: %s", managerName), false)
        }
        ran, err := tui.Launch(cfg, scriptName, projectRoot, mgr)
        // ...
    } else {
        // Delegate to turbo
        _, turboFlags := turbo.SplitFlags(parsed.ScriptArgs, cfg.TuiEnabled())
        ran, err := tui.DelegateLaunch(cfg, scriptName, projectRoot, turboFlags)
        // ...
    }
}
```

**What already works without changes:**

- `Launch()` handles `dependsOn` with `^` prefix via `HasDependsOn()` and topological sort
- `Launch()` discovers workspaces, creates processes, manages TUI
- Flag passthrough works naturally through `ScriptArgs` to the package manager

**`miso.schema.json` — update description**

Remove "only valid with mode 'mono'" from the tasks description. Update to: "Per-task configuration. In mono mode, defines dependency ordering. In turbo mode, tasks listed here use miso's direct orchestration instead of turbo delegation."

**Documentation updates:**

- `apps/docs/content/tui/configuration.mdx` — update the "tasks is only valid when mode is mono" note
- `apps/docs/content/turbo/index.mdx` — add section on per-task orchestration override

## Files to modify

- `apps/miso/internal/config/config.go` — relax tasks validation for turbo mode
- `apps/miso/internal/config/config_test.go` — add test for turbo + tasks
- `apps/miso/cmd/main.go` — task-aware routing in TUI interception
- `apps/miso/miso.schema.json` — update tasks description
- `apps/docs/content/tui/configuration.mdx` — update docs
- `apps/docs/content/turbo/index.mdx` — add override section

## Non-goals

- Translating turbo-specific flags (like `--filter`) into miso workspace scoping
- Per-task override for nx mode (can follow the same pattern later)
