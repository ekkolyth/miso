---
name: miso-scripting
description: Reference for creating, organizing, and debugging scripts in a miso scripts/ folder
---

# miso-scripting

**Use this skill** any time you need to create, organize, name, or debug scripts in a miso `scripts/` folder.

---

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
| `.ts` | `ts-node` |
| `.py` | `python3` |
| `.rb` | `ruby` |
| `.pl` | `perl` |
| `.lua` | `lua` |
| `.php` | `php` |

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

When you run `miso <command>`, miso checks in this order:
1. `scripts/` folder (script file match)
2. `package.json` `scripts` block
3. Passthrough to the package manager

A script file in the `scripts/` folder overrides a same-named `package.json` script. A `scripts/install.sh` file, for example, replaces `miso install`.

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

---

## Workspace-Scoped Scripts (Monorepos)

In a monorepo (`repo: "mono"`), run a script in a specific workspace using the `@workspace/script` syntax:

```bash
miso @api/build           # run "build" in the workspace identified as "api"
miso @myorg/api/test      # run "test" in the workspace with package name "@myorg/api"
miso @packages/web/dev    # run "dev" in the workspace at path "packages/web"
miso @api/test:unit       # run "test:unit" (colons are fine in script names)
```

The workspace identifier can be any of:
- **Directory basename** — `api` for a workspace at `packages/api`
- **Relative path from root** — `packages/api`
- **Package name** — the `name` field in the workspace's `package.json` (e.g. `@myorg/api`)

If the identifier matches more than one workspace, miso will error and list the conflicting paths.

Miso resolves the script from that workspace's own `scripts/` folder first, then falls back to its `package.json` scripts.

---

## Common Mistakes

- **Missing file extension** — `scripts/build` won't be discovered; must be `scripts/build.sh` (or another supported extension)
- **Using `miso run scriptname`** — in miso, the correct form is `miso scriptname` (no `run` subcommand needed)
- **Adding `chmod +x` to scripts** — never needed; miso always invokes the interpreter directly and passes the script as an argument, so the script file itself never needs to be executable
- **Adding `set -e` or `set -euo pipefail`** — not needed; miso automatically passes `-e` to shell interpreters, so scripts already exit on error without any boilerplate
- **Adding a shebang** — never needed; use the file extension to control which interpreter runs the script
