#!/bin/bash
# Build script for Go cimbar web server

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "=== Building CIMBAR Go Server ==="

# Step 1: Build all C++ libraries (required for dependencies)
echo "Step 1: Building C++ libraries..."
cmake -DBUILD_CGO=1 .
make -j$(nproc)

# Step 2: Build Go server
echo "Step 2: Building Go server..."
cd cmd/cimbar-server
go build -o ../../build-cgo/bin/cimbar-server
cd ../..

echo ""
echo "=== Build Complete ==="
echo "Binary: build-cgo/bin/cimbar-server"
echo ""
echo "Usage:"
echo "  ./build-cgo/bin/cimbar-server --addr :8080 --output-dir /tmp/cimbar"
