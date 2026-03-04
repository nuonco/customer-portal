#!/bin/bash
set -e
cd "$(dirname "$0")/.."
mkdir -p ./tmp
export LIVE_RELOAD=true

echo "[dev] Cleaning up stale processes..."
lsof -ti :7331 | xargs kill 2>/dev/null || true
lsof -ti :8080 | xargs kill 2>/dev/null || true
sleep 0.5

echo "[dev] Initial build..."
go tool templ generate --lazy
./scripts/build-css.sh
go build -o ./tmp/main .

echo "[dev] Starting CSS watcher..."
(
    if command -v fswatch &>/dev/null; then
        fswatch -o -e '.*' -i '\.css$' src/ | while read -r; do
            echo "[dev] CSS source changed, rebuilding..."
            ./scripts/build-css.sh 2>&1
        done
    else
        while true; do sleep 2; ./scripts/build-css.sh 2>&1 || true; done
    fi
) &

echo "[dev] Starting templ watch + proxy on :8080..."
exec go tool templ generate --watch \
    --cmd "go run ." \
    --proxy="http://localhost:7331" \
    --proxyport=8080 \
    --proxybind="0.0.0.0" \
    --open-browser=false
