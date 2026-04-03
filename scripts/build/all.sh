#!/usr/bin/env bash
set -e

BINARY=${BINARY:-miso}
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MISO_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)/apps/miso"

cd "$MISO_DIR"
mkdir -p bin
GOOS=darwin GOARCH=amd64 go build -o bin/$BINARY-darwin-amd64 ./cmd
GOOS=darwin GOARCH=amd64 go build -o bin/misox-darwin-amd64 ./cmd
GOOS=darwin GOARCH=arm64 go build -o bin/$BINARY-darwin-arm64 ./cmd
GOOS=darwin GOARCH=arm64 go build -o bin/misox-darwin-arm64 ./cmd
GOOS=linux GOARCH=amd64 go build -o bin/$BINARY-linux-amd64 ./cmd
GOOS=linux GOARCH=amd64 go build -o bin/misox-linux-amd64 ./cmd
GOOS=linux GOARCH=arm64 go build -o bin/$BINARY-linux-arm64 ./cmd
GOOS=linux GOARCH=arm64 go build -o bin/misox-linux-arm64 ./cmd
GOOS=windows GOARCH=amd64 go build -o bin/$BINARY-windows-amd64.exe ./cmd
GOOS=windows GOARCH=amd64 go build -o bin/misox-windows-amd64.exe ./cmd
