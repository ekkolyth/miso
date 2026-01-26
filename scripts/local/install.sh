#!/bin/sh
set -e

BINARY=${BINARY:-miso}
GOBIN=/Users/mikekenway/go/bin

# Build first
./scripts/build/miso.sh

echo "Installing $BINARY to $GOBIN"
cp apps/miso/bin/$BINARY $GOBIN/$BINARY
cp apps/miso/bin/$BINARY $GOBIN/misox
echo "✓ Installed $BINARY and misox to $GOBIN"
