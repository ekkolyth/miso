#!/usr/bin/env bash
# Resolve the next release version from the merged PR's release:* label.
# Finds the PR for the pushed merge commit, reads its release:{patch|minor|major}
# label, and computes the next semver from the latest git tag.
# Fails (non-zero) when no release:* label is present.
set -euo pipefail

SHA="${GITHUB_SHA:?GITHUB_SHA required}"

PR_NUM=$(gh pr list --state merged --search "$SHA" --json number -q '.[0].number' 2>/dev/null || true)
if [[ -z "$PR_NUM" ]]; then
    echo "::error::no merged PR found for commit $SHA" >&2
    exit 1
fi

LABELS=$(gh pr view "$PR_NUM" --json labels -q '.labels[].name')
LEVEL=""
for l in $LABELS; do
    case "$l" in
        release:patch) LEVEL="patch" ;;
        release:minor) LEVEL="minor" ;;
        release:major) LEVEL="major" ;;
    esac
done

if [[ -z "$LEVEL" ]]; then
    echo "::error::PR #$PR_NUM has no release:{patch|minor|major} label — refusing to release" >&2
    exit 1
fi

LATEST=$(git describe --tags --match 'v[0-9]*.[0-9]*.[0-9]*' --abbrev=0 2>/dev/null || echo "v0.0.0")
LATEST="${LATEST#v}"
IFS='.' read -r MAJOR MINOR PATCH <<< "$LATEST"

case "$LEVEL" in
    major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
    minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
    patch) PATCH=$((PATCH + 1)) ;;
esac

VERSION="${MAJOR}.${MINOR}.${PATCH}"
TITLE=$(gh pr view "$PR_NUM" --json title -q '.title')

echo "version=$VERSION" >> "$GITHUB_OUTPUT"
# random delimiter so a PR title can't close the heredoc early
DELIM="EOF_NOTES_$(openssl rand -hex 8)"
{
    echo "notes<<$DELIM"
    echo "$TITLE"
    echo "$DELIM"
} >> "$GITHUB_OUTPUT"
echo "Resolved $LEVEL bump from PR #$PR_NUM: $LATEST -> $VERSION"
