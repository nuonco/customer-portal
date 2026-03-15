#!/bin/bash
set -e
cd "$(dirname "$0")/.."
go build -o ./tmp/main .
exec ./tmp/main
