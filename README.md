# Meet Miso

the agnostic package manager

<img src="./internal/assets/miso.png" alt="miso" width="200"/>

Miso is a tiny cli tool that let's you stop worrying about which package manager your projects are using. Use the tools you and your team want, without the hassle.

## Why Miso?

Miso doesn't care where you work or what tools you use. It sits as a light wrapper on top of bun, pnpm, and more, that let's you remember one set of commands.

## Get Started

### Installation

#### Install via Go

If you have Go installed:

```bash
go install github.com/ekkolyth/miso@latest
```

For a specific branch (e.g., `dev`):

```bash
go install github.com/ekkolyth/miso@dev
```

This installs to `$GOPATH/bin` or `$GOBIN` (usually `~/go/bin`). Make sure this directory is in your PATH.

#### Install via npm

From npm registry:

```bash
npm install -g @ekkolyth/miso
```

From git (for latest or specific branch):

```bash
npm install -g git+https://github.com/ekkolyth/miso.git#main
# or for dev branch:
npm install -g git+https://github.com/ekkolyth/miso.git#dev
```

**Troubleshooting npm permission errors:**

If you get `EACCES` permission errors, you have a few options:

1. **Configure npm to use a user directory** (recommended):

   ```bash
   npm config set prefix ~/.npm-global
   echo 'export PATH="$HOME/.npm-global/bin:$PATH"' >> ~/.zshrc  # or ~/.bashrc
   source ~/.zshrc  # or source ~/.bashrc
   ```

2. Use `sudo` (not recommended):
   ```bash
   sudo npm install -g @ekkolyth/miso
   ```

Then run `miso init` to add miso to your project:

```bash
miso init
```

**Note:** If you installed locally, use `npx miso init` or `./node_modules/.bin/miso init`.

## Supported commands

- `miso install`
- `miso add <pkg>`
- `miso remove <pkg>`
- `miso dev [-- <args>]`
- `miso run <script> [-- <args>]`

## `miso.json`

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

You can edit this file at any time to switch managers or add more scripts. If a lockfile _is_ present, Miso will still generate the config automatically so you have something to customize later.

## Notes

- Use `MISO_DEBUG=1` for additional logging (resolved manager, etc.)

## Contributions welcome!
