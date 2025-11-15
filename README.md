# Meet Miso – the agnostic package manager

<img src="./internal/assets/miso.png" alt="miso" width="200"/>

Miso is a Charm-powered CLI that auto-detects your JavaScript package manager, generates a `miso.json`, and lets you define project-specific scripts — all from a single friendly command.

## Why Miso?

- 🔍 Detects bun, npm, pnpm, or yarn lockfiles automatically.
- 🪄 Creates a configurable `miso.json` so you can override the detected manager whenever you need.
- 🧩 Supports a `scripts` map (similar to `package.json`) so `miso dev`, `miso lint`, or any command you invent can run exactly what you expect.
- 🫧 Uses the Charm stack (Bubble Tea, Bubbles, Huh, Lip Gloss, Log) for onboarding prompts and output.

## Supported commands

- `miso install`
- `miso add <pkg>`
- `miso remove <pkg>`
- `miso dev [-- <args>]`
- `miso run <script> [-- <args>]`
- `miso <custom-script> [-- <args>]`

Scripts defined in `miso.json` always win. For example, adding `"dev": "bun run dev --hot"` to your config means `miso dev` will execute that shell command instead of delegating to a package manager.

## `miso.json`

When you run Miso in a directory without a lockfile, you'll be guided through a Charm onboarding flow:

1. Confirm whether this is a brand-new project.
2. Pick the package manager you want to use.
3. (If new) Give the project a name.

The answers are saved to `miso.json`, which looks like this:

```json
{
  "packageManager": "pnpm",
  "projectName": "miso-demo",
  "scripts": {
    "dev": "pnpm dev --host",
    "lint": "pnpm eslint ."
  }
}
```

You can edit this file at any time to switch managers or add more scripts. If a lockfile *is* present, Miso will still generate the config automatically so you have something to customize later.

## Notes

- Use `MISO_DEBUG=1` for additional logging (resolved manager, etc.).
- Manager commands and custom scripts run via `/bin/sh -c`, so feel free to use pipes, env vars, and other shell features.

## Contributions welcome!

Want to add another package manager, improve the UI, or hook into the config flow? Open an issue or PR — we'd love to jam with you.
