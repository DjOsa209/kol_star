#!/usr/bin/env bash
# Start the existing backend with its per-user browser collector enabled.
# Uses already installed Chrome, Go/backend binary, and (on Linux) Xvfb + xauth.
set -euo pipefail
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
export TREND_BROWSER_ENABLED=true
export TREND_BROWSER_MAX_SESSIONS="${TREND_BROWSER_MAX_SESSIONS:-2}"
export TREND_BROWSER_PROFILE_ROOT="${TREND_BROWSER_PROFILE_ROOT:-$PROJECT_ROOT/.runtime/trend-browser-profiles}"
umask 077
mkdir -p "$TREND_BROWSER_PROFILE_ROOT"
if [[ -L "$TREND_BROWSER_PROFILE_ROOT" ]]; then
  echo 'Browser profile directory must not be a symlink.' >&2
  exit 1
fi
case "$(uname -s)" in
  Darwin)
    export TREND_BROWSER_EXECUTABLE="${TREND_BROWSER_EXECUTABLE:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}"
    ;;
  Linux)
    if [[ "$(id -u)" == 0 ]]; then
      echo 'Run the backend as an unprivileged user; Chrome sandbox must stay enabled.' >&2
      exit 1
    fi
    export TREND_BROWSER_EXECUTABLE="${TREND_BROWSER_EXECUTABLE:-/usr/bin/google-chrome-stable}"
    ;;
  *) echo 'Supported platforms: Linux and macOS.' >&2; exit 1 ;;
esac
if [[ ! -x "$TREND_BROWSER_EXECUTABLE" ]]; then
  echo 'Chrome is not installed at TREND_BROWSER_EXECUTABLE. Provision an approved browser first.' >&2
  exit 1
fi
cd "$PROJECT_ROOT/backend"
if [[ -n "${TREND_BACKEND_BINARY:-}" ]]; then
  if [[ "$TREND_BACKEND_BINARY" != /* || ! -x "$TREND_BACKEND_BINARY" ]]; then
    echo 'TREND_BACKEND_BINARY must be an executable absolute path.' >&2; exit 1
  fi
  backend_command=("$TREND_BACKEND_BINARY")
else
  backend_command=(go run ./cmd/server)
fi
if [[ "$(uname -s)" == Linux && -z "${DISPLAY:-}" ]]; then
  if ! command -v xvfb-run >/dev/null || ! command -v xauth >/dev/null; then
    echo 'Linux requires approved Xvfb and xauth packages. No packages were installed.' >&2; exit 1
  fi
  exec xvfb-run -a -s '-screen 0 1280x800x24 -nolisten tcp' "${backend_command[@]}"
fi
exec "${backend_command[@]}"
