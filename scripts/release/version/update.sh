#!/usr/bin/env bash
# Consume the pending changesets: bump apps/miso/package.json, write apps/miso/CHANGELOG.md,
# delete the consumed changesets, then stamp the schema.
# Usage: VERSION=0.3.2 ./scripts/release/version/update.sh (run from repo root)

set -euo pipefail

VERSION="${VERSION:?VERSION is required}"

bunx --no-install changeset version

# resolve.sh predicted VERSION; the commit must not disagree with what gets tagged
BUMPED=$(node -p "require('./apps/miso/package.json').version")
if [[ "$BUMPED" != "$VERSION" ]]; then
    echo "::error::changeset version produced $BUMPED, expected $VERSION" >&2
    exit 1
fi
echo "Updated apps/miso/package.json to version $VERSION"

# Stamp the same version into the schema: source (latest) + pinned public copy.
VERSION="$VERSION" "$(dirname "$0")/../schema/render.sh"
