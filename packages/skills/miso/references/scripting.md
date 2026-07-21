# Scripts

## Scripts Folder

By default, miso looks for scripts in `./scripts` relative to `miso.json`. Override with the `scripts` field:

```json
{ "scripts": "./tasks" }
```

Run `miso scripts` to list all discovered scripts in the current project.

---

## Extension-Based Interpreter Dispatch

Miso selects the interpreter based on file extension:

| Extension | Interpreter |
|---|---|
| `.sh` | `sh` |
| `.bash` | `bash` |
| `.zsh` | `zsh` |
| `.js`, `.mjs` | `node` |
| `.ts` | `bun` or `node` |
| `.py` | `python3` |
| `.rb` | `ruby` |
| `.pl` | `perl` |
| `.lua` | `lua` |
| `.php` | `php` |

`.ts` runs on `bun` when the project uses bun, otherwise `node` (bare `node script.ts` needs Node 23.6+; `bun` runs TypeScript with no extra setup).

**Never add a shebang.** Miso selects the interpreter from the file extension — the extension is how you control which interpreter runs your script. If a script does start with `#!`, that interpreter takes precedence, but this should never be necessary in miso.

The `shell` field in `miso.json` sets the fallback interpreter for scripts with no recognized extension and no shebang.

**Shell scripts get `-e` by default.** Miso automatically passes `-e` (exit on error) to `sh`, `bash`, `zsh`, and other shell interpreters. You do not need `set -e` (or `set -euo pipefail`) at the top of your scripts — miso handles this for you.

---

## Invocation

```bash
miso <scriptname>       # runs scripts/<scriptname>.<ext>
miso build/docs         # runs scripts/build/docs.<ext>  (subdirectory)
miso <scriptname> --arg # passes --arg to the script
```

**You don't need `miso run`** — unlike npm, miso resolves scripts via `miso <name>` directly. `miso run` is supported for muscle memory but is not the intended form.

---

## Resolution Order

When you run `miso <command>`, miso resolves it from **at most one** source within a given scope:
1. `scripts/` folder (script file match)
2. `package.json` `scripts` block
3. Passthrough to the package manager

A name may live in only one of them. If the same name is defined in **both** the `scripts/` folder and `package.json`, miso **errors** and asks you to rename one — it will not silently pick a winner, because the two invoke different things (see below). Two files in the folder that share a name (`dev.sh` and `dev.ts`) are the same kind of error.

In a monorepo, "scope" means the root first, then each workspace member — the same two-source, at-most-one-match rule applies independently within each. miso checks the root before fanning out to members; see `miso-tui` for the full resolution algorithm.

## Folder script vs package.json script — they are not interchangeable

In `turbo`/`nx` mode this distinction matters:

- A **`package.json` script** whose name is a turbo/nx task gets **delegated** — miso runs `turbo run <name>` itself and wraps the output in the miso TUI chrome (tabs). This works because miso controls the invocation (it adds `--log-order=stream`, withholds a pty, and parses turbo's streamed output). This is the way to get the chrome.
- A **folder script** runs **literally** — exactly the command you wrote. miso does **not** wrap it in the turbo chrome, because it can't reliably parse an arbitrary script's output into tabs (the script controls turbo's flags, not miso). You get turbo's own output.

So: want the tabbed chrome → put `"dev": "turbo run dev"` in `package.json`. Want an exact custom command (extra turbo tasks, flags) → use a folder `dev.sh` and accept turbo's own UI. Defining `dev` in both is the conflict above — pick one.

---

## Subdirectory Organization

Scripts can be organized in subdirectories:

```
scripts/
  build/
    docs.sh
    api.sh
  test/
    unit.sh
    e2e.sh
```

Invoke with path syntax: `miso build/docs`, `miso test/e2e`.

Nest related commands rather than prefixing flat filenames. Create `scripts/docker/up.sh` (→ `miso docker/up`), **not** `scripts/docker-up.sh`. Stuttering flat names (`docker-up.sh`, `docker-build.sh`) should be a nested `docker/` folder.

---

## Where to put a command

Pick the surface by size:

- **One-liner → `package.json` scripts.** A single command like `vite --config vite.config.ts` belongs in the `package.json` `scripts` block — miso resolves it the same as a folder script, and it's cleaner than a one-line `.sh`. (Simple mode ignores `package.json`, so folder scripts are the only option there.)
- **Multi-line or `&&`-chained → `scripts/` folder.** Once a command spans multiple lines, chains with `&&`, or needs real logic, a `.sh` file reads far better than a cramped JSON string.
- **Opt-in dev variants → `dev:<name>` in `package.json`.** A dev target that shouldn't run under a bare `miso dev` (an optional service, an alternate entry point) belongs in the workspace's `package.json` under a distinct name — `dev:ekklipse`, `dev:studio` — not a root launcher script. Start it with `miso dev:ekklipse @<workspace>`. Bare `dev` stays the default set.

---

## Workspace-Scoped Scripts (Monorepos)

In a repo with workspaces (auto-detected from your package manager), run a script against specific workspace(s) by adding one or more `@scope` targets **after** the script. The `@` sigil is required.

```bash
miso build @api            # run "build" in the workspace named "api"
miso test @myorg/api       # "test" in the workspace whose package.json name is "@myorg/api"
miso dev @packages/web     # "dev" in the workspace at path "packages/web"
miso test:unit @api        # colons are fine in script names
miso dev @web @api         # multiple scopes at once
```

A scope resolves by identity in **priority order** — first match wins, so a workspace's name is never shadowed by an unrelated one that just shares a folder name:

1. **Full `package.json` name** — `@myorg/api`, or a bare `api`
2. **Scoped short-name** — the segment after `/` in a scoped name (`@myorg/api` → `api`)
3. **Relative path from root** — `packages/api`
4. **Directory basename** — used only when nothing above matches

If two workspaces match in the same tier, miso errors and lists their paths so you can qualify by path (`@packages/api`). In `turbo`/`nx` mode the scope becomes a `--filter` / `--projects`; in miso mode it filters the launched workspace tasks.

> The old `miso @scope/script` prefixed form has been removed — always use `miso <script> @scope`.

---

## Common Mistakes

- **Missing file extension** — `scripts/build` won't be discovered; must be `scripts/build.sh` (or another supported extension)
- **Using `miso run scriptname`** — in miso, the correct form is `miso scriptname` (no `run` subcommand needed)
- **Adding `chmod +x` to scripts** — never needed; miso always invokes the interpreter directly and passes the script as an argument, so the script file itself never needs to be executable
- **Adding `set -e` or `set -euo pipefail`** — not needed; miso automatically passes `-e` to shell interpreters, so scripts already exit on error without any boilerplate
- **Adding a shebang** — never needed; use the file extension to control which interpreter runs the script
