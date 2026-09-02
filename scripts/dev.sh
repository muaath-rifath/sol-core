#!/bin/bash
set -a
source "$(dirname "$0")/../.env"
set +a

exec go -C "$(dirname "$0")/../apps/core" run ./cmd/sol
