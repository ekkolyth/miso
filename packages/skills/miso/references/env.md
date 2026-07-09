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
| `scope` | `string` | **Required on every root entry in miso mode** (see Env Scoping). The target this env applies to — a workspace/member name, or the reserved `"global"`. Omit inside a member's own `<member>/miso.json` (scope implicit by location), and in `turbo`/`nx` mode where miso doesn't inject and scope isn't used. |
| `path` | `string` | Path to the `.env` file, relative to `miso.json` |
| `override` | `string` | Optional. Path to this scope's override file. Layered on top of the baseline values (populated by `-p` when given) by `-o`/`--override` (override wins). |
| `label` | `string` | Optional. Display name used in validation output |
| `required` | `"all" \| "none" \| string[]` | Which variables must be present. `"all"` = all defined variables. Array = specific keys only. |
| `variables` | `object \| string[]` | Variable name → type validator (object), or list of variable names for presence-only checking (array) |

---

## Env Scoping

Scoping and env **injection are a miso-mode feature.** In `miso` mode (the default — miso runs the processes) miso injects each target's env and **requires** a `scope` on every root entry. In `turbo`/`nx` mode the delegate owns env for the processes it orchestrates, so miso injects nothing and `scope` is **not required** — `env` is then a pure validation surface. (miso still prepends each `node_modules/.bin` to `PATH` in every mode so scripts resolve local binaries.)

In miso mode, every **root** `env` entry declares a `scope` — the target it applies to, or the reserved `"global"`. Scoping keeps one workspace's variables out of another's.

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

- `miso env` (and `--env`) **validates** the declared entries in every mode — a pre-build gate that works for `miso`, `turbo`, and `nx` repos alike. In miso mode it also validates every scope (global, target-scoped, and member-local).
- **Injection** happens only in **miso mode**, using the resolved subset above. In `turbo`/`nx` mode miso injects nothing — the delegate owns env.

In miso mode, a scope that names no known member isn't a hard error (it may name a root script or task) — miso surfaces it as a warning.

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

**Note:** `--env` triggers validation only. In **miso mode**, env injection happens automatically for every script regardless of whether `--env` is passed; in `turbo`/`nx` mode miso injects nothing (the delegate owns env).

---

## Generating templates

`miso env` flags (combinable):

- `-g` / `--generate` — write `.env.generated` per scope, keys only (from each scope's declared `variables`).
- `-p` / `--populate` — fill values from each entry's `path` baseline.
- `-o` / `--override` — layer each entry's `override` file on top (override wins).
- `-x` / `--example` — write `.env.example` per scope with type-valid fake values (valid port/url/uuid/…). Passes `miso env` validation — use it as a committable example or to rule env out as a crash cause. Prompts before overwriting an existing file. Can't combine with `-p`/`-o`.

File location follows the scope: `global` → repo root; `<member>` → that member's dir. With `-p`/`-o` the resolved values are validated (type + required) and nothing is written for any scope if one fails.

Entry field `override`: path to that scope's override file (root/global override, or one per scope).

---

## Injection Behavior

- Injection applies in **miso mode** only. In `turbo`/`nx` mode miso injects no env values (the delegate owns env); it still prepends each `node_modules/.bin` to `PATH` so scripts resolve local binaries.
- Shell environment variables take precedence over `.env` file values
- `.env` files only fill in variables not already set in the shell environment
- Each target receives only its resolved scopes (global + its target-scoped + its member-local); root variables never bleed across workspaces. See **Env Scoping** above.

---

## Common Mistakes

- **`"required": true`** — invalid; use `"required": "all"` or `"required": ["KEY1", "KEY2"]`
- **`"type": "enum"` without `values`** — will fail validation; `values` array is required for enum type
- **Expecting `--env` to inject variables** — `--env` only triggers validation; in miso mode injection is automatic for every script (in `turbo`/`nx` mode the delegate owns env)
- **Assuming `.env` wins over shell** — shell-exported variables always win; `.env` only fills gaps
- **Expecting miso to inject env in turbo/nx mode** — it doesn't; the delegate owns env there, so `scope` isn't used and entries validate only
- **Root `env` entry without `scope` in miso mode** — in miso mode every root entry needs `scope` (a target name or `"global"`); a missing or empty scope is a config error (in `turbo`/`nx` mode scope isn't required)
- **Naming a workspace `global`** — `global` is reserved for env scope; no member or target may use it
