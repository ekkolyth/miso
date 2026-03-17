# Meet Miso

the agnostic package manager

<img src="https://raw.githubusercontent.com/ekkolyth/miso/main/apps/miso/internal/assets/miso.png" alt="miso" width="200"/>

Miso is a tiny cli tool that let's you stop worrying about which package manager your projects are using. Use the tools you and your team want, without the hassle.

## Why Miso?

Miso doesn't care where you work or what tools you use. It sits as a light wrapper on top of bun, pnpm, and more, that let's you remember one set of commands.

## Simple Mode

Don't use JavaScript? Miso also works as a standalone script runner. Set `"packageManager": false` in your `miso.json` and use miso for script execution, TUI, env validation, and task orchestration in any project.

```json
{
    "$schema": "https://misojs.dev/miso.schema.json",
    "packageManager": false,
    "scripts": "./scripts"
}
```

## Get Started

### Installation

#### Install via npm

Global Install (recommended)

```bash
npm install -g @ekkolyth/miso
```

Then run `miso init` to add miso to your project:

```bash
miso init
```

### Example miso.json (Kitchen Sink)

```json
{
    "$schema": "https://misojs.dev/miso.schema.json",
    "package-manager": "bun",
    "name": "my-project",
    "scripts": "./scripts",
    "shell": "bash",
    "flags": {
        "add": ["--dev"],
        "remove": [],
        "install": ["--frozen-lockfile"],
        "dev": ["--env"]
    },
    "env": {
        "path": [".env.local", ".env"],
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
}
```

### Contributions welcome!
