# Env Injection: Load .env Files Into Script Processes

**Date:** 2026-03-17
**Status:** Draft
**Branch:** feature/tui-script-execution

## Problem

Miso validates `.env` files but never injects their values into the scripts it runs. Scripts spawned by miso inherit only the parent shell's environment, which means variables defined exclusively in `.env` files (like `DATABASE_URL`) are empty at runtime. This causes silent failures — scripts run but can't connect to databases, APIs, or other services that rely on env-configured credentials.

## Solution

Automatically load registered `.env` files into the process environment of every script miso orchestrates. Process environment variables take precedence over `.env` file values (standard dotenv behavior). The existing `--env` flag becomes an opt-in validation gate rather than the trigger for loading.

## Design

### New function: `env.BuildProcessEnv`

```go
func BuildProcessEnv(projectRoot string, cfg config.Config, workspaceDir string) ([]string, error)
```

**Behavior:**

1. Start with `os.Environ()` as the base (current shell environment)
2. Determine which env entries to load:
   - **Single repo / simple mode:** All configured entries. If none configured, discover using standard order (`.env.local` → `.env.production` → `.env.development` → `.env`) from the project root.
   - **Monorepo with env config:** Only entries whose resolved `path` falls under `workspaceDir`. If `workspaceDir` is the project root (root-level script), load all entries.
   - **Monorepo without env config:** Discover per-workspace using the standard discovery order, searching within `workspaceDir`. Each workspace gets only its own `.env` file. Uses workspace paths already resolved from `package.json`.
3. For each matching entry, read the file via `godotenv.Read(path)` → `map[string]string`
4. Merge into the base: **process env wins** — only set vars that aren't already present in the shell environment
5. Return the merged `[]string` in `KEY=VALUE` format (ready for `cmd.Env`)

**Monorepo path matching:** Resolve the entry's `path` to an absolute path and check if it starts with the workspace directory. E.g., entry `"path": "apps/web/.env.local"` resolves to `/project/apps/web/.env.local` — starts with workspace `/project/apps/web/`, so it matches.

**When no env config exists and discovery finds nothing:** Return `nil`. A nil return means "use default inherited env" — `cmd.Env = nil` inherits from the parent process, preserving current behavior.

### Modified: `ExecScriptFile`

Add an `environ []string` parameter:

```go
func ExecScriptFile(scriptPath string, args []string, workDir string, defaultShell string, environ []string) error
```

When `environ` is non-nil, set `cmd.Env = environ`. When nil, don't set it (Go inherits from parent — same as today).

### Injection points

Env injection happens wherever **miso spawns a process it orchestrates**:

| Call site | What it does | Inject? |
|-----------|-------------|---------|
| `main.go` — `ActionScriptFolder` | Folder script in normal mode | Yes |
| `main.go` — `ActionScriptOverride` | Script override (folder script overriding a built-in command) | Yes |
| `main.go` — `ActionWorkspaceScript` | Workspace-scoped folder script in monorepo | Yes, per-workspace |
| `main.go` — simple mode block | Folder script in simple mode | Yes |
| `tui/launch.go` — `Launch` loop | TUI-spawned process (including turbo/nx task overrides) | Yes, per-workspace |
| `commands.Run` | PM runs package.json script (`bun run dev`) | No — PM handles env |
| `commands.Dev` | PM runs dev script | No — PM handles env |
| `commands.RunMultiple` | PM runs multiple scripts | No — PM handles env |
| `tui.DelegateLaunch` | Turbo/nx spawns processes | No — turbo/nx handles env |
| `cli.RunPassthrough` | PM passthrough | No — PM handles env |
| `cli.RunMisox` | PM exec (npx/bunx) | No — PM handles env |

**Rule:** If miso is spawning the process, inject. If a package manager, turbo, or nx is spawning it, don't.

### TUI process env injection mechanism

The TUI process manager (`process.go`) builds `exec.Command` directly — it does not go through `ExecScriptFile`. To inject env into TUI-spawned processes:

1. Add an `Environ []string` field to the `Process` struct
2. In `Launch` (launch.go), after building each process entry, call `BuildProcessEnv(root, cfg, entry.WorkspaceDir)` and store the result on the `Process`
3. In `ProcessManager.Start()`, set `p.cmd.Env = p.Environ` before starting the command (when non-nil)

### `--env` flag behavior

**Current:** `--env` triggers validation. No injection ever happens.

**New:** Injection is automatic (always, when env entries exist). `--env` adds a validation gate:

1. Build env (always)
2. If `--env` flag present: validate → fail on errors → strip the flag
3. Run script with injected env

`miso env` as a standalone command remains a validation-only tool. It does not inject (there's no script to inject into).

### Precedence rules

**Process env wins over `.env` file values.** If `PORT=5000` is set in the shell and `.env` has `PORT=3000`, the script sees `PORT=5000`. The `.env` file fills in variables that aren't already set. This matches standard dotenv behavior across ecosystems.

### Example walkthrough

Project structure:
```
my-project/
├── miso.json
├── .env.local          ← DATABASE_URL=postgres://..., PORT=3000
├── scripts/
│   └── migrate/
│       └── up.sh       ← runs goose, needs DATABASE_URL
```

`miso.json`:
```json
{
  "packageManager": false,
  "scripts": "./scripts",
  "env": {
    "path": ".env.local",
    "variables": {
      "DATABASE_URL": "url",
      "PORT": "port"
    }
  }
}
```

Running `miso migrate/up`:

1. Miso reads `.env.local` → gets `{DATABASE_URL: "postgres://...", PORT: "3000"}`
2. Checks the shell env → neither variable is set
3. Merges both into the script's environment
4. Spawns `scripts/migrate/up.sh` with the merged environment
5. Script sees `DATABASE_URL=postgres://...` and connects successfully

If `PORT=5000` is already set in the shell, miso keeps `5000` — the file's `3000` is ignored for that variable. The shell always wins.

### Monorepo scoping

In a monorepo with multiple env entries:

```json
"env": [
  { "label": "web", "path": "apps/web/.env.local", ... },
  { "label": "api", "path": "apps/api/.env", ... }
]
```

When running a script in the `web` workspace, miso resolves the entry paths and checks which fall under the workspace directory. Only `apps/web/.env.local` matches — the `api` entry is not loaded. This prevents env vars from bleeding across workspace boundaries.

Root-level scripts (run from project root, not inside a workspace) get all entries loaded.

### Edge cases

- **Monorepo with no env config:** Discover `.env` files per-workspace using the standard discovery order (`.env.local` → `.env.production` → `.env.development` → `.env`), searching within each workspace directory. Each workspace gets only the env from its own directory. This uses the workspace paths already resolved from `package.json`.
- **Multiple entries with overlapping variables:** Entries are processed in config array order. Later entries overwrite earlier ones. Process env still wins over all file values.
- **Missing `.env` file during injection:** Soft-fail with a warning log. Don't abort the script — the file might be optional (e.g., `.env.local` in CI). Validation via `--env` or `miso env` is the mechanism for hard-failing on missing files.
- **`--env` with TUI:** Validation happens once before TUI launch (in `main.go`), not per-process inside the TUI. The flag is stripped before the TUI sees it.

## Approach

**Approach C: Build env slice once, inject into exec helpers.** One new function (`BuildProcessEnv`) builds the `[]string` environment. One new parameter on `ExecScriptFile` (`environ []string`) accepts it. The TUI process spawning loop calls `BuildProcessEnv` per-process with the workspace dir. No structural changes to Config.

Alternatives considered:
- **Per-call-site building:** Same logic but duplicated at each spawn point. Error-prone.
- **Config-level preloading:** Loads env into Config struct at startup. Adds weight to Config and threads the loaded env through every path unnecessarily.
