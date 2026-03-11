#!/usr/bin/env bash
set -e

BINARY=${BINARY:-miso}
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$REPO_ROOT"

echo "→ building miso binaries..."
cd apps/miso
mkdir -p bin
GOOS=darwin  GOARCH=amd64 go build -o bin/$BINARY-darwin-amd64   ./cmd
GOOS=darwin  GOARCH=arm64 go build -o bin/$BINARY-darwin-arm64   ./cmd
GOOS=linux   GOARCH=amd64 go build -o bin/$BINARY-linux-amd64    ./cmd
GOOS=linux   GOARCH=arm64 go build -o bin/$BINARY-linux-arm64    ./cmd
GOOS=windows GOARCH=amd64 go build -o bin/$BINARY-windows-amd64.exe ./cmd
GOOS=darwin  GOARCH=amd64 go build -o bin/misox-darwin-amd64     ./cmd
GOOS=darwin  GOARCH=arm64 go build -o bin/misox-darwin-arm64     ./cmd
GOOS=linux   GOARCH=amd64 go build -o bin/misox-linux-amd64      ./cmd
GOOS=linux   GOARCH=arm64 go build -o bin/misox-linux-arm64      ./cmd
GOOS=windows GOARCH=amd64 go build -o bin/misox-windows-amd64.exe ./cmd
echo "✓ miso binaries built"

cd "$REPO_ROOT"

echo "→ building docs..."
cd apps/docs && bun run build
echo "✓ docs built"
