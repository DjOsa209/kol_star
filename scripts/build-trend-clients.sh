#!/usr/bin/env bash
# Build first-party clients using the repository's locked Go modules. No installers.
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
destination="$project_root/backend/data/trend-clients"
mkdir -p "$destination"
cd "$project_root/backend"
platforms=(darwin/arm64 darwin/amd64 linux/amd64 windows/amd64)
if [[ $# -gt 0 ]]; then platforms=("$@"); fi
for platform in "${platforms[@]}"; do
  case "$platform" in darwin/arm64|darwin/amd64|linux/amd64|windows/amd64) ;; *) echo 'Unsupported client platform' >&2; exit 1 ;; esac
  target_os="${platform%/*}"; target_arch="${platform#*/}"
  name="kol-trend-collector-$target_os-$target_arch"
  if [[ "$target_os" == windows ]]; then name="$name.exe"; fi
  CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" go build -mod=readonly -trimpath -ldflags='-s -w' -o "$destination/$name.pending" ./cmd/server
  chmod 755 "$destination/$name.pending"
  mv "$destination/$name.pending" "$destination/$name"
  echo "Built $name"
done
