#!/bin/sh
set -e

BINARY=${BINARY:-miso}
GOBIN=/Users/mikekenway/go/bin

rm -f $GOBIN/$BINARY
rm -f $GOBIN/misox
