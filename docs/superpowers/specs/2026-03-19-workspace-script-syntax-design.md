# Workspace Script Syntax Redesign

**Date:** 2026-03-19  
**Status:** Approved

## Problem

In monorepo mode, `package.json` scripts with colons in their names (e.g. `test:unit`, `build:prod`) cause miso to break. The CLI parser in `router.go` greedily splits any colon-containing command on `:` and routes it as a `workspace:script` command — before ever checking whether the full name exists as a script. This means `miso test:unit` in a monorepo tries to find a workspace named `test` with a script named `unit`, and fails.

Additionally, `ResolveWorkspaceScript` only checks the workspace's `scripts/` folder. It does not fall through to the workspace's `package.json`, meaning workspace-scoped commands cannot run `package.json` scripts.

## Decision

Replace the `workspace:script` colon syntax with `@workspace/script`. Use `@` as an unambiguous sigil for workspace scope, mirroring npm/pnpm/yarn workspace conventions. This is a clean break — no backwards compatibility with the old syntax. The feature was introduced on a feature branch and has no stable users.

## Design

### 1. CLI Parsing (`internal/cli/router.go`)

The colon-split block (currently lines 155–168) is replaced with `@workspace/script` detection:

- A command starting with `@` and containing `/` is a workspace-scoped command
- Parse: strip `@`, split on the **last** `/`
  - Everything before the last `/` is the workspace name
  - Everything after the last `/` is the script name
- This correctly handles scoped workspace names: `@scope/pkg/build` → workspace `scope/pkg`, script `build`
- Colon is no longer special in the parser; `@api/test:unit` → workspace `api`, script `test:unit`
- A command starting with `@` but containing no `/` returns an error: `usage: @<workspace>/<script>`
- Commands not starting with `@` proceed directly to `ResolveScript` — colon-named scripts like `test:unit` now resolve correctly through the normal chain

Resolution order for non-`@` commands (unchanged):
1. Built-in command check (install, add, remove, run, dev, etc.)
2. `ResolveScript` — scripts folder, then package.json
3. Passthrough to package manager

### 2. Workspace Script Resolution (`internal/cli/scripting/workspace.go`)

`ResolveWorkspaceScript` is extended to mirror the full resolution chain of `ResolveScript`:

1. Check workspace `scripts/` folder (existing behavior)
2. If not found, read `package.json` from the workspace directory and look up the script name
3. If found in `package.json`, return `ScriptSourcePackageJSON` with the command as `Path`
4. If not found in either, return `ScriptSourceNone`

This gives workspace-scoped commands the same resolution behavior as root commands.

### 3. Execution (`cmd/main.go`)

The `ActionWorkspaceScript` handler (currently lines 321–339) handles only `ScriptSourceFolder`. It is extended to handle `ScriptSourcePackageJSON`:

- `ScriptSourceFolder`: unchanged — call `scripting.ExecScriptFile` with `workDir`
- `ScriptSourcePackageJSON`: call `commands.Run(managerName, scriptName, args, workDir)` — same as `ActionScriptPackageJSON` but with the workspace directory as the working directory
- `ScriptSourceNone`: error as today

### 4. Tests

Two new test files are created since neither currently exists:

**`internal/cli/router_test.go`:**
- `@api/test` parses to `ActionWorkspaceScript` with workspace=`api`, script=`test`
- `@api/test:unit` parses to workspace=`api`, script=`test:unit` (colon in script name preserved)
- `@scope/pkg/build` parses to workspace=`scope/pkg`, script=`build` (scoped workspace name)
- `@workspace` with no `/` returns a parse error
- `test:unit` in monorepo mode resolves as a script via `ResolveScript`, not as `ActionWorkspaceScript`

**`internal/cli/scripting/workspace_test.go`:**
- Script found in workspace `scripts/` folder → `ScriptSourceFolder`
- Script not in folder but present in workspace `package.json` → `ScriptSourcePackageJSON`
- Script found in neither → `ScriptSourceNone`

## Files Changed

| File | Change |
|---|---|
| `apps/miso/internal/cli/router.go` | Replace colon-split block with `@workspace/script` detection |
| `apps/miso/internal/cli/scripting/workspace.go` | Add `package.json` fallback to `ResolveWorkspaceScript` |
| `apps/miso/cmd/main.go` | Handle `ScriptSourcePackageJSON` in `ActionWorkspaceScript` handler |
| `apps/miso/internal/cli/router_test.go` | New — parser tests |
| `apps/miso/internal/cli/scripting/workspace_test.go` | New — workspace resolution tests |
