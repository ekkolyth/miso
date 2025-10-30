# Meet Miso

<img src="./internal/assets/miso.png" alt="miso" width="200"/>

A tiny CLI that routes shared JS package-manager commands to the right tool.

## Supported managers

- bun
- npm
- pnpm
- yarn

## Supported commands

- `miso install`
- `miso add <pkg>`
- `miso remove <pkg>`
- `miso dev <args>`
- `miso run <script> <args>`

## Notes

- Error if no lockfile or multiple lockfiles are present.
- Set `MISO_DEBUG=1` to print the detected manager.

## Contributions Welcome!

I would love to see what others could/might add! I'm sure there are tons of bugs, I'm brand new to this, so don't hesitate to leave issues or reach out!
