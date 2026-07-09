#!/usr/bin/env bash
# Create tar and zip archives for GitHub Release
# Usage: VERSION=0.3.2 ./scripts/release/create-archives.sh

set -e

VERSION="${VERSION:?VERSION is required}"

cd dist
tar -czf miso_${VERSION}_darwin_amd64.tar.gz -C bin miso-darwin-amd64
tar -czf miso_${VERSION}_darwin_arm64.tar.gz -C bin miso-darwin-arm64
tar -czf miso_${VERSION}_linux_amd64.tar.gz -C bin miso-linux-amd64
tar -czf miso_${VERSION}_linux_arm64.tar.gz -C bin miso-linux-arm64
tar -czf misox_${VERSION}_darwin_amd64.tar.gz -C bin misox-darwin-amd64
tar -czf misox_${VERSION}_darwin_arm64.tar.gz -C bin misox-darwin-arm64
tar -czf misox_${VERSION}_linux_amd64.tar.gz -C bin misox-linux-amd64
tar -czf misox_${VERSION}_linux_arm64.tar.gz -C bin misox-linux-arm64
# Emit sha256 checksums for all archives (consumed by the homebrew bump job,
# and attached to the GitHub Release).
if command -v sha256sum >/dev/null 2>&1; then
    sha256sum ./*.tar.gz > checksums.txt
else
    shasum -a 256 ./*.tar.gz > checksums.txt
fi
echo "✓ wrote dist/checksums.txt"
