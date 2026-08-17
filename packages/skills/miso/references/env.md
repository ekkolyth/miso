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
| `scope` | `string` | **Required on every root entry in miso mode** (see Env Scoping). The target this env applies to — a workspace/member name, or the reserved `"global"`. Omit inside a member's own `<member>/miso.json` (scope implicit by location), and in `turbo`/`nx` mode where the delegate resolves targets. A scope belongs to the root config or the member's, never both. |
| `path` | `string` | Path to the `.env` file, relative to `miso.json` |
| `override` | `string` | Optional. Path to this scope's override file. Layered on top of the baseline values (populated by `-p` when given) by `-o`/`--override` (override wins). |
| `label` | `string` | Optional. Display name used in validation output |
| `required` | `"all" \| "none" \| string[]` | Which variables must be present. `"all"` = all defined variables. Array = specific keys only. |
| `variables` | `object \| string[]` | Variable name → type validator (object), or list of variable names for presence-only checking (array) |

---

## Env Scoping

**`scope` is a miso-mode requirement.** In `miso` mode (the default — miso runs the processes) every root entry **requires** a `scope`. In `turbo`/`nx` mode the delegate resolves targets, so `scope` is **not required** and `"global"` carries no reserved meaning. (miso prepends each `node_modules/.bin` to `PATH` in every mode so scripts resolve local binaries.)

Validation is not mode-dependent: `miso env` and `--env` traverse every workspace member's `miso.json` in every repo mode. Injection follows orchestration — see Validation vs injection below.

In miso mode, every **root** `env` entry declares a `scope` — the target it applies to, or the reserved `"global"`. Scoping keeps one workspace's variables out of another's.

- `"global"` — injected into every target. No workspace/member may be named `global`.
- A **target name** — injected only when running that target (a workspace member, or a root script/task by that name).

Member workspaces can carry their own env in `<member>/miso.json`. Those entries need **no** `scope` key — their scope is implicit by location (the member they live in).

A scope lives in exactly one config. A root entry scoped to a member that also declares its own `env` is a config error:

```
env scope declared in two places — keep it in the root config or the member's, not both
    web — root miso.json and apps/web/miso.json
```

Declare each app's env in one place or the other — the root config when you want every schema in one file, the member's config when the app owns its own:

```json
// root miso.json — "api" declared here; "web" left to the member
"env": [
  { "scope": "global", "path": ".env",                "variables": { "LOG_COLORS": "bool" } },
  { "scope": "api",    "path": "apps/api/.env.local", "variables": { "REDIS_URL": "url" } }
]

// apps/web/miso.json  (scope implicit by location)
"env": [
  { "path": ".env.local", "variables": { "CONVEX_DEPLOYMENT": "string" } }
]
```

### Resolution

For a run target `T`, the injected env is:

```
global entries  ⊕  ( root entries scoped to T  |  T's member-local entries )
```

The second layer comes from whichever config declares `T` — never both, since declaring a scope in two places is an error. Later layers win on key conflict: **target layer > global**. Finally the shell environment gap-fills — exported shell variables always win.

Fan-out (`miso dev` across members) resolves each member's env independently, so root variables never bleed between members.

### Validation vs injection

The two follow different rules — this is the part people get wrong:

- **Validation** (`miso env`, `--env`) covers the **whole repo in every mode**. It reads the root config and every workspace member's `miso.json`, for `miso`, `turbo`, and `nx` alike. A clean exit means every declared variable across the repo is present and well-typed — that's what makes it a usable CI gate. A member config that won't parse **fails** the run rather than being skipped.
- **Injection** follows **orchestration, not repo mode**. Whoever runs the process owns its env:
    - miso mode → miso runs everything, so miso injects.
    - `turbo`/`nx` mode, name owned by the delegate → turbo/nx runs it and owns its env; miso injects nothing.
    - `turbo`/`nx` mode, name claimed by a `repo.tasks.<name>` entry → **miso** spawns those processes, so miso injects the resolved env exactly as in miso mode.

Membership comes from `package.json` workspaces or `pnpm-workspace.yaml`. A `miso.json` in a directory outside that set is not traversed — declare those schemas at the root.

In miso mode a scope names the target its vars inject for — a member, or a root script/task. A scope that matches no known member is not an error (it may name a root script/task); those vars simply inject only when that target runs, and never leak into another target's environment.

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

**Note:** `--env` triggers validation only. Injection happens automatically for every script miso runs, regardless of whether `--env` is passed — including a `repo.tasks` name in a delegated repo. Names the delegate runs get their env from the delegate.

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

- Injection applies wherever **miso runs the process** — miso mode always, and in `turbo`/`nx` mode for any name claimed by a `repo.tasks` entry. Names the delegate runs get their env from the delegate. `node_modules/.bin` is prepended to `PATH` in every mode regardless, so scripts resolve local binaries.
- Shell environment variables take precedence over `.env` file values
- `.env` files only fill in variables not already set in the shell environment
- Each target receives only its resolved scopes (global + whichever config declares that target); root variables never bleed across workspaces. See **Env Scoping** above.

---

## Common Mistakes

- **`"required": true`** — invalid; use `"required": "all"` or `"required": ["KEY1", "KEY2"]`
- **`"type": "enum"` without `values`** — will fail validation; `values` array is required for enum type
- **Expecting `--env` to inject variables** — `--env` only triggers validation; injection is automatic for every script miso runs
- **Assuming `.env` wins over shell** — shell-exported variables always win; `.env` only fills gaps
- **Assuming turbo/nx mode means miso never injects** — it injects for any name a `repo.tasks` entry hands it, since miso spawns those processes. Only the names the delegate actually runs are the delegate's to fill.
- **Assuming a clean `miso env` skips member schemas** — validation traverses every workspace member in every mode; a member `miso.json` that won't parse fails the run rather than being skipped
- **Declaring one scope in both the root and the member config** — a config error; keep each scope in exactly one file
- **Root `env` entry without `scope` in miso mode** — in miso mode every root entry needs `scope` (a target name or `"global"`); a missing or empty scope is a config error (in `turbo`/`nx` mode scope isn't required)
- **Naming a workspace `global`** — `global` is reserved for env scope; no member or target may use it
