#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
version=$(cat VERSION)
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Invalid VERSION" >&2; exit 1; }
if [[ $# -ne 1 || "$1" != "v$version" ]]; then
  echo "Usage: $0 v$version (must match VERSION)" >&2
  exit 1
fi
# Refuse to replace previous build artifacts.
output="dist/release-$version"
mkdir -p dist
mkdir "$output"
for arch in amd64 arm64; do
  bundle="stackchan-mcp_${version}_linux_${arch}"
  mkdir "$output/$bundle"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -ldflags="-s -w -X stackchan-mcp/internal/app.Version=$version" -o "$output/$bundle/stackchan-mcp" ./cmd/stackchan-mcp
  cp README.md VERSION "$output/$bundle/"

  tar --sort=name --mtime="@$(git log -1 --format=%ct)" --owner=0 --group=0 --numeric-owner -C "$output/$bundle" -czf "$output/$bundle.tar.gz" .
  rm -r "$output/$bundle"
done
(cd "$output" && sha256sum *.tar.gz > checksums.txt)
