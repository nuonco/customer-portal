#!/usr/bin/env bash
set -euo pipefail

# Single local-dev entry for customer-dashboard, mirroring dashboard-ui's
# scripts/dev.sh: run the Go server and the React/Vite client together under
# one process group so nuonctl shows a single collapsible entry and a restart
# tears down every child cleanly.
#
# The Go binary itself is built by local_build_cmds in service.yml (into
# ./tmp/main); nuonctl's file watcher rebuilds it and restarts this script on
# .go / .templ changes. The ui/ directory is in watch_ignore because Vite owns
# its own HMR.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

# Kill the whole process group on exit so the Go server, Vite, CSS watch, and
# Ladle all go down together when nuonctl sends SIGTERM.
DEV_PGID=$(ps -o pgid= -p $$ | tr -d ' ')
cleanup() {
    kill -TERM -- "-$DEV_PGID" 2>/dev/null || true
    wait 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# Go backend: Templ/HTMX app + JSON /bff endpoints on :8080
echo "Starting Go server (./tmp/main)..."
./tmp/main &

# TypeScript frontend: Vite dev server (:51273), CSS watch, and Ladle (:61001).
# dev:all runs all three in parallel via bun. Kept in the foreground (like
# dashboard-ui's dev.sh) so this works on macOS's bash 3.2 without `wait -n`;
# when it exits, the trap tears down the Go server too and nuonctl retries.
cd ui
if [[ ! -d node_modules ]]; then
  echo "Installing client dependencies with bun..."
  bun install
fi
echo "Starting React client (bun run dev:all)..."
bun run dev:all
