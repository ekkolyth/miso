# Workspace Script Syntax Redesign

**Date:** 2026-03-19  
**Status:** Approved

## Problem

In monorepo mode, `package.json` scripts with colons in their names (e.g. `test:unit`, `build:prod`) cause miso to break. The CLI parser in `router.go` greedily splits any colon-containing command on `:` and routes it as a `workspace:script` command — before ever checking whether the full name exists as a script. This means `miso test:unit` in a monorepo tries to find a workspace named `test` with a script named `unit`, and fails.

Additionally, `ResolveWorkspaceScript` only checks the workspace's `scripts/` folder and does not fall through to the workspace's `package.json`. And `FindWorkspace` only matches on directory basename, which breaks for scoped package names (`@myorg/api`) and path-based references (`packages/api`).

This is all new functionality on a feature branch with no stable users. It can be redesigned cleanly.

## Decision

1. Replace the `workspace:script` colon syntax with `@workspace/script` — an unambiguous sigil that mirrors pnpm/yarn/turbo workspace conventions and frees `:` for script names entirely.
2. Redesign `FindWorkspace` to match by all three identifiers: directory basename, relative path from root, and `package.json` `name` field. This mirrors how `--filter` works in pnpm and turbo.
3. Extend `ResolveWorkspaceScript` to fall through to the workspace's `package.json`, giving workspace commands the same resolution chain as root commands.

## Design

### 1. CLI Parsing (`internal/cli/router.go`)

The colon-split block (currently lines 155–168) is replaced with `@workspace/script` detection:

- A command starting with `@` and containing `/` is a workspace-scoped command
- Parse: strip `@`, split on the **last** `/`
  - Everything before the last `/` is the workspace identifier
  - Everything after the last `/` is the script name
- Examples:
  - `@api/test` → workspace `api`, script `test`
  - `@api/test:unit` → workspace `api`, script `test:unit` (colon preserved in script name)
  - `@myorg/api/build` → workspace `myorg/api`, script `build` (scoped package name)
  - `@packages/api/build` → workspace `packages/api`, script `build` (path-based)
- A command starting with `@` but containing no `/` returns an error: `usage: @<workspace>/<script>`
- Commands not starting with `@` proceed directly to `ResolveScript` — colon-named scripts like `test:unit` now resolve correctly

Resolution order for non-`@` commands (unchanged):
1. Built-in command check (install, add, remove, run, dev, etc.)
2. `ResolveScript` — scripts folder, then package.json
3. Passthrough to package manager

### 2. Workspace Lookup (`internal/config/config.go`)

`FindWorkspace` is redesigned to match a workspace identifier against three candidates per workspace, in order:

1. **Directory basename** — `filepath.Base(wsPath)` e.g. `api` for `packages/api`
2. **Relative path from root** — the path relative to the project root e.g. `packages/api`
3. **Package name** — the `name` field from the workspace's own `package.json` e.g. `@myorg/api`

`FindWorkspace` signature changes to accept the project root so it can resolve relative paths and read workspace `package.json` files:

```go
func FindWorkspace(name string, workspaces []string, root string) (string, error)
```

Return type changes from `(string, bool)` to `(string, error)` to carry the ambiguity error.

**Ambiguity:** If the identifier matches more than one workspace across any of the three candidate types, return an error listing the conflicting matches and asking the user to be more specific (e.g. use the full relative path or full package name).

**No match:** Return an empty string and a "workspace not found" error listing available workspaces by basename for discoverability.

### 3. Workspace Script Resolution (`internal/cli/scripting/workspace.go`)

`ResolveWorkspaceScript` is updated to:

1. Call the updated `FindWorkspace` (passing `root`) and propagate its error directly — this surfaces both "not found" and "ambiguous" errors correctly
2. Check workspace `scripts/` folder (existing behavior)
3. If not found, read `package.json` from the workspace directory and look up the script name
4. If found in `package.json`, return `ScriptSourcePackageJSON` with the command as `Path`
5. If not found in either, return `ScriptSourceNone`

### 4. Execution (`cmd/main.go`)

The `ActionWorkspaceScript` handler (currently lines 321–339) handles only `ScriptSourceFolder`. It is extended to handle `ScriptSourcePackageJSON`:

- `ScriptSourceFolder`: unchanged — call `scripting.ExecScriptFile` with `workDir`
- `ScriptSourcePackageJSON`: call `commands.Run(managerName, parsed.ScriptName, parsed.ScriptArgs, workDir)` — use `workDir` (workspace directory), not `originalWorkDir`
- `ScriptSourceNone`: error with `"script %q not found in workspace %q"` — this branch is only reached when `FindWorkspace` succeeded but the script was not found in either the `scripts/` folder or `package.json`. `FindWorkspace` errors (not found, ambiguous) are propagated earlier and never reach this branch.
- The existing `err` check after `ResolveWorkspaceScript` (line 326) continues to surface `FindWorkspace` errors directly — no change needed there.

### 5. Tests

**`internal/cli/router_test.go`** (new):
- `@api/test` → `ActionWorkspaceScript`, workspace=`api`, script=`test`
- `@api/test:unit` → workspace=`api`, script=`test:unit`
- `@myorg/api/build` → workspace=`myorg/api`, script=`build`
- `@packages/api/build` → workspace=`packages/api`, script=`build`
- `@workspace` with no `/` → parse error
- `test:unit` in monorepo mode → resolves as a script, not `ActionWorkspaceScript`

**`internal/config/config_test.go`** (extended):
- Matches by basename (`api` → `packages/api`)
- Matches by relative path (`packages/api` → `packages/api`)
- Matches by package name (`@myorg/api` → `packages/api`)
- Ambiguous match (two workspaces share basename) → error listing both
- No match → error listing available workspaces

**`internal/cli/scripting/workspace_test.go`** (new):
- Script found in workspace `scripts/` folder → `ScriptSourceFolder`
- Script not in folder but present in workspace `package.json` → `ScriptSourcePackageJSON`
- Script found in neither → `ScriptSourceNone`

## Files Changed

| File | Change |
|---|---|
| `apps/miso/internal/cli/router.go` | Replace colon-split block with `@workspace/script` detection |
| `apps/miso/internal/config/config.go` | Redesign `FindWorkspace` — match by basename, relative path, and package name; return `error` instead of `bool` |
| `apps/miso/internal/cli/scripting/workspace.go` | Update `FindWorkspace` call; add `package.json` fallback to `ResolveWorkspaceScript` |
| `apps/miso/cmd/main.go` | Handle `ScriptSourcePackageJSON` in `ActionWorkspaceScript` handler |
| `apps/miso/internal/cli/router_test.go` | New — parser tests |
| `apps/miso/internal/config/config_test.go` | Extended — `FindWorkspace` matching tests |
| `apps/miso/internal/cli/scripting/workspace_test.go` | New — workspace resolution tests |
