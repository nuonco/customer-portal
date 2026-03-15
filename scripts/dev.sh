#!/bin/bash
set -e
cd "$(dirname "$0")/.."

kill_port_processes() {
    sleep 0.5
    lsof -ti :7331 | xargs kill 2>/dev/null || true
    lsof -ti :8080 | xargs kill 2>/dev/null || true
    sleep 0.5
    lsof -ti :7331 | xargs kill -9 2>/dev/null || true
    lsof -ti :8080 | xargs kill -9 2>/dev/null || true
}

cleanup() {
    echo "[dev] Shutting down..."
    pkill -P $$ 2>/dev/null || true
    kill_port_processes
}
trap cleanup EXIT
mkdir -p ./tmp
export LIVE_RELOAD=true

# Use a stable project-local directory for templ dev mode txt files.
# macOS periodically cleans /var/folders/.../T/ (the default os.TempDir()),
# which deletes templ's watched string files and crashes live reload.
export TEMPL_DEV_MODE_ROOT="$(pwd)/tmp/templ-dev"
rm -rf "$TEMPL_DEV_MODE_ROOT"
mkdir -p "$TEMPL_DEV_MODE_ROOT"

echo "[dev] Cleaning up stale processes..."
kill_port_processes

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
while true; do
    go tool templ generate --watch \
        --cmd "./scripts/run-server.sh" \
        --proxy="http://localhost:7331" \
        --proxyport=8080 \
        --proxybind="0.0.0.0" \
        --open-browser=false || true
    echo "[dev] templ proxy exited or stuck, cleaning up..."
    kill_port_processes
    echo "[dev] Restarting templ watch..."
done
