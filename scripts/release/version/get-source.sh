#!/usr/bin/env bash
# Get the text to parse for version (PR title for merge commits, else commit message).
# Writes to GITHUB_OUTPUT when run in GitHub Actions.
# Usage: COMMIT_MSG="Merge pull request #7 from ..." ./scripts/release/get-version-source.sh

MSG="${COMMIT_MSG:-}"

if [[ "$MSG" =~ [Mm]erge\ pull\ request\ \#([0-9]+) ]]; then
    PR_NUM="${BASH_REMATCH[1]}"
    TITLE=$(gh pr view "$PR_NUM" --json title -q .title 2>/dev/null || true)
    TEXT="${TITLE:-$MSG}"
else
    TEXT="$MSG"
fi

{
    echo "text<<GITHUB_ACTIONS_DELIMITER"
    echo "$TEXT"
    echo "GITHUB_ACTIONS_DELIMITER"
} >> "$GITHUB_OUTPUT"
