# miso-config

**Use this skill** any time you need to create or modify `miso.json`.

---

## `miso.json` Field Reference

### `$schema`
**Type:** `string`
**Default:** not set

IDE autocomplete URL. Always use:
```json
"$schema": "https://misojs.dev/miso.schema.json"
```

---

### `packageManager`
**Type:** `boolean`
**Default:** `true`

`true` — miso wraps your package manager (npm, bun, pnpm, yarn). Detects lockfile automatically.
`false` — simple mode. No package manager. `miso run <script>` resolves only from the `scripts/` folder. See `miso-scripting` for script conventions and `miso-env` for env injection.

---

### `scripts`
**Type:** `string`
**Default:** `"./scripts"`

Path to the folder containing your runnable scripts. Can be relative or absolute.

```json
"scripts": "./tasks"
```

---

### `shell`
**Type:** `string`
**Default:** `"sh"`

Fallback interpreter used when a script has no shebang and no recognized extension.

```json
"shell": "bash"
```

---

### `repo`
**Type:** `"single" | "mono" | "turbo" | "nx" | object`
**Default:** `"single"`

Controls monorepo orchestration:
- `"single"` — one project, no workspace awareness
- `"mono"` — miso-native orchestration; requires `workspaces` array in root `package.json`
- `"turbo"` — delegates to Turborepo; parses output into TUI tabs
- `"nx"` — delegates to Nx (`nx run-many --target=<script>`); parses output into TUI tabs
- Object form: `{ "mode": "turbo", "tasks": { ... } }` — use when you need `tasks` config alongside delegation

---

### `tui`
**Type:** `"off" | "tabbed" | "merged" | object`
**Default:** `"off"`

Multi-process TUI display mode:
- `"off"` — standard output, no TUI
- `"tabbed"` — sidebar with per-process log panes, arrow key navigation, `r` to restart
- `"merged"` — interleaved output with color-coded process labels and filter bar
- Object form: `{ "mode": "tabbed", "cleanExit": true }`

See `miso-tui` for full TUI configuration including concurrent tasks and task ordering.

---

### `flags`
**Type:** `object` — keys are command names, values are string arrays
**Default:** `{}`

Persistent flags injected into specific commands. Useful for CI or project-wide defaults.

```json
"flags": {
  "install": ["--frozen-lockfile"],
  "dev": ["--turbo"]
}
```

---

### `env`
**Type:** `object | array`
**Default:** not set (auto-discovery mode)

Env file paths and validation rules. When not set, miso auto-discovers `.env.local` → `.env.production` → `.env.development` → `.env` (first found per variable).

See `miso-env` for full env configuration including variable types and validation.

---

## Annotated Examples

### Minimal config (simple mode project)

```json
{
  "$schema": "https://misojs.dev/miso.schema.json",
  "packageManager": false
}
```

### Full config (monorepo with TUI and env validation)

```json
{
  "$schema": "https://misojs.dev/miso.schema.json",
  "packageManager": true,
  "scripts": "./scripts",
  "shell": "bash",
  "repo": {
    "mode": "turbo",
    "tasks": {
      "dev": {
        "concurrent": ["studio"]
      }
    }
  },
  "tui": "tabbed",
  "flags": {
    "install": ["--frozen-lockfile"]
  },
  "env": [
    {
      "path": ".env.local",
      "required": "all",
      "variables": {
        "DATABASE_URL": "url",
        "PORT": "port",
        "NODE_ENV": { "type": "enum", "values": ["development", "production", "test"] }
      }
    }
  ]
}
```

---

## Common Mistakes

- **`"package-manager"` (hyphenated)** — the field is `"packageManager"` (camelCase)
- **`repo: "mono"` without `workspaces`** — miso reads `workspaces` from root `package.json` to discover workspace directories; if it's missing, mono mode won't find any workspaces
- **`tui` inside `repo`** — `tui` is a top-level field, not nested under `repo`
- **String flags** — `"flags": { "install": "--frozen-lockfile" }` is wrong; must be an array: `["--frozen-lockfile"]`
