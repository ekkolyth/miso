#!/bin/sh
set -e

cd apps/miso && go test ./...
