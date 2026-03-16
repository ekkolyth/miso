# Batch Env Validation Errors

## Problem

`miso env` / `miso dev --env` uses fail-fast validation: it returns on the first error per variable, per entry, and across entries. Users must run the command repeatedly to discover each broken variable one at a time.

## Solution

Collect all validation errors across all entries and display them grouped by label in a single run. Successful entries still log their INFO line. The command still exits with an error when any validation fails.

## Output Format

Successful entries log as before:

```
INFO miso: env validation passed label=halo variables=13
```

When failures exist, a grouped error summary is printed after all entries are processed:

```
ERROR miso: env validation failed:
  onboarding:
    - missing required variable: APP__CLIENT_SECRET
    - missing required variable: GOOGLE_MAPS__API_CLIENT_ID
  beacon:
    - variable DATABASE_URL: url scheme must be one of [http https]
```

## Changes

### `validator.go` — `validateVariables()`

- Change return type from `error` to `[]error`.
- Replace all early `return err` / `return fmt.Errorf(...)` with `errs = append(errs, ...)`.
- `validateVar()` signature is unchanged (returns a single `error` for one variable). The accumulation happens by capturing its return value in `validateVariables()` instead of returning it.
- Continue iterating through all variables.
- Return accumulated `[]error` (nil if empty).

### `env.go` — `runEntry()`

- Change return type from `error` to `[]error`.
- Path resolution and file loading errors remain fatal for that entry — returned as a single-element `[]error` (can't validate variables if the file can't be loaded).
- Array (presence-only) mode: collect all missing keys into a slice instead of returning on the first one.
- Object mode: pass through the `[]error` from `validateVariables()`.
- Error messages must NOT include the label prefix — the label is provided by the grouping header in the formatted output. Remove the `fmt.Errorf("%s: %w", label, err)` wrapping for validation errors.

### `env.go` — `Run()`

- Collect errors per label using a slice of structs (label + errors) to preserve entry order.
- For entries that pass validation, log INFO as before.
- After all entries are processed, if any errors exist:
  - Format grouped error output (label headers with indented bullet errors).
  - Return a single combined error.
- Discovery mode (no env config) is unchanged.
- Exit behavior unchanged — caller still exits on error.

### Tests

- Update existing tests to verify multiple errors are returned per entry.
- Add test for grouped output across multiple entries.
- Test that successful entries still log INFO even when other entries fail.
- Test that an entry can have both missing required variables and type validation failures in the same run.
