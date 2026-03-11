#!/usr/bin/env bash
# Build Go binaries to dist/bin/ and copy package files for npm publish
# Usage: ./scripts/release/build-npm.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(realpath "$SCRIPT_DIR/../../..")"

cd "$REPO_ROOT"

mkdir -p dist/bin
cd apps/miso

CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o ../../dist/bin/miso-darwin-amd64 ./cmd
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o ../../dist/bin/miso-darwin-arm64 ./cmd
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../../dist/bin/miso-linux-amd64 ./cmd
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o ../../dist/bin/miso-linux-arm64 ./cmd
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o ../../dist/bin/miso-windows-amd64.exe ./cmd
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o ../../dist/bin/misox-darwin-amd64 ./cmd
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o ../../dist/bin/misox-darwin-arm64 ./cmd
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../../dist/bin/misox-linux-amd64 ./cmd
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o ../../dist/bin/misox-linux-arm64 ./cmd
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o ../../dist/bin/misox-windows-amd64.exe ./cmd

chmod +x \
  ../../dist/bin/miso-darwin-amd64 \
  ../../dist/bin/miso-darwin-arm64 \
  ../../dist/bin/miso-linux-amd64 \
  ../../dist/bin/miso-linux-arm64 \
  ../../dist/bin/misox-darwin-amd64 \
  ../../dist/bin/misox-darwin-arm64 \
  ../../dist/bin/misox-linux-amd64 \
  ../../dist/bin/misox-linux-arm64

cd ../..
cp apps/miso/miso.mjs apps/miso/misox.mjs apps/miso/package.json README.md dist/

# sync schema to docs public folder so misojs.dev always serves the latest
cp apps/miso/miso.schema.json apps/docs/public/miso.schema.json
echo "✓ schema synced to apps/docs/public/miso.schema.json"
