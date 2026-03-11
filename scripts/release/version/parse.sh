#!/usr/bin/env bash
# Parse semantic version from text (PR title, commit message, etc.)
# Looks for x.y.z anywhere in the string (e.g. "Release v0.3.2" or "v0.3.2 - fixes")
# Writes version to GITHUB_OUTPUT when run in GitHub Actions.
# Usage: COMMIT_MSG="Release v0.3.2" ./scripts/release/parse-version.sh

MSG="${COMMIT_MSG:-}"

if [[ "$MSG" =~ ([0-9]+\.[0-9]+\.[0-9]+) ]]; then
    echo "version=${BASH_REMATCH[1]}" >> "$GITHUB_OUTPUT"
    echo "Version: ${BASH_REMATCH[1]}"
else
    echo "version=" >> "$GITHUB_OUTPUT"
    echo "No version found - skipping release"
fi
