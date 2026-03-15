# Turbo Integration Layer

## Problem

Miso's turbo mode (`repo: "turbo"`) has several gaps that make the experience feel incomplete for turborepo users:

1. **No flag passthrough** — `DelegateLaunch` hardcodes `turbo run <script> --log-order=stream`. Users can't pass `--filter`, `--concurrency`, `--force`, or any other turbo flags. They must be forwarded transparently.
2. **Per-workspace exit codes lost** — Every tab gets turbo's overall exit code. If workspace A succeeded and B failed, both show the same code.
3. **No turbo.json awareness** — `miso scripts` doesn't show turbo-defined tasks. Users get an incomplete picture of available commands.
4. **No turbo-specific documentation** — No docs covering setup, behavior, or edge cases.

## Philosophy

Miso should be invisible. In turbo mode, the user's experience should feel like using turbo directly, with the TUI as an optional enhancement. Miso doesn't replace turbo's orchestration — it wraps it.

## Design

### New package: `internal/turbo`

A dedicated package encapsulating all turbo-specific logic. Three files:

#### `config.go` — turbo.json parsing

- `LoadConfig(root string) (TurboConfig, error)` — reads `turbo.json` at project root
- If `turbo.json` is not found, returns an empty `TurboConfig` (not an error) — similar to how `config.Load` handles missing `miso.json`
- If the file exists but is malformed or has neither `tasks` nor `pipeline` key, returns an error
- Auto-detects turbo.json version: `tasks` key = v2, `pipeline` key = v1
- Extracts per-task config: name, dependsOn, cache, persistent, outputs, env
- `TurboConfig.TaskNames() []string` — returns all defined task names

```go
type TurboConfig struct {
    Version int                    // 2 or 1
    Tasks   map[string]TurboTask
}

type TurboTask struct {
    DependsOn  []string
    Cache      *bool    // nil = default (true), explicit true/false
    Persistent bool
    Outputs    []string
    Env        []string
}
```

#### `output.go` — turbo output parsing

Extracted from what's currently inline in `delegate.go`. Handles:

- **Line parsing**: the existing `parseTurboLine` regex logic, moved here
- **Task status parsing**: detects per-task exit and cache status lines:
  - `<label>: exited with code <N>` → per-task exit code
  - `<label>: cache hit, replaying output` → cache status
  - `<label>: cache miss, executing` → cache status

```go
type LineMeta struct {
    Label    string
    Text     string
    Skip     bool      // boilerplate line, discard
    IsExit   bool
    ExitCode int
    CacheHit *bool     // nil = not a cache line, true/false = hit/miss
}

// ParseLine is a stateless function — no accumulated state needed.
func ParseLine(line string) LineMeta
```

#### `flags.go` — flag splitting

- `SplitFlags(args []string, tuiActive bool) (misoFlags, turboFlags []string)` — separates miso-owned flags from passthrough
- Miso-owned flags: `--env` (the only one today)
- Everything else passes through to turbo
- Special handling: when TUI is active, strip any user-provided `--log-order` flag (miso must control this for output parsing). When TUI is off, pass it through.

```go
func SplitFlags(args []string, tuiActive bool) (misoFlags, turboFlags []string)
```

### Changes to `delegate.go`

`DelegateLaunch` becomes a thin orchestrator:

1. **New signature**: `DelegateLaunch(cfg config.Config, scriptName string, root string, extraArgs []string) (bool, error)`
2. Uses `turbo.ParseLine` instead of inline regex logic
3. Appends `extraArgs` (turbo flags from user) to the command: `turbo run dev --log-order=stream --filter=web`
4. Per-workspace exit code handling:
   - When `turbo.ParseLine` returns `IsExit: true`, update that specific process's state and exit code individually
   - The blanket exit code assignment when turbo exits becomes a fallback — only applies to processes that didn't get an individual exit code
   - **Race condition fix**: stdout/stderr scanners must be fully drained (via WaitGroup) before processing `cmd.Wait()`'s exit code. The current code has fire-and-forget goroutines for scanning — these need to be synchronized so per-task exit lines aren't missed.

### Changes to CLI (main.go)

The TUI interception block passes `parsed.ScriptArgs` through to `DelegateLaunch` after flag splitting:

```go
misoFlags, turboFlags := turbo.SplitFlags(parsed.ScriptArgs, cfg.TuiEnabled())
// handle misoFlags (--env, etc.)
ran, err := tui.DelegateLaunch(cfg, scriptName, projectRoot, turboFlags)
```

### Changes to `miso scripts`

When `cfg.IsDelegated()` and mode is `"turbo"`:

1. Load turbo.json via `turbo.LoadConfig(root)`
2. Merge task sources with precedence: **scripts folder > package.json > turbo.json**
3. Add turbo tasks as a third group in the existing grouped display format (scripts folder, package.json, turbo.json — each as its own section), consistent with how `List()` in `apps/miso/internal/cli/scripting/scripts.go` currently renders.
4. Also update `ListNames()` in the same file so turbo tasks appear in tab-completion.

### Documentation

New section at `apps/docs/content/turbo/`:

**`index.mdx`** — Overview and setup:
- `repo: "turbo"` config
- What miso delegates vs what it handles
- TUI on vs TUI off behavior

**`flag-passthrough.mdx`** — Flag behavior:
- Any flag miso doesn't own is forwarded to turbo transparently
- `miso dev --filter=web --concurrency=2` just works
- `--log-order` edge case: when TUI is active, miso overrides to `--log-order=stream` (required for output parsing). When TUI is off, user's value is respected.

**`task-discovery.mdx`** — turbo.json integration:
- How turbo.json tasks appear in `miso scripts`
- Precedence order: scripts folder > package.json > turbo.json
- Version support (v1 pipeline, v2 tasks)

Also update `apps/docs/content/tui/configuration.mdx` to document the `cleanExit` option:
```json
{ "tui": { "mode": "tabbed", "cleanExit": true } }
```

## Files to create

- `apps/miso/internal/turbo/config.go`
- `apps/miso/internal/turbo/config_test.go`
- `apps/miso/internal/turbo/output.go`
- `apps/miso/internal/turbo/output_test.go`
- `apps/miso/internal/turbo/flags.go`
- `apps/miso/internal/turbo/flags_test.go`
- `apps/docs/content/turbo/index.mdx`
- `apps/docs/content/turbo/flag-passthrough.mdx`
- `apps/docs/content/turbo/task-discovery.mdx`

## Files to modify

- `apps/miso/internal/tui/delegate.go` — use `turbo.ParseLine`, accept `extraArgs`, per-task exit codes, fix scanner drain race
- `apps/miso/internal/tui/delegate_test.go` — update/move tests for parsing logic that moves to `turbo` package
- `apps/miso/cmd/main.go` — pass args through to `DelegateLaunch` after flag splitting
- `apps/miso/internal/cli/scripting/scripts.go` — merge turbo.json tasks into `List()` and `ListNames()`
- `apps/docs/content/tui/configuration.mdx` — document `cleanExit` option

## Non-goals

- Replacing turbo's orchestration (caching, dependency graph execution)
- Reading turbo.json for cache configuration or env hashing
- Supporting turbo's `--graph` or `--dry-run` output formats in TUI
- Nx-specific equivalent changes (can follow later using the same package pattern)
