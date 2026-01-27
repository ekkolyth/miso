#!/bin/sh
set -e

cd apps/miso && gofmt -w $(go list -f '{{.Dir}}' ./...)
