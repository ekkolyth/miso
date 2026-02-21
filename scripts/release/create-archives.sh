#!/usr/bin/env bash
# Create tar and zip archives for GitHub Release
# Usage: VERSION=0.3.2 ./scripts/release/create-archives.sh

set -e

VERSION="${VERSION:?VERSION is required}"

cd dist
tar -czf miso_${VERSION}_darwin_amd64.tar.gz miso-darwin-amd64
tar -czf miso_${VERSION}_darwin_arm64.tar.gz miso-darwin-arm64
tar -czf miso_${VERSION}_linux_amd64.tar.gz miso-linux-amd64
tar -czf miso_${VERSION}_linux_arm64.tar.gz miso-linux-arm64
zip miso_${VERSION}_windows_amd64.zip miso-windows-amd64.exe
tar -czf misox_${VERSION}_darwin_amd64.tar.gz misox-darwin-amd64
tar -czf misox_${VERSION}_darwin_arm64.tar.gz misox-darwin-arm64
tar -czf misox_${VERSION}_linux_amd64.tar.gz misox-linux-amd64
tar -czf misox_${VERSION}_linux_arm64.tar.gz misox-linux-arm64
zip misox_${VERSION}_windows_amd64.zip misox-windows-amd64.exe
