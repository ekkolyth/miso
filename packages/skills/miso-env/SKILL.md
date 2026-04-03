---
name: miso-env
description: Reference for configuring env validation and understanding miso's .env file loading
---

# miso-env

**Use this skill** when configuring env validation, debugging env injection, or understanding which `.env` files miso loads.

---

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
    "path": ".env.local",
    "label": "Local",
    "required": "all",
    "variables": {
      "DATABASE_URL": "url",
      "PORT": "port"
    }
  },
  {
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
| `path` | `string` | Path to the `.env` file, relative to `miso.json` |
| `label` | `string` | Optional. Display name used in validation output |
| `required` | `"all" \| "none" \| string[]` | Which variables must be present. `"all"` = all defined variables. Array = specific keys only. |
| `variables` | `object \| string[]` | Variable name → type validator (object), or list of variable names for presence-only checking (array) |

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
- In monorepos, each workspace receives env from its own directory only — no cross-workspace variable bleed

---

## Common Mistakes

- **`"required": true`** — invalid; use `"required": "all"` or `"required": ["KEY1", "KEY2"]`
- **`"type": "enum"` without `values`** — will fail validation; `values` array is required for enum type
- **Expecting `--env` to inject variables** — `--env` only triggers validation; injection is automatic for all scripts
- **Assuming `.env` wins over shell** — shell-exported variables always win; `.env` only fills gaps
