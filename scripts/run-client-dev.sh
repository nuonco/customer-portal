#!/usr/bin/env bash
set -euo pipefail

# Runs only the client-side watch build (Vite into ui/dist, CSS watch, Ladle).
# The bundle is served by the Go server on :8080, so this alone does not make the
# app reachable — use scripts/dev.sh (or nuonctl dev) for that.
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR/ui"

if [[ ! -d node_modules ]]; then
  echo "Installing client dependencies with bun..."
  bun install
fi

exec bun run dev:all
