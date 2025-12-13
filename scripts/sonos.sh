#!/usr/bin/env bash
set -euo pipefail

pnpm -s build >/dev/null

# pnpm passes an extra "--" delimiter through to scripts.
if [[ "${1:-}" == "--" ]]; then
  shift
fi

exec ./bin/sonos "$@"
