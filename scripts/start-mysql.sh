#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ -f "$HOME/.local/share/dev-env.sh" ]]; then
  source "$HOME/.local/share/dev-env.sh"
else
  source "$PROJECT_ROOT/scripts/dev-env.sh"
fi

MYSQL_HOME="$HOME/.local/share/mysql"
mkdir -p "$MYSQL_HOME/run"

mysqld \
  --no-defaults \
  --daemonize \
  --basedir="$HOME/.local/opt/mysql" \
  --datadir="$MYSQL_HOME/data" \
  --socket="/tmp/mysql.sock" \
  --pid-file="$MYSQL_HOME/run/mysql.pid" \
  --log-error="$MYSQL_HOME/run/mysql.err" \
  --port=3306
