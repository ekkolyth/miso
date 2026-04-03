#!/usr/bin/env bash
# Commit and push version bump to package.json
# Usage: VERSION=0.3.2 ./scripts/release/version-bump.sh

set -e

VERSION="${VERSION:?VERSION is required}"

git config user.name "github-actions[bot]"
git config user.email "github-actions[bot]@users.noreply.github.com"
git add apps/miso/package.json

if git diff --staged --quiet; then
    echo "package.json already at $VERSION - nothing to commit"
    exit 0
fi

git commit -m "chore: bump version to $VERSION [skip ci]"
git push
