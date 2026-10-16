#!/usr/bin/env bash
# Resolve the next release version from the pending changesets.
# An empty version means nothing is pending, and the release jobs skip.
# Usage: GITHUB_OUTPUT=<file> ./scripts/release/version/resolve.sh (run from repo root)
set -euo pipefail

OUTPUT="${GITHUB_OUTPUT:?GITHUB_OUTPUT required}"
STATUS_FILE="$(mktemp)"
trap 'rm -f "$STATUS_FILE"' EXIT

# a status failure (bad config, missing CLI) must fail the job, not read as "nothing pending"
bunx --no-install changeset status --output="$STATUS_FILE"

VERSION=$(node -e '
  const status = JSON.parse(require("fs").readFileSync(process.argv[1], "utf8"));
  const release = status.releases.find((r) => r.name === "@ekkolyth/miso");
  process.stdout.write(release ? release.newVersion : "");
' "$STATUS_FILE")

echo "version=$VERSION" >> "$OUTPUT"
if [[ -z "$VERSION" ]]; then
    echo "No pending changesets - skipping release"
else
    echo "Resolved pending changesets: $VERSION"
fi
