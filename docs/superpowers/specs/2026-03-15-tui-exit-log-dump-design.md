# TUI Exit Log Dump & Clean Exit Config

## Problem

When exiting a TUI session (dev, build, etc.), the alternate screen is restored and all log history is lost. Crashes, error codes, and other output disappear completely. Turbo repo dumps output on exit by default — miso should too.

Secondary bug: In turbo+merged mode, dynamically-added tabs default to hidden (closed) instead of visible (open).

## Design

### 1. Default: Dump logs on exit

After `p.Run()` returns and the alt screen is restored, dump all buffered process output to stdout in merged format before cleanup.

**Format:** Each line prefixed with the workspace label, matching the merged view style:
```
workspace-a  Starting dev server...
workspace-b  Compiled successfully
workspace-a  Error: port 3000 already in use
```

**Implementation points:**
- Add a `DumpLogs(pm *ProcessManager)` function in a new file `internal/tui/dump.go`
- Iterates processes, reads each `RingBuffer.Lines()`, interleaves chronologically
- Labels are padded to max label width for alignment
- No color in dump output (plain text for piping/CI compatibility)
- Called in both `Launch()` and `DelegateLaunch()` after `p.Run()` returns, before `StopAll()`
- Skipped when `TuiCleanExit` is true

**Chronological ordering note:** RingBuffers store per-process output without global timestamps. For the dump, we'll output grouped by process (all lines from process 1, then process 2, etc.) with a label header per group. This is the simplest correct approach — interleaved ordering would require timestamps we don't have.

### 2. Config: `tui` field becomes polymorphic

Following the existing `repo` field pattern (string or object):

**String form (existing, unchanged):**
```json
{ "tui": "tabbed" }
```

**Object form (new):**
```json
{ "tui": { "mode": "tabbed", "cleanExit": true } }
```

**Config struct changes:**
- Replace `Tui string` with `TuiMode string` and `TuiCleanExit bool`
- Add `parseTuiField()` following `parseRepoField()` pattern
- Add `configLoad.TuiRaw json.RawMessage` replacing `configLoad.Tui string`
- Update `TuiEnabled()` to use `TuiMode`
- Update `MarshalJSON` / serialization to support both forms

### 3. Bug fix: Merged mode tabs default closed in turbo mode

**Root cause:** `NewMergedModel()` initializes `visible[i] = true` for processes that exist at creation time. In delegated mode, processes are added dynamically via `pm.Add()` after model creation, so they never get a `visible` entry. Missing map keys return `false`, making them hidden.

**Fix:** In `MergedModel.Update()`, when handling `ProcessOutputMsg` or `ProcessStateMsg`, check if the process index exists in `visible`. If not, set it to `true`.

## Files to modify

1. `internal/config/config.go` — polymorphic tui field, new TuiMode/TuiCleanExit fields
2. `internal/tui/launch.go` — call DumpLogs after p.Run()
3. `internal/tui/delegate.go` — call DumpLogs after p.Run()
4. `internal/tui/merged.go` — fix visibility for dynamically-added processes
5. `internal/tui/dump.go` — new file: DumpLogs function

## Non-goals

- Persisting logs to files (not requested)
- Configurable buffer size
- Color in dump output
