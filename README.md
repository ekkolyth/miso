# Meet Miso
the agnostic package manager

<img src="./internal/assets/miso.png" alt="miso" width="200"/>

Miso is a tiny cli tool that let's you stop worrying about which package manager your projects are using. Use the tools you and your team want, without the hassle. 

## Why Miso?

Miso doesn't care where you work or what tools you use. It sits as a light wrapper on top of bun, pnpm, and more, that let's you remember one set of commands. 

## Get Started

1) install miso 
2) run "miso init" to add miso to your project. 

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

You can edit this file at any time to switch managers or add more scripts. If a lockfile *is* present, Miso will still generate the config automatically so you have something to customize later.

## Notes

- Use `MISO_DEBUG=1` for additional logging (resolved manager, etc.)

## Contributions welcome!
