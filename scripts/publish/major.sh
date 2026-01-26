#!/bin/sh
# Publish with major version bump
# Usage: publish/major.sh [MESSAGE...]

set -e

MESSAGE="$*"
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

# Build bump tool if needed
if [ ! -f ".github/release/bump/bump" ]; then
  cd apps/miso && go build -o ../../.github/release/bump/bump ../../.github/release/bump/main.go
  cd ../..
fi

# Bump version and publish
if [ "$DRY_RUN" = "1" ]; then
  NEXT_VERSION=$(./.github/release/bump/bump -level=major -dry-run)
  if [ -n "$MESSAGE" ]; then
    echo "DRY RUN: next version would be $NEXT_VERSION"
    echo "DRY RUN: would create tag v$NEXT_VERSION with message: release v$NEXT_VERSION: $MESSAGE"
  else
    echo "DRY RUN: next version would be $NEXT_VERSION"
    echo "DRY RUN: would create tag v$NEXT_VERSION and push to origin"
  fi
else
  NEXT_VERSION=$(./.github/release/bump/bump -level=major)
  echo "Bumped version to $NEXT_VERSION"
  git add apps/miso/package.json
  git commit -m "chore: release v$NEXT_VERSION"
  if [ -n "$MESSAGE" ]; then
    git tag -a "v$NEXT_VERSION" -m "release v$NEXT_VERSION: $MESSAGE"
  else
    git tag -a "v$NEXT_VERSION" -m "Release v$NEXT_VERSION"
  fi
  git push origin HEAD
  git push origin "v$NEXT_VERSION"
fi
