#!/usr/bin/env bash
# Kura in-place upgrade orchestrator: preflight -> backup -> build+
# install -> restart -> postflight. Production data requires the explicit
# DOPE_LIVE_OPT_IN=yes guard, matching the rest of the production scripts.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ENV_NAME="${DOPE_ENV:-test}"

case "$ENV_NAME" in
  test) export DOPE_DATA_DIR="${DOPE_DATA_DIR:-$HOME/.dope-test}"
        export DOPE_DAEMON_ADDR="${DOPE_DAEMON_ADDR:-127.0.0.1:19192}" ;;
  prod) export DOPE_DATA_DIR="${DOPE_DATA_DIR:-$HOME/.dope}"
        export DOPE_DAEMON_ADDR="${DOPE_DAEMON_ADDR:-127.0.0.1:19191}" ;;
  *) echo "unsupported DOPE_ENV: $ENV_NAME (test|prod)" >&2; exit 1 ;;
esac

echo "== upgrade preflight (schema/integrity/tenant snapshot) =="
"${ROOT_DIR}/scripts/production/upgrade-preflight.sh"

echo "== backup =="
"${ROOT_DIR}/scripts/production/backup-test-state.sh"

echo "== build + install =="
"${ROOT_DIR}/scripts/install.sh"

echo "== restart daemon =="
pkill -f '(^|/)kura([[:space:]]|$)' 2>/dev/null && sleep 2 || echo "no running daemon found"
BIN_DIR="${DOPE_BIN_DIR:-$HOME/.local/bin}"
nohup env DOPE_ENV="$ENV_NAME" "${BIN_DIR}/kura" >"${DOPE_DATA_DIR}/daemon.log" 2>&1 &
sleep 3

echo "== postflight (health + schema verification) =="
"${ROOT_DIR}/scripts/production/upgrade-postflight.sh"

echo "upgrade complete; rollback path: scripts/production/restore-test-state.sh with the backup above"
