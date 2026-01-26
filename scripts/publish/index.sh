#!/bin/sh
# Default publish (patch level)
# Usage: publish/index.sh [MESSAGE...]

set -e

./scripts/publish/patch.sh "$@"
