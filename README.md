# Meet Miso

the agnostic package manager

<img src="./internal/assets/miso.png" alt="miso" width="200"/>

Miso is a tiny cli tool that let's you stop worrying about which package manager your projects are using. Use the tools you and your team want, without the hassle.

## Why Miso?

Miso doesn't care where you work or what tools you use. It sits as a light wrapper on top of bun, pnpm, and more, that let's you remember one set of commands.

## Get Started

### Installation

**Recommended: Install via Go** (no permission issues):

```bash
go install github.com/ekkolyth/miso@dev
```

**Alternative: Install via npm**:

From npm registry (if published):

```bash
npm install -g @ekkolyth/miso
```

From git (for dev branch or latest):

```bash
npm install -g git+https://github.com/ekkolyth/miso.git#dev
```

If you get permission errors with npm, you can either:

- Use `sudo npm install -g @ekkolyth/miso` (not recommended)
- Configure npm to use a different directory: `npm config set prefix ~/.npm-global` and add `~/.npm-global/bin` to your PATH

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
