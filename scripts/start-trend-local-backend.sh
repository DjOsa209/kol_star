#!/usr/bin/env bash
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
export TREND_LOCAL_ENABLED=true
export TREND_BROWSER_ENABLED=false
export TREND_BROWSER_DESKTOP_ENABLED=false
cd "$project_root/backend"
if [[ -n "${TREND_BACKEND_BINARY:-}" ]]; then
  [[ "$TREND_BACKEND_BINARY" == /* && -x "$TREND_BACKEND_BINARY" ]] || { echo 'TREND_BACKEND_BINARY must be an executable absolute path.' >&2; exit 1; }
  exec "$TREND_BACKEND_BINARY"
fi
exec go run ./cmd/server
