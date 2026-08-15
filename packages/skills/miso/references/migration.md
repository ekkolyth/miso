# Migration

Two hops are documented: **0.6.x → 0.7.0** first, then **0.5.x → 0.6.0** below. Coming from 0.5.x, read both — and note that 0.7.0 reverses one 0.6.0 rule (root-vs-fan-out precedence), flagged inline where it appears.

## Upgrade the binary

```bash
miso upgrade
```

Downloads the latest binary and swaps it in place, no sudo. Homebrew users run `brew upgrade miso`; npm users reinstall `@ekkolyth/miso` at the new version.

> Windows binaries are no longer published. macOS (Intel + Apple Silicon) and Linux (x86-64 + ARM64) only.

### Refresh the skill

The `miso` agent skill isn't bundled in the binary — `miso skills --add` pulls the current reference each time it runs. Re-run it after upgrading so your editor's skill matches the new binary:

```bash
miso skills --add
```

The binary and the skill aren't version-locked, so an upgraded binary can sit next to a stale skill until you re-add. Make `miso upgrade` → `miso skills --add` a habit — run the pair whenever a new miso ships to stay current.

# 0.6.x → 0.7.0

Script resolution collapsed into one ladder that reads the same in every repo mode. Most projects need no edit. The ones that do relied on a root script shadowing workspace fan-out, or had a `concurrent` entry that was silently doing nothing.

## The resolution ladder

`miso <name>` resolves in this order, in every mode:

1. **`repo.tasks` entry** — miso orchestrates. The task supplies `concurrent` and `dependsOn`; the command body still resolves through the steps below.
2. **Delegated pipeline task** — turbo/nx mode only. A name in `turbo.json` runs under turbo inside miso's wrapper.
3. **Workspace member fan-out** — every member defining the name, in the shared TUI. The root is not a member.
4. **Root script** — root `package.json` script or `scripts/` file, single pane.
5. **Passthrough** — forwarded to the package manager.

A task entry never carries a command body. It decorates whatever steps 3–4 resolve.

## Breaking changes

### Root no longer preempts workspace fan-out

**This reverses the 0.6.0 rule below.** When workspace members exist, the root is the orchestrator, not a fan-out member: root scripts no longer short-circuit fan-out, and a root script runs as the body only when no member defines the name.

The practical win is that the turbo-style entry point is now safe:

```jsonc
// root package.json — 0.6.0: infinite recursion. 0.7.0: correct.
{ "scripts": { "dev": "miso dev" } }
```

If a root script was deliberately shadowing fan-out, declare it as a task or address it explicitly:

```jsonc
// before (0.6.0) — root scripts/dev.sh silently won
{}

// after (0.7.0) — pick one
{ "repo": { "tasks": { "dev": { "concurrent": ["#dev"] } } } }
```

Single repos and simple mode are unaffected — no members means no fan-out set, so the root script is the body as always.

### `concurrent` entries that resolve nowhere now fail the launch

Previously skipped in silence, which is how a typo'd or misplaced companion could sit dead in a config for months. Now:

```
concurrent "db/up": no script found at root scope (scripts folder "./scripts" and root package.json)
```

The error names the entry and every location searched. Fix the name or create the script.

### A turbo task and a miso script can't share a name

In a delegated repo, a name claimed by both `turbo.json` and a miso script used to let the script win silently. Now it's an error:

```
"build" is both a turbo task and a miso script — declare repo.tasks.build to have miso own it, or rename the script so turbo owns it
```

### An empty task entry is a config error

A `repo.tasks` entry that resolves no body anywhere and declares no `concurrent`/`dependsOn` describes nothing:

```
repo.tasks.ghost declares nothing: no script named "ghost" found and no concurrent entries
```

### Plain scripts in delegated repos now get chrome

In turbo/nx mode, a name turbo doesn't own used to exec literally with no TUI. It now runs under miso's single-pane chrome. `"tui": "off"` restores plain output.

Nx repos see more of this than turbo repos: miso parses `turbo.json` only, so in nx mode every name without a `repo.tasks` entry runs under miso's own orchestration rather than falling through.

## New in 0.7.0

### `#name` addresses the root explicitly

`concurrent` entries now have three forms:

| Form | Resolves against |
|---|---|
| `name` | the scope declaring it — a root list at root, a member list in that member |
| `#name` | the repo root, explicitly, regardless of declaring scope |
| `@member/name` | the named workspace member |

### A root task's companions spawn once during fan-out

A root `concurrent` list runs once per invocation no matter how many members fan out — so one Compose stack alongside every member's dev server:

```jsonc
{ "repo": { "tasks": { "dev": { "concurrent": ["#db/up"] } } } }
```

Member lists stay member-scoped; a root list never broadcasts into members.

### A `repo.tasks` entry is never shadowed by a same-named script

Under delegation, a folder script sharing a task's name used to win silently, leaving the task entry dead. Step 1 of the ladder now runs first — declaring `repo.tasks.<name>` means miso owns that name, and the script becomes its body.

# 0.5.x → 0.6.0

Upgrading from 0.5.x (e.g. 0.5.2) to 0.6.0. Most projects need one or two small edits — the `repo` field, a `scope` on each env entry if you validate env, and possibly a `tui` or root-script tweak if you relied on the old orchestration defaults. Everything else is either automatic or additive.

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

### `tui` default flips from `off` to `tabbed`

A repo with no `tui` field now renders the tabbed chrome on a TTY — including single-process repos, since there's no minimum process count for chrome. Set `tui: "off"` to keep the old plain look:

```jsonc
// before (0.5.x) — no tui field, default off, plain stdout
{}

// after (0.6.0) — no tui field, default tabbed, chrome on a TTY
{}

// to keep plain output
{ "tui": "off" }
```

### `tui: "off"` no longer means "no orchestration"

Previously `off` (or the pre-0.6.0 default) meant miso stepped aside and proxied straight to the package manager at the repo root — no fan-out, no `concurrent`, no `dependsOn`. Now `off` only drops the chrome: miso still discovers workspace members, starts `concurrent` companions, and orders `dependsOn` levels — it streams plain `[label] line` output instead of drawing tabs. Headless runs (no TTY — CI, agents) work the same way: plain output, full orchestration.

A project that set `tui: "off"` specifically to avoid fan-out (not just the chrome) now gets it on every `miso <script>` run. See the next entry if that fan-out is hitting a root script unexpectedly.

### A root script now wins over workspace fan-out

> **Reversed in 0.7.0.** Root is the orchestrator now — it neither joins nor preempts fan-out. Going straight to 0.7.0, skip this entry and read [Root no longer preempts workspace fan-out](#root-no-longer-preempts-workspace-fan-out) instead.

`miso <script>` resolves the root scope first — its `scripts/` folder and its `package.json`. If the script exists at root, miso runs that one process and stops; it only fans out to workspace members when the root has no matching script. This applies to every script, not just `dev`.

A repo that carries a root `"dev": "..."` (or `scripts/dev.sh`) expecting `miso dev` to fan out across every member now gets just the root script instead. Remove the root script to restore fan-out.

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

## Also new in 0.6.0

Nothing to migrate — just worth knowing once you're upgraded:

- **Interactive TUI mode.** Press `i` to forward keystrokes to the focused process, `Ctrl+Z` to hand control back to miso. On Unix each task runs in its own pseudo-terminal, so TTY-gated tools render as if run directly. See `miso-tui`.
- **Env template generation.** `miso env -g` writes a keys-only `.env.generated` per scope; add `-p` to fill from baselines, `-o` to layer overrides, or `-x` for a type-valid `.env.example`. See `miso-env`.
- **One unified skill + harness selector.** The agent skills collapsed into a single `miso` skill that adapts to your package manager and orchestration engine. Re-run `miso skills` to refresh an older install.
