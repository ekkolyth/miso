# miso agent skills

AI agent skills for working with [miso](https://misojs.dev) — the smart package manager wrapper and script runner.

Install all skills at once:

```bash
npx skills add ekkolyth/miso --all
```

Install for a specific agent (e.g. OpenCode):

```bash
npx skills add ekkolyth/miso --all -a opencode
```

Install a specific skill:

```bash
npx skills add ekkolyth/miso --skill miso-env
```

---

## Available Skills

| Skill | When to use | What it covers |
|---|---|---|
| `miso-config` | Creating or modifying `miso.json` | All top-level fields, types, defaults, annotated examples, common mistakes |
| `miso-scripting` | Creating, organizing, or debugging scripts | Scripts folder, extension dispatch, shebang, subdirectories, resolution order |
| `miso-tui` | Configuring multi-process TUI, concurrent tasks, task ordering | `tui` field, `repo` modes, `tasks.concurrent`, `tasks.dependsOn` |
| `miso-env` | Configuring env validation or debugging env injection | `env` config, variable types, `--env` flag, injection behavior |

---

Full documentation: [misojs.dev](https://misojs.dev)
