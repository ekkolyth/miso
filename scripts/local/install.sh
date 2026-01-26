#!/bin/sh
set -e

BINARY=${BINARY:-miso}
GOBIN=$(go env GOBIN || go env GOPATH)/bin

# Build first
./scripts/build/miso.sh

echo "Installing $BINARY to $GOBIN"
cp apps/miso/bin/$BINARY $GOBIN/$BINARY
cp apps/miso/bin/$BINARY $GOBIN/misox
echo "✓ Installed $BINARY and misox to $GOBIN"
