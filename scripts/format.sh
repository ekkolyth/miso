#!/usr/bin/env sh
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Biome format on docs
(cd apps/docs && npx biome format --write .)

# Go fmt on Go app
(cd apps/miso && go fmt ./...)
