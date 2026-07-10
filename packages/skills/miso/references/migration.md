# Migrating to 0.6.0

Upgrading from 0.5.x (e.g. 0.5.2) to 0.6.0. Most projects need one or two small edits — the `repo` field, and a `scope` on each env entry if you validate env. Everything else is either automatic or additive.

## Upgrade the binary

```bash
miso upgrade
```

Downloads the latest binary and swaps it in place, no sudo. Homebrew users run `brew upgrade miso`; npm users reinstall `@ekkolyth/miso` at the new version.

> Windows binaries are no longer published. 0.6.0 ships macOS (Intel + Apple Silicon) and Linux (x86-64 + ARM64) only.

## Breaking changes

### `repo` no longer accepts `single` or `mono`

The `repo` field now takes only `miso`, `turbo`, or `nx`. `single` and `mono` are gone — always-on workspace discovery made both behave exactly like `miso`, so they collapse into it. A config still using them fails to load:

```
repo: unknown mode "single" (use miso, turbo, or nx)
```

Drop the field (it defaults to `miso`) or set it explicitly:

```jsonc
// before (0.5.x)
{ "repo": "single" }
{ "repo": "mono" }
{ "repo": { "mode": "mono" } }

// after (0.6.0)
{}                       // repo omitted → "miso"
{ "repo": "miso" }
```

`turbo` and `nx` are unchanged, in both the string and object (`{ "mode": "turbo", "tasks": { … } }`) forms. Workspace membership is auto-detected from `package.json` workspaces or `pnpm-workspace.yaml` regardless of `repo` — see `miso-config`.

### Root `env` entries now require a `scope`

If you use an `env` block, every **root** entry must declare a `scope` — the target it applies to, or the reserved `"global"`. A root entry without one is a config error:

```
env config invalid — every root entry needs a scope (a target name or "global")
```

Add a `scope` to each root entry:

```jsonc
// before (0.5.x)
"env": [
  { "path": ".env.local", "variables": { "PORT": "port" } }
]

// after (0.6.0)
"env": [
  { "scope": "global", "path": ".env.local", "variables": { "PORT": "port" } }
]
```

Auto-discovery (no `env` block) is unaffected. Member `miso.json` files scope by file location and don't set `scope`. See `miso-env`.

### Workspace scoping is verb-first

The prefixed `miso @workspace/script` form is removed — put the `@scope` after the script name:

```bash
# before
miso @web/dev
miso @api/build

# after
miso dev @web
miso build @api
```

The old form now errors with a pointer to the new syntax. Pass more than one scope to fan out (`miso dev @web @api`); anything after `--` is passed through and never treated as a scope. See `miso-scripting`.

### `miso misox` is now just `misox`

`misox` is a standalone binary again, not a `miso` subcommand. A typed `miso misox …` falls through to your package manager, so call it directly:

```bash
# before
miso misox create-vite

# after
misox create-vite
```

The curl and npm installers ship `misox` alongside `miso`.

### `miso version` prints the bare version

Output is just the version string now, so it parses cleanly:

```bash
# before
$ miso version
miso 1.2.3

# after
$ miso version
1.2.3
```

Update any script that scraped the `miso ` prefix.

## Behavior changes (no edit required)

- **Env scope and injection are miso-mode only.** In a delegated repo (`repo: "turbo"`/`"nx"`) the delegate owns env — `miso env` type-checks the declared root entries only; nothing is scoped or injected. See `miso-env`.
- **`.bash` scripts run under `bash`.** Previously `sh`. `.sh` still uses `sh`. See `miso-scripting`.
- **A name defined in both `scripts/` and `package.json` is an error.** miso stops and asks you to rename one instead of quietly picking a winner.
- **Workspace discovery is always on.** Members are detected from the package manager's own config independent of `repo`, so a monorepo no longer needs `repo: "mono"` to fan out.

## New in 0.6.0

Nothing to migrate — just worth knowing once you're upgraded:

- **Interactive TUI mode.** Press `i` to forward keystrokes to the focused process, `Ctrl+Z` to hand control back to miso. On Unix each task runs in its own pseudo-terminal, so TTY-gated tools render as if run directly. See `miso-tui`.
- **Env template generation.** `miso env -g` writes a keys-only `.env.generated` per scope; add `-p` to fill from baselines, `-o` to layer overrides, or `-x` for a type-valid `.env.example`. See `miso-env`.
- **One unified skill + harness selector.** The agent skills collapsed into a single `miso` skill that adapts to your package manager and orchestration engine. Re-run `miso skills` to refresh an older install.
