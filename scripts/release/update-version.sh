#!/usr/bin/env bash
# Update version in apps/miso/package.json
# Usage: VERSION=0.3.2 ./scripts/release/update-version.sh

set -e

VERSION="${VERSION:?VERSION is required}"

node -e "
const fs = require('fs');
const pkg = JSON.parse(fs.readFileSync('apps/miso/package.json', 'utf8'));
pkg.version = process.env.VERSION;
fs.writeFileSync('apps/miso/package.json', JSON.stringify(pkg, null, 2) + '\n');
"
echo "Updated apps/miso/package.json to version $VERSION"
