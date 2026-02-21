#!/usr/bin/env bash
# Check package info and npm registry status before publish
# Usage: VERSION=0.3.2 ./scripts/release/check-package-info.sh (run from repo root)

set -e

VERSION="${VERSION:?VERSION is required}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$REPO_ROOT/dist"

PACKAGE_NAME=$(node -p "require('./package.json').name")
PACKAGE_VERSION="$VERSION"

echo "Package name: $PACKAGE_NAME"
echo "Package version: $PACKAGE_VERSION"
echo "Checking npm registry..."
npm config get registry
echo "Checking if package exists on npm..."
if npm view "$PACKAGE_NAME" version 2>/dev/null; then
  echo "Package exists on npm"
  echo "Checking if version $PACKAGE_VERSION already exists..."
  npm view "$PACKAGE_NAME@$PACKAGE_VERSION" version 2>/dev/null && echo "Version $PACKAGE_VERSION already published" || echo "Version $PACKAGE_VERSION not found - will publish"
else
  echo "Package does not exist on npm - first time publish"
fi
