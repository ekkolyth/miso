# @ekkolyth/miso

## 0.8.2

### Patch Changes

- 0863b31: fix `miso skills --add` and `--rm` failing for Claude Code and Gemini CLI by passing the agent ids the skills CLI accepts (`claude-code`, `gemini-cli`)
- 0863b31: redraw `dotnet run` and other MSBuild build progress in place in the `miso dev` panes instead of stacking a blank and a `(0.5s)` row on every tick

## 0.8.1

### Patch Changes

- 56fb1ea: document the 0.8.0 member, package manager, and init changes in the migration guide

## 0.8.0

### Minor Changes

- cd0e51f: tui updates, standlone mode overhaul

## 0.7.2

### Patch Changes

- 3a44ab4: - run the member's own scripts when miso is started inside a workspace member
  - only treat `--env` before the command as miso's, and pass args to every workspace member
  - pass turbo the raw args when a turbo task has no root script
  - skip missing env files in CI so clean checkouts can run format and lint
  - drop the node completion scripts; use `miso completion` instead

## 0.7.1

### Patch Changes

- dc018a4: bug fixes
