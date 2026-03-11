#!/usr/bin/env bash
# Build binaries for all platforms and copy to dist/
# Usage: ./scripts/release/build-release.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$REPO_ROOT"
./scripts/build/all.sh

mkdir -p dist
cp apps/miso/bin/miso-* dist/
cp apps/miso/bin/misox-* dist/
cp apps/miso/miso.mjs apps/miso/misox.mjs dist/
