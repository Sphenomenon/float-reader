#!/usr/bin/env bash
set -euo pipefail
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"
export GOCACHE="${GOCACHE:-/tmp/floatreader-go-cache}"
mkdir -p dist
python3 scripts/resources.py
go test ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet -unsafeptr=false ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags '-H windowsgui -s -w' -o dist/FloatReader.exe ./cmd/floatreader
python3 scripts/package.py
