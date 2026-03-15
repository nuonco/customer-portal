#!/bin/bash
# Stress test: repeatedly modify all customerui theme .templ files
# to trigger templ watcher batch regeneration.
# Usage: ./scripts/stress-templ.sh [iterations] [delay_seconds]

set -e
cd "$(dirname "$0")/.."

ITERATIONS=${1:-500}
DELAY=${2:-0}
FILES=$(find internal/views/customerui/theme -name '*.templ' -not -name '*_templ.go')
COUNT=$(echo "$FILES" | wc -l | tr -d ' ')

echo "[stress] Found $COUNT theme .templ files"
echo "[stress] Running $ITERATIONS iterations with ${DELAY}s delay"
echo ""

for i in $(seq 1 "$ITERATIONS"); do
    echo "[stress] Iteration $i/$ITERATIONS — adding comment..."
    for f in $FILES; do
        sed -i '' "1s|^|// stress-test-$i\n|" "$f"
    done
    sleep "$DELAY"

    echo "[stress] Iteration $i/$ITERATIONS — removing comment..."
    for f in $FILES; do
        sed -i '' "/^\/\/ stress-test-$i$/d" "$f"
    done
    sleep "$DELAY"

    echo "[stress] Iteration $i/$ITERATIONS done"
    echo ""
done

echo "[stress] Complete. Check /tmp/nuonctl-customer-dashboard for recovery logs."
