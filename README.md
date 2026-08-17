<div align="center">

# Meet Miso

Miso is a tiny cli tool that let's you stop worrying about which package manager your projects are using. It lets you work with npm, pnpm, yarn, or bun using a single, consistent command interface.

<img src="https://raw.githubusercontent.com/ekkolyth/miso/main/apps/miso/internal/assets/miso.png" alt="miso" width="200"/>

[![npm](https://img.shields.io/npm/v/@ekkolyth/miso)](https://www.npmjs.com/package/@ekkolyth/miso)
[![license](https://img.shields.io/npm/l/@ekkolyth/miso)](LICENSE)

</div>

Miso doesn't care where you work or what tools you use. It sits as a light wrapper on top of bun, pnpm, and more, that let's you remember one set of commands.

## Get Started

#### Recommended: Install with curl

```bash
curl -fsSL https://misojs.dev/install | bash
```

Supported platforms: macOS (Intel & Apple Silicon), Linux (x86-64 & ARM64).
Windows: Use the npm install method below, or download a binary from the [GitHub Releases](https://github.com/ekkolyth/miso/releases) page.

#### Install via package manager

```bash
npm install -g @ekkolyth/miso
```

#### Upgrading

```bash
miso upgrade
```

Then run `miso init` to add miso to your project:

```bash
miso init
```

### Example miso.json

```json
{
    "$schema": "https://misojs.dev/miso.schema.json",
    "scripts": "./scripts",
    "shell": "bash",
    "tui": {
        "mode": "tabbed",
        "cleanExit": true
    },
    "repo": {
        "tasks": {
            "dev": {
                "concurrent": ["convex"]
            },
            "build": {
                "dependsOn": ["^build"]
            }
        }
    },
    "flags": {
        "add": ["--dev"],
        "remove": [],
        "install": ["--frozen-lockfile"],
        "dev": ["--env"]
    },
    "env": [
        {
            "scope": "global",
            "path": ".env.local",
            "override": ".env.infra",
            "required": "all",
            "variables": {
                "PORT": "port",
                "DATABASE_URL": "url",
                "API_KEY": {
                    "type": "string",
                    "min": 1,
                    "max": 32
                },
                "NODE_ENV": {
                    "type": "enum",
                    "values": ["development", "production", "test"],
                    "optional": true
                },
                "REDIS_URL": {
                    "type": "url",
                    "schemes": ["redis", "rediss"]
                },
                "RATE_LIMIT": {
                    "type": "int+",
                    "min": 1,
                    "max": 1000
                },
                "FEATURE_FLAGS": "json",
                "USER_ID": "uuid",
                "ADMIN_EMAIL": "email",
                "DEBUG_MODE": "bool",
                "SLUG": {
                    "type": "pattern",
                    "pattern": "^[a-z0-9-]+$"
                }
            }
        }
    ]
}
```

## Simple Mode

Don't use JavaScript? Miso also works as a standalone script runner. Set `"packageManager": false` in your `miso.json` and use miso for script execution, TUI, env validation, and task orchestration in any project.

```json
{
    "$schema": "https://misojs.dev/miso.schema.json",
    "packageManager": false,
    "scripts": "./scripts"
}
```

### Contributions welcome!
