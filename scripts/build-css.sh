#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$ROOT_DIR"

# Always copy source assets (fast, idempotent)
./scripts/copy-assets.sh

MARKER="static/css/vendor.css"

# If marker doesn't exist, always rebuild
if [[ -f "$MARKER" ]]; then
    # Check if any source files are newer than the built CSS
    NEWER=$(find src/*.css internal/views -name '*.templ' -o -name '*.css' 2>/dev/null | while read -r f; do
        if [[ "$f" -nt "$MARKER" ]]; then
            echo "changed"
            break
        fi
    done)

    if [[ -z "$NEWER" ]]; then
        echo "CSS sources unchanged, skipping Tailwind build"
        exit 0
    fi
fi

echo "Building CSS..."

go tool gotailwind -i ./src/vendor.css -o ./static/css/vendor.css --content './internal/views/vendorui/**/*.templ' --content './src/base.css'
go tool gotailwind -i ./src/customer.css -o ./static/css/customer.css --content './internal/views/customerui/**/*.templ' --content './src/base.css'

# Hash assets after CSS is built
./scripts/hash-assets.sh
