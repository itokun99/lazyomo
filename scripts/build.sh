#!/bin/bash
set -e

echo "Building lazyomo..."

go build -o lazyomo ./cmd/lazyomo

echo "Build complete: ./lazyomo"
