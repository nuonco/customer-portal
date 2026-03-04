#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$ROOT_DIR"

mkdir -p ./tmp

echo "Running templ generate..."
go tool templ generate ./...

echo "Running CSS build..."
./scripts/build-css.sh

echo "Building Go binary..."
go build -o ./tmp/main .

echo "Build complete."
