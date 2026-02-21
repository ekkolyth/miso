#!/usr/bin/env bash
# Parse semantic version from commit message (e.g. v0.3.2 or 0.3.2 at start)
# Writes version to GITHUB_OUTPUT when run in GitHub Actions.
# Usage: COMMIT_MSG="v0.3.2 - fixes" ./scripts/release/parse-version.sh

MSG="${COMMIT_MSG:-}"

if [[ "$MSG" =~ ^v?([0-9]+\.[0-9]+\.[0-9]+) ]]; then
  echo "version=${BASH_REMATCH[1]}" >> "$GITHUB_OUTPUT"
  echo "Version: ${BASH_REMATCH[1]}"
else
  echo "version=" >> "$GITHUB_OUTPUT"
  echo "No version in commit message - skipping release"
fi
