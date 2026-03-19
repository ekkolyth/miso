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

**Shebang takes precedence.** If a script starts with `#!/usr/bin/env python3`, miso uses that interpreter regardless of extension.

The `shell` field in `miso.json` sets the fallback interpreter for scripts with no recognized extension and no shebang.

---

## Invocation

```bash
miso <scriptname>       # runs scripts/<scriptname>.<ext>
miso build/docs         # runs scripts/build/docs.<ext>  (subdirectory)
miso <scriptname> --arg # passes --arg to the script
```

**Do not use `miso run`** — unlike npm, miso resolves scripts via `miso <name>` directly.

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

In a monorepo (`repo: "mono"`), run a script in a specific workspace:

```bash
miso workspace:script    # e.g. miso api:build
```

Miso resolves the script from that workspace's own `scripts/` folder first, then falls back to its `package.json` scripts.

---

## Common Mistakes

- **Missing file extension** — `scripts/build` won't be discovered; must be `scripts/build.sh` (or another supported extension)
- **Using `miso run scriptname`** — in miso, the correct form is `miso scriptname` (no `run` subcommand needed)
- **Shebang scripts not executable** — scripts invoked via shebang need `chmod +x scripts/myscript.sh`
