---
"@ekkolyth/miso": patch
---

- run the member's own scripts when miso is started inside a workspace member
- only treat `--env` before the command as miso's, and pass args to every workspace member
- pass turbo the raw args when a turbo task has no root script
- skip missing env files in CI so clean checkouts can run format and lint
- drop the node completion scripts; use `miso completion` instead
