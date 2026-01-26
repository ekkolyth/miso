#!/bin/sh
set -e

BINARY=${BINARY:-miso}
GOBIN=$(go env GOBIN || go env GOPATH)/bin

rm -f $GOBIN/$BINARY
rm -f $GOBIN/misox
