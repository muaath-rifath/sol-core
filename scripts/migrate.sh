#!/usr/bin/env bash
set -euo pipefail

if [[ $# -eq 0 ]]; then
  set -- up
fi

exec docker compose run --rm migrate "$@"
