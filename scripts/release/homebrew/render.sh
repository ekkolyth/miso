#!/usr/bin/env bash
# Render Formula/miso.rb from the template using dist/checksums.txt.
# Usage: VERSION=0.5.3 ./scripts/release/homebrew/render.sh > miso.rb
set -euo pipefail

VERSION="${VERSION:?VERSION required}"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$DIR/../../.." && pwd)"
CHECKSUMS="$ROOT/dist/checksums.txt"

sha_for() { grep -E "miso_${VERSION}_$1\.tar\.gz\$" "$CHECKSUMS" | awk '{print $1}'; }

D_ARM=$(sha_for darwin_arm64)
D_AMD=$(sha_for darwin_amd64)
L_ARM=$(sha_for linux_arm64)
L_AMD=$(sha_for linux_amd64)

for pair in "D_ARM:$D_ARM" "D_AMD:$D_AMD" "L_ARM:$L_ARM" "L_AMD:$L_AMD"; do
    [[ -n "${pair#*:}" ]] || { echo "::error::missing checksum for ${pair%%:*}" >&2; exit 1; }
done

sed \
    -e "s|__VERSION__|$VERSION|g" \
    -e "s|__SHA_DARWIN_ARM64__|$D_ARM|g" \
    -e "s|__SHA_DARWIN_AMD64__|$D_AMD|g" \
    -e "s|__SHA_LINUX_ARM64__|$L_ARM|g" \
    -e "s|__SHA_LINUX_AMD64__|$L_AMD|g" \
    "$DIR/formula.rb.tmpl"
