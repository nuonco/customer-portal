#!/bin/bash
set -e

# Copy source assets from src/ into static/ for serving.
# static/ is fully build-generated; all source files live under src/.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$ROOT_DIR"

mkdir -p static/fonts static/images/logos static/js static/css

cp -p src/fonts/Phosphor-Bold.woff2     static/fonts/
cp -p src/fonts/Phosphor-Bold.woff      static/fonts/

cp -p src/images/favicon-admin.svg      static/images/
cp -p src/images/favicon.svg            static/images/
cp -p src/images/oss-hero.png           static/images/
cp -p src/images/logos/aws.svg          static/images/logos/
cp -p src/images/logos/azure.svg        static/images/logos/
cp -p src/images/logos/google.svg       static/images/logos/

cp -p src/js/dev-reload.js              static/js/
cp -p src/js/install-detail.js          static/js/
cp -p src/js/sortable.min.js            static/js/

cp -p src/phosphor-bold.css             static/css/
