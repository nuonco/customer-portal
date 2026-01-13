#!/bin/bash
set -e

# Hash CSS assets and generate manifest.json
# This script creates content-hashed copies of CSS files for cache-busting

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
STATIC_DIR="${ROOT_DIR}/static"
CSS_DIR="${STATIC_DIR}/css"
MANIFEST_FILE="${STATIC_DIR}/manifest.json"

# Remove old hashed files (files with 8-char hex pattern before .css)
find "$CSS_DIR" -name "*.*.css" -type f | while read -r file; do
    filename=$(basename "$file")
    # Match pattern like vendor.a1b2c3d4.css
    if [[ "$filename" =~ ^[a-z]+\.[a-f0-9]{8}\.css$ ]]; then
        rm "$file"
    fi
done

# Initialize manifest
echo "{" > "$MANIFEST_FILE"

first=true

# Process each base CSS file (not already hashed)
for css_file in "$CSS_DIR"/*.css; do
    if [[ -f "$css_file" ]]; then
        filename=$(basename "$css_file")

        # Skip already-hashed files (contain 8-char hex pattern before extension)
        if [[ "$filename" =~ \.[a-f0-9]{8}\.css$ ]]; then
            continue
        fi

        # Compute MD5 hash (first 8 chars)
        if [[ "$(uname)" == "Darwin" ]]; then
            hash=$(md5 -q "$css_file" | cut -c1-8)
        else
            hash=$(md5sum "$css_file" | cut -c1-8)
        fi

        # Create hashed filename
        base="${filename%.css}"
        hashed_filename="${base}.${hash}.css"

        # Copy file with hashed name
        cp "$css_file" "${CSS_DIR}/${hashed_filename}"

        # Add to manifest
        if [ "$first" = true ]; then
            first=false
        else
            echo "," >> "$MANIFEST_FILE"
        fi

        printf '  "css/%s": "css/%s"' "$filename" "$hashed_filename" >> "$MANIFEST_FILE"
    fi
done

echo "" >> "$MANIFEST_FILE"
echo "}" >> "$MANIFEST_FILE"

echo "Asset manifest generated: $MANIFEST_FILE"
cat "$MANIFEST_FILE"
