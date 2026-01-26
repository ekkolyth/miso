#!/bin/sh
set -e

# Build both miso binary and docs
./scripts/build/miso.sh
./scripts/build/docs.sh
