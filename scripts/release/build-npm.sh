#!/usr/bin/env bash
# Build Go binaries to dist/bin/ and copy package files for npm publish
# Usage: ./scripts/release/build-npm.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$REPO_ROOT"

mkdir -p dist/bin
cd apps/miso

GOOS=darwin GOARCH=amd64 go build -o ../../dist/bin/miso-darwin-amd64 ./cmd
GOOS=darwin GOARCH=arm64 go build -o ../../dist/bin/miso-darwin-arm64 ./cmd
GOOS=linux GOARCH=amd64 go build -o ../../dist/bin/miso-linux-amd64 ./cmd
GOOS=linux GOARCH=arm64 go build -o ../../dist/bin/miso-linux-arm64 ./cmd
GOOS=windows GOARCH=amd64 go build -o ../../dist/bin/miso-windows-amd64.exe ./cmd
GOOS=darwin GOARCH=amd64 go build -o ../../dist/bin/misox-darwin-amd64 ./cmd
GOOS=darwin GOARCH=arm64 go build -o ../../dist/bin/misox-darwin-arm64 ./cmd
GOOS=linux GOARCH=amd64 go build -o ../../dist/bin/misox-linux-amd64 ./cmd
GOOS=linux GOARCH=arm64 go build -o ../../dist/bin/misox-linux-arm64 ./cmd
GOOS=windows GOARCH=amd64 go build -o ../../dist/bin/misox-windows-amd64.exe ./cmd

cd ../..
cp apps/miso/miso.mjs apps/miso/misox.mjs apps/miso/package.json README.md dist/
