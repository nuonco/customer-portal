#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR/ui"

if [[ ! -d node_modules ]]; then
  echo "Installing client dependencies with bun..."
  bun install
fi

exec bun run dev:all
