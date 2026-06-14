#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$PROJECT_ROOT/scripts/dev-env.sh"

cd "$PROJECT_ROOT/vue-pure-admin"
exec pnpm dev
