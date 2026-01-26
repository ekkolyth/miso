#!/bin/sh
# Publish current version without bumping
# Usage: publish/current.sh

set -e

DRY_RUN=${DRY_RUN:-0}

# Check current branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" = "dev" ]; then
  echo "Error: Cannot publish from dev branch. Please switch to main or another branch."
  exit 1
fi

# Check working tree is clean
if [ "$(git status --porcelain)" != "" ]; then
  echo "Working tree is not clean. Commit or stash changes before publishing."
  exit 1
fi

# Build version tool if needed
if [ ! -f ".github/release/version/version" ]; then
  cd apps/miso && go build -o ../../.github/release/version/version ../../.github/release/version/main.go
  cd ../..
fi

# Publish current version without bumping
CURRENT_VERSION=$(./.github/release/version/version)
if [ "$DRY_RUN" = "1" ]; then
  echo "DRY RUN: current version is $CURRENT_VERSION"
  echo "DRY RUN: would create tag v$CURRENT_VERSION and push to origin"
else
  echo "Publishing current version $CURRENT_VERSION"
  if git rev-parse "v$CURRENT_VERSION" >/dev/null 2>&1; then
    echo "Tag v$CURRENT_VERSION already exists locally, deleting it"
    git tag -d "v$CURRENT_VERSION"
  fi
  git tag -a "v$CURRENT_VERSION" -m "Release v$CURRENT_VERSION"
  git push origin HEAD
  git push origin "v$CURRENT_VERSION"
fi
