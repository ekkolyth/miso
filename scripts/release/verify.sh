#!/usr/bin/env bash
# Verify required files exist in dist/ before npm publish
# Usage: ./scripts/release/verify.sh (run from repo root)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$REPO_ROOT/dist"

echo "Verifying required files exist in dist/..."
echo "Files in dist/:"
ls -la
echo ""

echo "Checking required files:"
[ -f "README.md" ] || { echo "ERROR: README.md is missing!"; exit 1; }
echo "✓ README.md exists"
[ -f "miso.mjs" ] || { echo "ERROR: miso.mjs is missing!"; exit 1; }
echo "✓ miso.mjs exists"
[ -f "misox.mjs" ] || { echo "ERROR: misox.mjs is missing!"; exit 1; }
echo "✓ misox.mjs exists"
[ -f "package.json" ] || { echo "ERROR: package.json is missing!"; exit 1; }
echo "✓ package.json exists"

echo ""
echo "Checking bin paths in package.json:"
BIN_MISO=$(node -p "require('./package.json').bin.miso")
BIN_MISOX=$(node -p "require('./package.json').bin.misox")
echo "bin.miso: $BIN_MISO"
echo "bin.misox: $BIN_MISOX"
[ -f "$BIN_MISO" ] || { echo "ERROR: Bin file $BIN_MISO does not exist!"; exit 1; }
echo "✓ $BIN_MISO exists"
[ -f "$BIN_MISOX" ] || { echo "ERROR: Bin file $BIN_MISOX does not exist!"; exit 1; }
echo "✓ $BIN_MISOX exists"
echo ""
echo "All files verified successfully!"
