#!/usr/bin/env bash
# Local monthly-trends backend; leaves the existing browser-attached service alone.
set -euo pipefail
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
monthly_port="${MONTHLY_BACKEND_PORT:-8086}"
if [[ ! "$monthly_port" =~ ^[0-9]{4,5}$ ]] || (( 10#$monthly_port < 1024 || 10#$monthly_port > 65535 )); then
  echo 'MONTHLY_BACKEND_PORT must be a local port from 1024 to 65535.' >&2
  exit 1
fi
export SERVER_ADDR="127.0.0.1:$monthly_port"
cd "$PROJECT_ROOT/backend"
exec go run ./cmd/server
