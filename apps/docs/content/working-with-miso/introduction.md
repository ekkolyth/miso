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

## Editing Your Configuration

You can edit `miso.json` directly to change settings:

- **Switch package managers** - Change the `package-manager` field
- **Change scripts folder path** - Update the `scripts` field (defaults to `./scripts`)
- **Update project name** - Change the `project-name` field

No need to run `miso init` again after editing the file.
