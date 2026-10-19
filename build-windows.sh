#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="${SCRIPT_DIR}"
OUTPUT_BIN="${SCRIPT_DIR}/skynet.exe"

echo "Starting cross-compile (Linux → Windows)..."

if [ ! -d "$PROJECT_DIR" ]; then
    echo "Error: Directory '$PROJECT_DIR' not found!"
    exit 1
fi

cd "$PROJECT_DIR"

# Formatting
echo "Formatting..."
go fmt ./...

# Cross-compile for Windows
echo "Cross-compiling for Windows..."
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
    go build -trimpath -ldflags="-H windowsgui -s -w -buildid=" -o "$OUTPUT_BIN" .

echo "Build successful!"
echo "Binary location: $OUTPUT_BIN"
