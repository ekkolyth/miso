#!/usr/bin/env bash
# Print one version's section of apps/miso/CHANGELOG.md, used as the GitHub release body.
# Usage: ./scripts/release/notes.sh 0.7.1
set -euo pipefail

VERSION="${1:?usage: notes.sh <version>}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHANGELOG="$ROOT/apps/miso/CHANGELOG.md"

if [[ ! -f "$CHANGELOG" ]]; then
    echo "::error::$CHANGELOG not found" >&2
    exit 1
fi

# "### Patch Changes" subheadings don't match "^## ", so they stay in the section
NOTES=$(awk -v heading="## $VERSION" '
  $0 == heading { found = 1; next }
  found && /^## / { exit }
  found { print }
' "$CHANGELOG" | sed '/./,$!d')

if [[ -z "${NOTES//[[:space:]]/}" ]]; then
    echo "::error::no changelog section for $VERSION in apps/miso/CHANGELOG.md" >&2
    exit 1
fi

printf '%s\n' "$NOTES"
