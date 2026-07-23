# miso agent skills

AI agent skills for working with [miso](https://misojs.dev) — the smart package manager wrapper and script runner.

Install for the harnesses miso detects:

```bash
miso skills --add
```

Or install directly with the skills CLI:

```bash
npx skills add ekkolyth/miso
```

Install for a specific agent (e.g. OpenCode):

```bash
npx skills add ekkolyth/miso -a opencode
```

---

## What it covers

| Area | Reference |
|---|---|
| `miso.json` configuration | `references/config.md` |
| Scripts folder, extension dispatch, shebang, resolution order | `references/scripting.md` |
| Multi-process TUI, `repo` modes, `tasks.concurrent`, `tasks.dependsOn` | `references/tui.md` |
| Env validation, variable types, `--env`, injection behavior | `references/env.md` |
| Upgrading a project from an older miso (0.5.x → 0.6.0) | `references/migration.md` |

---

Full documentation: [misojs.dev](https://misojs.dev)
