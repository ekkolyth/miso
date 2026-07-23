#!/usr/bin/env bash
# Stamp the release version into the miso config schema and emit two copies:
#   - apps/miso/miso.schema.json                    (source; $id = latest URL)
#   - apps/docs/public/v<VERSION>/miso.schema.json  (pinned; $id = versioned URL)
# The docs site serves public/ at misojs.dev, so the pinned copy is committed and
# persists — letting a miso.json lock "$schema" to a specific version instead of
# always-latest. Single source of truth: VERSION (same value as package.json).
# Usage: VERSION=0.6.0 ./scripts/release/schema/render.sh
set -euo pipefail

VERSION="${VERSION:?VERSION required}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

node -e '
  const fs = require("fs");
  const path = require("path");
  const [root, version] = process.argv.slice(1);
  const base = "https://misojs.dev";
  const src = path.join(root, "apps/miso/miso.schema.json");

  const schema = JSON.parse(fs.readFileSync(src, "utf8"));
  schema.version = version;               // assigning an existing key keeps its position
  schema.$id = base + "/miso.schema.json";
  fs.writeFileSync(src, JSON.stringify(schema, null, "\t") + "\n");

  const pinned = { ...schema, $id: `${base}/v${version}/miso.schema.json` };
  const dir = path.join(root, "apps/docs/public", "v" + version);
  fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(path.join(dir, "miso.schema.json"), JSON.stringify(pinned, null, "\t") + "\n");
' "$ROOT" "$VERSION"

echo "✓ stamped schema v$VERSION (latest source + pinned public/v$VERSION)"
