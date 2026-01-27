#!/bin/sh
set -e

# Build first
./scripts/build/miso.sh

# Run with remaining arguments
cd apps/miso && go run ./cmd "$@"
