# Using Miso

This guide explains how to interact with Miso in your projects. Learn how to set up Miso, run commands from anywhere in your project, and understand what to do when things go wrong.

## Running Commands from Anywhere

You can run Miso commands from any subdirectory within your project. Miso automatically finds your project root by looking for `miso.json`, lockfiles, or `node_modules` in parent directories.


## Setting Up a Project (`miso init`)

Run `miso init` to set up Miso in your project. This creates a `miso.json` configuration file.

### What Happens

1. **Project name** - You'll be prompted to name your project (defaults to your directory name)
2. **Package manager selection** - Choose your package manager from the list (bun, npm, pnpm, or yarn)
3. **Configuration created** - Miso creates `miso.json` in the current directory
4. **Package manager init** - If `package.json` doesn't exist, Miso runs your selected package manager's `init` command

## Working with Monorepos

Miso works great in monorepo structures. You can run commands from any subdirectory, and Miso will find your project configuration at the root.

### Example

```
my-monorepo/
├── miso.json
├── package.json
└── packages/
    └── frontend/
        └── src/
            └── components/
```

If you're in `packages/frontend/src/components/` and run `miso add react`:

- Miso finds your `miso.json` at the monorepo root
- The package manager command runs from your current directory (`packages/frontend/src/components/`)
- This means packages are added to the correct location in your monorepo structure

**Key point**: Keep one `miso.json` at your monorepo root, and run Miso commands from anywhere in your project structure.

## Error Messages

Miso provides clear error messages to help you understand what went wrong and how to fix it.

### "miso is not currently initialized in this project"

**When you see this**: Miso found a lockfile or `node_modules` folder, but no `miso.json` exists.

**What it means**: Your project has dependencies installed, but Miso hasn't been set up yet.

**How to fix**: Run `miso init` from the project root (where the lockfile or `node_modules` is located).

### "no project found. miso was not able to locate a miso.json, node_modules folder, or lockfile in the parent tree"

**When you see this**: Miso walked up to the filesystem root without finding any project indicators.

**What it means**: You're either not in a project directory, or the project hasn't been initialized with a package manager.

**How to fix**: 
- Navigate to your project directory
- If starting a new project, run `miso init` to set up both Miso and your package manager
- If in an existing project, ensure you're in the correct directory

### "package manager not configured in miso.json"

**When you see this**: Your `miso.json` exists but doesn't have a `package-manager` field.

**What it means**: The configuration file is incomplete or corrupted.

**How to fix**: Edit `miso.json` and add a `package-manager` field, or delete `miso.json` and run `miso init` again.

### "unsupported package manager 'X' specified in package.json. supported managers: [bun, npm, pnpm, yarn]"

**When you see this**: Your `package.json` has a `packageManager` field with an unsupported manager.

**What it means**: Miso doesn't support the package manager specified in your `package.json`.

**How to fix**: Either switch to a supported manager, or manually edit `miso.json` after initialization to use a different manager.

### "miso.json already exists"

**When you see this**: You tried to run `miso init` but `miso.json` already exists.

**What it means**: Your project is already initialized with Miso.

**How to fix**: If you want to reinitialize, delete `miso.json` first, or edit it directly to change settings.

## Editing Your Configuration

You can edit `miso.json` directly to change settings:

- **Switch package managers** - Change the `package-manager` field
- **Add or modify scripts** - Edit the `scripts` object
- **Update project name** - Change the `project-name` field

No need to run `miso init` again after editing the file.

## Best Practices

1. **Run `miso init` from the project root** - This ensures `miso.json` is created in the correct location
2. **Keep `miso.json` at the project root** - Even in monorepos, maintain a single `miso.json` at the root
3. **Edit `miso.json` directly** - Make changes to the file as needed; no reinitialization required
