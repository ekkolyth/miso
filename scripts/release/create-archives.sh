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
zip -j miso_${VERSION}_windows_amd64.zip bin/miso-windows-amd64.exe
tar -czf misox_${VERSION}_darwin_amd64.tar.gz -C bin misox-darwin-amd64
tar -czf misox_${VERSION}_darwin_arm64.tar.gz -C bin misox-darwin-arm64
tar -czf misox_${VERSION}_linux_amd64.tar.gz -C bin misox-linux-amd64
tar -czf misox_${VERSION}_linux_arm64.tar.gz -C bin misox-linux-arm64
zip -j misox_${VERSION}_windows_amd64.zip bin/misox-windows-amd64.exe
