#!/usr/bin/env bash
# Connect to an already-open, user-authorized Chrome. Does not launch Chrome,
# modify a profile, install extensions, or read login credentials.
set -euo pipefail
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
: "${TREND_BROWSER_ATTACHED_USER_ID:?Set the authorized backend numeric user ID}"
if [[ ! "$TREND_BROWSER_ATTACHED_USER_ID" =~ ^[1-9][0-9]*$ ]]; then
  echo 'Attached user ID must be a positive integer.' >&2; exit 1
fi
export TREND_BROWSER_ENABLED=true
export TREND_BROWSER_MODE=attach
export TREND_BROWSER_ENDPOINT="${TREND_BROWSER_ENDPOINT:-ws://127.0.0.1:9222/devtools/browser}"
export TREND_BROWSER_MAX_SESSIONS=1
cd "$PROJECT_ROOT/backend"
if [[ -n "${TREND_BACKEND_BINARY:-}" ]]; then
  if [[ "$TREND_BACKEND_BINARY" != /* || ! -x "$TREND_BACKEND_BINARY" ]]; then
    echo 'TREND_BACKEND_BINARY must be an executable absolute path.' >&2; exit 1
  fi
  exec "$TREND_BACKEND_BINARY"
fi
exec go run ./cmd/server
