#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_BACKEND_DIR="$ROOT_DIR/backend-go"
DIST_DIR="$ROOT_DIR/dist"

if ! command -v go >/dev/null 2>&1; then
  printf 'go is required to build backend binaries\n' >&2
  exit 1
fi

mkdir -p "$DIST_DIR/linux" "$DIST_DIR/mac" "$DIST_DIR/windows"

build_target() {
  local goos="$1"
  local goarch="$2"
  local output="$3"

  printf '[BUILD] %s/%s -> %s\n' "$goos" "$goarch" "$output"
  (
    cd "$GO_BACKEND_DIR"
    GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$output" ./cmd/server
  )
}

build_target linux amd64 "$DIST_DIR/linux/dataguardian-backend-go"
build_target darwin amd64 "$DIST_DIR/mac/dataguardian-backend-go-amd64"
build_target darwin arm64 "$DIST_DIR/mac/dataguardian-backend-go-arm64"
build_target windows amd64 "$DIST_DIR/windows/dataguardian-backend-go.exe"

printf '[BUILD] Go backend binaries are ready under %s\n' "$DIST_DIR"
