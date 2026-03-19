# Miso Skills CLI Flags: Design Spec

**Date:** 2026-03-19
**Status:** Draft
**Branch:** feature/tui-script-execution

## Problem

Users who install the miso agent skills via `npx skills add` must use a long subpath URL to target only the distributable skills in `packages/skills/`, rather than accidentally pulling in every `SKILL.md` in the repo. This is poor UX. Additionally, users who run `miso skills add` intending to use their package manager would silently pass through to the PM, potentially doing the wrong thing.

The goal is to give miso users a first-class `miso skills --add` and `miso skills --rm` command that handles the correct subpath URL automatically.

## Solution

Add a `skills` case to miso's pre-config command handler. When `--add` or `--rm` is present, miso runs the appropriate `npx skills` invocation with the correct subpath baked in. When neither flag is present, `miso skills` falls through to normal routing (PM passthrough), so `miso skills add lodash` still works as expected.

The implementation lives in a new `internal/cli/commands/skills.go` file, consistent with how other meta-commands like `upgrade` are structured.

## Design

### Behavior

| Command | Result |
|---|---|
| `miso skills --add` | `npx skills add "https://github.com/ekkolyth/miso/tree/main/packages/skills" --all` |
| `miso skills --rm` | `npx skills remove miso-config miso-scripting miso-tui miso-env` |
| `miso skills` | fall through to normal routing → PM passthrough |
| `miso skills --add --rm` | error: flags are mutually exclusive |
| `miso skills add lodash` | fall through → PM passthrough (`bun add lodash` / `npm install lodash` etc.) |

### New file: `internal/cli/commands/skills.go`

Three exported functions:

**`ParseSkillsFlags(args []string) (add bool, rm bool)`**
Linear scan of `args` for `"--add"` and `"--rm"`. Returns booleans. Does not modify `args`.

**`RunSkillsAdd() error`**
Execs:
```
npx skills add "https://github.com/ekkolyth/miso/tree/main/packages/skills" --all
```
Uses `manager.Exec` (same pattern as `upgrade.go`) with stdout/stderr inherited so the interactive skills CLI renders correctly in the terminal.

**`RunSkillsRemove() error`**
Execs:
```
npx skills remove miso-config miso-scripting miso-tui miso-env
```
Same exec pattern.

### Change to `cmd/main.go`

Add a `case "skills":` block in the pre-config `switch args[0]`, immediately after `case "upgrade":`:

```go
case "skills":
    add, rm := commands.ParseSkillsFlags(args[1:])
    if add && rm {
        cli.Fail(logger, fmt.Errorf("--add and --rm are mutually exclusive"), false)
    }
    if add {
        if err := commands.RunSkillsAdd(); err != nil {
            cli.Fail(logger, err, false)
        }
        return
    }
    if rm {
        if err := commands.RunSkillsRemove(); err != nil {
            cli.Fail(logger, err, false)
        }
        return
    }
    // Neither flag present: fall through to normal routing (PM passthrough)
```

No `return` or `break` at the end — execution continues past the switch into project root lookup and eventually `ActionPassthrough`, so `miso skills add lodash` works as a PM command.

### Exec pattern

`upgrade.go` uses `manager.Exec(spec, "")` where `spec` is a `manager.CommandSpec`. The skills commands use the same pattern, passing `npx` as the command with appropriate args. Working directory is `""` (inherits caller's CWD), which is correct — `npx skills` installs to the project or global agent config, not to a specific directory.

### Shell completion

No changes needed. `miso skills` as a bare command already completes via PM passthrough. The `--add` and `--rm` flags are not added to completion — consistent with `--local` on `upgrade`, which is also not in the completion list.

## Out of Scope

- A `miso skills --update` command (users can run `npx skills update` directly)
- Configuring which skills to install/remove via `miso.json`
- Support for `--add` and `--rm` in simple mode (these are development tooling commands, not script execution)
