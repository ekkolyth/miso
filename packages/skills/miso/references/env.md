# Env validation

## Auto-Discovery (No `env` Config)

When no `env` block is set in `miso.json`, miso auto-discovers `.env` files in this order:

```
.env.local → .env.production → .env.development → .env
```

First found per variable wins. Shell-exported variables always take precedence over `.env` file values.

---

## `env` Config Block

### Single entry object

```json
"env": {
  "scope": "global",
  "path": ".env.local",
  "required": "all",
  "variables": {
    "DATABASE_URL": "url",
    "PORT": "port"
  }
}
```

### Array of entries (multiple files)

```json
"env": [
  {
    "scope": "global",
    "path": ".env.local",
    "label": "Local",
    "required": "all",
    "variables": {
      "DATABASE_URL": "url",
      "PORT": "port"
    }
  },
  {
    "scope": "api",
    "path": ".env.secrets",
    "required": ["API_KEY"],
    "variables": {
      "API_KEY": "string"
    }
  }
]
```

### `variables` — presence-only array form

When you only need to verify that variables exist (no type checking), use an array of names:

```json
"env": {
  "scope": "global",
  "path": ".env.local",
  "required": "all",
  "variables": ["DATABASE_URL", "API_KEY", "PORT"]
}
```

This checks that each named variable is present in the env file but does not validate its format or type.

---

## Entry Fields

| Field | Type | Description |
|---|---|---|
| `scope` | `string` | **Required on every root entry.** The target this env applies to — a workspace/member name, or the reserved `"global"`. Omit only inside a member's own `<member>/miso.json`, where scope is implicit by location. |
| `path` | `string` | Path to the `.env` file, relative to `miso.json` |
| `label` | `string` | Optional. Display name used in validation output |
| `required` | `"all" \| "none" \| string[]` | Which variables must be present. `"all"` = all defined variables. Array = specific keys only. |
| `variables` | `object \| string[]` | Variable name → type validator (object), or list of variable names for presence-only checking (array) |

---

## Env Scoping

Every **root** `env` entry declares a `scope` — the target it applies to, or the reserved `"global"`. Scoping keeps one workspace's variables out of another's.

- `"global"` — injected into every target. No workspace/member may be named `global`.
- A **target name** — injected only when running that target (a workspace member, or a root script/task by that name).

Member workspaces can carry their own env in `<member>/miso.json`. Those entries need **no** `scope` key — their scope is implicit by location (the member they live in).

```json
// root miso.json
"env": [
  { "scope": "global", "path": ".env",               "variables": { "LOG_COLORS": "bool" } },
  { "scope": "web",    "path": "apps/web/.env.local", "variables": { "CONVEX_DEPLOYMENT": "string" } }
]

// apps/web/miso.json  (scope implicit by location)
"env": [
  { "path": ".env.local", "variables": { "CONVEX_DEPLOYMENT": "string" } }
]
```

### Resolution

For a run target `T`, the injected env is:

```
global entries  ⊕  root entries scoped to T  ⊕  T's member-local entries (if T is a member)
```

Later layers win on key conflict: **member-local > target-scoped > global**. Finally the shell environment gap-fills — exported shell variables always win.

Fan-out (`miso dev` across members) resolves each member's env independently, so root variables never bleed between members.

### Validation vs injection

- `miso env` (and `--env`) **validates every scope** — global, every target-scoped, and every member-local entry — so a pre-build validation gate stays whole.
- **Injection** uses only the resolved subset above.

A scope that names no known member isn't a hard error (it may name a root script or task) — miso surfaces it as a warning.

---

## Variable Type Validators

### Shorthand

```json
"PORT": "port"
```

Equivalent to `"PORT": { "type": "port" }`.

### All Types

| Type | Validates |
|---|---|
| `"string"` | Any string. Optional `min`/`max` for length. |
| `"port"` | Integer 1–65535 |
| `"int"` | Integer. Optional `min`/`max`. |
| `"int+"` | Positive integer (> 0). Optional `min`/`max`. |
| `"float"` | Floating point number |
| `"bool"` | `"true"` or `"false"` (string) |
| `"url"` | Valid URL. Optional `schemes` array (e.g. `["redis", "rediss"]`). |
| `"enum"` | Must be one of `values`. Requires `values` array. |
| `"email"` | Valid email address |
| `"json"` | Valid JSON string |
| `"uuid"` | Valid UUID v4 |
| `"pattern"` | Matches regex. Requires `pattern` string. |

### `bool` — custom truthy/falsy strings

By default, `bool` accepts:
- True: `"true"`, `"1"`, `"yes"`, `"on"`
- False: `"false"`, `"0"`, `"no"`, `"off"`

Override with `trueValues` and `falseValues`:

```json
"ENABLE_FEATURE": {
  "type": "bool",
  "trueValues": ["enabled", "yes"],
  "falseValues": ["disabled", "no"]
}
```

### Examples

```json
"variables": {
  "PORT": "port",
  "NODE_ENV": { "type": "enum", "values": ["development", "production", "test"] },
  "DATABASE_URL": { "type": "url", "schemes": ["postgres", "postgresql"] },
  "DESCRIPTION": { "type": "string", "min": 1, "max": 255 },
  "RETRY_COUNT": { "type": "int+", "max": 10 },
  "FEATURE_FLAG": "bool",
  "ENABLE_FEATURE": { "type": "bool", "trueValues": ["enabled"], "falseValues": ["disabled"] },
  "API_SECRET": { "type": "string", "optional": true }
}
```

Add `"optional": true` to any variable config to allow it to be absent without failing validation.

---

## `--env` Flag

Triggers env validation **before** script execution. The flag is stripped before args reach the script:

```bash
miso dev --env        # validates env, then runs dev
miso build --env      # validates env, then runs build
```

Run `miso env` to validate without executing any script.

**Note:** `--env` triggers validation only. Env file injection happens automatically for all scripts regardless of whether `--env` is passed.

---

## Injection Behavior

- Shell environment variables take precedence over `.env` file values
- `.env` files only fill in variables not already set in the shell environment
- Each target receives only its resolved scopes (global + its target-scoped + its member-local); root variables never bleed across workspaces. See **Env Scoping** above.

---

## Common Mistakes

- **`"required": true`** — invalid; use `"required": "all"` or `"required": ["KEY1", "KEY2"]`
- **`"type": "enum"` without `values`** — will fail validation; `values` array is required for enum type
- **Expecting `--env` to inject variables** — `--env` only triggers validation; injection is automatic for all scripts
- **Assuming `.env` wins over shell** — shell-exported variables always win; `.env` only fills gaps
- **Root `env` entry without `scope`** — every root entry needs `scope` (a target name or `"global"`); a missing or empty scope is a config error
- **Naming a workspace `global`** — `global` is reserved for env scope; no member or target may use it
