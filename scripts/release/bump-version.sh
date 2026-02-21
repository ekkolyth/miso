#!/usr/bin/env bash
# Bump version in apps/miso/package.json (patch, minor, or major)
# Usage: LEVEL=patch ./scripts/release/bump-version.sh
#        LEVEL=minor DRY_RUN=1 ./scripts/release/bump-version.sh
# Outputs the new version. Updates package.json unless DRY_RUN=1.

set -e

LEVEL="${LEVEL:?LEVEL is required (patch, minor, or major)}"
DRY_RUN="${DRY_RUN:-0}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

NEXT_VERSION=$(cd "$REPO_ROOT" && node -e "
const fs = require('fs');
const pkg = JSON.parse(fs.readFileSync('apps/miso/package.json', 'utf8'));
const [major, minor, patch] = pkg.version.split('.').map(Number);
const level = process.env.LEVEL;
let next;
if (level === 'patch') next = major + '.' + minor + '.' + (patch + 1);
else if (level === 'minor') next = major + '.' + (minor + 1) + '.0';
else if (level === 'major') next = (major + 1) + '.0.0';
else throw new Error('LEVEL must be patch, minor, or major');
if (process.env.DRY_RUN === '1') {
  console.log(next);
  process.exit(0);
}
pkg.version = next;
fs.writeFileSync('apps/miso/package.json', JSON.stringify(pkg, null, 2) + '\n');
console.log(next);
")

echo "$NEXT_VERSION"
