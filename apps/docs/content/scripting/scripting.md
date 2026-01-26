# Scripting with Miso

## Overview

By default, Miso discovers and executes scripts from the `scripts/` . Scripts can be organized in subdirectories and support any executable language.

## Script Resolution Order

When a command is executed, Miso resolves it in the following order:

1. **Built-in commands** - Commands like `install`, `add`, `remove`, `run`, `dev`, `init`, `version`, `misox`, `update`, and `scripts` are checked first
2. **Scripts folder** - Scripts discovered from the configured scripts folder (default: `./scripts`)
3. **package.json scripts** - Scripts defined in the `scripts` field of package.json
4. **Passthrough** - If not found in any of the above, the command is forwarded to the package manager with a notification

## Scripts Folder

### Configuration

The scripts folder path is configured in `miso.json`:

```json
{
  "scripts": "./scripts"
}
```

If not specified, the default is `./scripts` relative to the project root.

### Execution Context

Scripts execute from the project root. Use relative paths accordingly:

```bash
# scripts/build/miso.sh
#!/bin/sh
cd apps/miso && go build ./cmd
```

### Discovery

Miso recursively scans the scripts directory for:

- **Executable files** - Files with executable permissions
- **Known extensions** - `.sh`, `.bash`, `.zsh`, `.js`, `.mjs`, `.ts`, `.py`, `.rb`, `.pl`, `.lua`, `.php`

Hidden files (starting with `.`) and helper files (starting with `_`) are skipped.

### Script Naming

Scripts are identified by their path relative to the scripts directory:

- `scripts/jump.sh` → `miso jump`
- `scripts/publish/patch.sh` → `miso publish/patch`
- `scripts/build/miso.sh` → `miso build/miso`

### Index Files

An `index.sh` file in a subdirectory acts as the default handler for that directory:

- `scripts/publish/index.sh` → `miso publish` 
- `scripts/build/index.sh` → `miso build` 


### Explicit Extension Support

Use the explicit filename with extension to target a specific file:

- `miso jump.sh` - runs `scripts/jump.sh` even if `jump.py` exists
- `miso publish/patch.sh` - runs `scripts/publish/patch.sh`

## Script Execution

### Shebang Detection

Miso reads the first line to detect shebangs:

```bash
#!/usr/bin/env node
// script content
```

If found, Miso uses the specified interpreter.

### Extension-Based Interpreter Selection

If no shebang is present, Miso selects an interpreter by extension:

| Extension | Interpreter |
|-----------|-------------|
| `.sh`, `.bash` | `sh` |
| `.zsh` | `zsh` |
| `.js`, `.mjs` | `node` |
| `.ts` | `ts-node` |
| `.py` | `python3` |
| `.rb` | `ruby` |
| `.pl` | `perl` |
| `.lua` | `lua` |
| `.php` | `php` |


## package.json Scripts

Miso falls back to `package.json` scripts if not found in the scripts directory:

```json
{
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build"
  }
}
```

These execute via the configured package manager.


If a command is not found in built-in commands, scripts folder, or package.json, Miso forwards it to the package manager.


## Scripts List Command

The `miso scripts` command lists all available scripts from both sources:

```
available scripts

scripts folder:
  build (build/index.sh)
  build/miso (build/miso.sh)
  publish (publish/index.sh)
  publish/patch (publish/patch.sh)

package.json:
  dev: vite
  lint: eslint .
```

Scripts are grouped by source and sorted alphabetically within each group.
