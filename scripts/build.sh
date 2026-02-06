#!/bin/bash
set -e

# Build CSS with isolated content paths
# - vendor.css scans vendorui templates + base.css
# - customer.css scans customerui templates + base.css

./bin/tailwindcss -i ./src/vendor.css -o ./static/css/vendor.css \
  --content './internal/views/vendorui/**/*.templ' \
  --content './src/base.css'

./bin/tailwindcss -i ./src/customer.css -o ./static/css/customer.css \
  --content './internal/views/customerui/**/*.templ' \
  --content './src/base.css'

# Generate content-hashed CSS files and manifest for cache-busting
./scripts/hash-assets.sh

# Regenerate templ and rebuild Go
templ generate ./internal/views/...
go build -o ./tmp/main .
