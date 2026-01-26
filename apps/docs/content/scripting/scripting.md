# Scripting with Miso

## Overview

Miso's script system supports discovering and executing scripts from multiple sources with a clear resolution order. Scripts can be defined in a scripts folder (like Makefile targets) or in package.json, with support for shebang-based execution in any language.

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

### Discovery

Miso scans the scripts folder for executable files and files with known script extensions:

- **Executable files** - Files with the executable bit set (regardless of extension)
- **Known extensions** - Files with extensions: `.sh`, `.bash`, `.zsh`, `.js`, `.mjs`, `.ts`, `.py`, `.rb`, `.pl`, `.lua`, `.php`

Hidden files (starting with `.`) and directories are skipped.

### Script Naming

Scripts are identified by their basename (filename without extension). For example:
- `scripts/jump.sh` → accessible as `miso jump`
- `scripts/jump.py` → accessible as `miso jump`
- `scripts/build.js` → accessible as `miso build`

### Extension Conflicts

If multiple files share the same basename (e.g., `jump.sh` and `jump.py`), Miso will report a conflict:

```
multiple scripts named 'jump' found: jump.sh, jump.py. use 'miso jump.sh' or 'miso jump.py'
```

To resolve conflicts, use the explicit filename with extension:
- `miso jump.sh` - runs `scripts/jump.sh`
- `miso jump.py` - runs `scripts/jump.py`

### Explicit Extension Support

You can always use the explicit filename with extension:
- `miso jump.sh` - runs `scripts/jump.sh` even if `jump.py` also exists
- `miso build.js` - runs `scripts/build.js`

## Script Execution

### Shebang Detection

Miso reads the first line of script files to detect shebangs:

```bash
#!/usr/bin/env node
// script content
```

If a shebang is found, Miso extracts the interpreter and executes:
```
interpreter [interpreter-args] script-path [script-args]
```

### Extension-Based Interpreter Selection

If no shebang is present, Miso selects an interpreter based on the file extension:

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

### Direct Execution

If a file has the executable bit set and no extension, Miso will attempt to execute it directly:

```bash
chmod +x scripts/my-script
miso my-script
```

### Error Handling

- **Missing interpreter**: If the interpreter specified in the shebang or detected by extension is not found in PATH, Miso reports:
  ```
  interpreter 'python3' not found in PATH. install it or update script shebang
  ```

- **No interpreter found**: If a script has no shebang, no known extension, and is not executable:
  ```
  no interpreter found for script "script-name". add shebang or use known extension
  ```

## package.json Scripts

Miso falls back to package.json scripts if a command is not found in the scripts folder. Scripts are read from the `scripts` field:

```json
{
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build"
  }
}
```

These scripts are executed via the configured package manager (e.g., `pnpm run dev`).

## Passthrough Behavior

If a command is not found in built-in commands, scripts folder, or package.json, Miso forwards it to the package manager with a notification:

```
miso: command "jump" not found in scripts folder or package.json, forwarding to pnpm
```

The command is then executed as: `pnpm jump [args...]`

## Scripts List Command

The `miso scripts` command lists all available scripts from both sources:

```
available scripts

scripts folder:
  build (build.sh)
  test (test.sh)

package.json:
  dev: vite
  lint: eslint .
```

Scripts are grouped by source and sorted alphabetically within each group.

## Examples

### Basic Script Execution

```bash
# scripts/jump.sh
#!/bin/sh
echo "Jumping!"

# Execute
miso jump
```

### Python Script

```bash
# scripts/process.py
#!/usr/bin/env python3
import sys
print(f"Processing: {sys.argv[1:]}")

# Execute
miso process arg1 arg2
```

### JavaScript Script with Shebang

```bash
# scripts/build.js
#!/usr/bin/env node
console.log('Building...');

# Execute
miso build
```

### Extension Conflict Resolution

```bash
# scripts/jump.sh
#!/bin/sh
echo "Shell jump"

# scripts/jump.py
#!/usr/bin/env python3
print("Python jump")

# Resolve conflict by using explicit extension
miso jump.sh  # runs jump.sh
miso jump.py   # runs jump.py
```

### package.json Fallback

```json
// package.json
{
  "scripts": {
    "dev": "vite"
  }
}

// Execute (if not in scripts folder)
miso dev  # runs via package manager
```

## Error Messages

### Extension Conflicts

```
multiple scripts named 'jump' found: jump.sh, jump.py. use 'miso jump.sh' or 'miso jump.py'
```

### Missing Interpreter

```
interpreter 'python3' not found in PATH. install it or update script shebang
```

### No Interpreter Found

```
no interpreter found for script "script-name". add shebang or use known extension
```

## Edge Cases

- **Scripts folder doesn't exist**: Silently skipped (not an error)
- **package.json missing**: Silently skipped (not an error)
- **Empty scripts folder**: No scripts listed, falls back to package.json
- **Empty package.json scripts**: No scripts listed, falls back to passthrough
- **Hidden files**: Skipped during discovery
- **Directories**: Skipped during discovery
