# Publish current version without bumping
# Usage: publish/current.sh

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

# Get current version from package.json
CURRENT_VERSION=$(node -p "require('./apps/miso/package.json').version")

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
