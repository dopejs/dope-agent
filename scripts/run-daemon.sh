#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
DAEMON_DIR="${ROOT_DIR}/daemon"
ENV_NAME="${1:-test}"

case "${ENV_NAME}" in
  test)
    export DOPE_ENV=test
    export DOPE_CONNECTORS_DISCORD_ENABLED="${DOPE_CONNECTORS_DISCORD_ENABLED:-false}"
    ;;
  prod)
    export DOPE_ENV=prod
    ;;
  *)
    echo "usage: $0 [test|prod]" >&2
    exit 1
    ;;
esac

cd "${DAEMON_DIR}"
exec go run ./cmd/dope
