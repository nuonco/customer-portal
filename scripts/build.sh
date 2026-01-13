#!/bin/bash
set -e

# Always rebuild CSS (Tailwind scans templ files for class names)
./bin/tailwindcss -i ./src/vendor.css -o ./static/css/vendor.css
./bin/tailwindcss -i ./src/customer.css -o ./static/css/customer.css

# Generate content-hashed CSS files and manifest for cache-busting
./scripts/hash-assets.sh

# Regenerate templ and rebuild Go
templ generate ./internal/views/...
go build -o ./tmp/main .
