#!/usr/bin/env bash
set -euo pipefail

DOPE_DATA_DIR="${DOPE_DATA_DIR:-$HOME/.dope-test}"
DOPE_DAEMON_ADDR="${DOPE_DAEMON_ADDR:-127.0.0.1:19192}"

printf 'production upgrade postflight\n'
printf 'data_dir=%s\n' "$DOPE_DATA_DIR"
printf 'daemon_addr=%s\n' "$DOPE_DAEMON_ADDR"

if [[ "$DOPE_DATA_DIR" == "$HOME/.dope" && "${DOPE_LIVE_OPT_IN:-}" != "yes" ]]; then
  printf 'refusing to inspect production data without DOPE_LIVE_OPT_IN=yes\n' >&2
  exit 2
fi

scripts/check-daemon-health.sh "$DOPE_DAEMON_ADDR"
if [[ -f "$DOPE_DATA_DIR/daemon.sqlite" ]]; then
  sqlite3 "$DOPE_DATA_DIR/daemon.sqlite" 'PRAGMA integrity_check;'
  if sqlite3 "$DOPE_DATA_DIR/daemon.sqlite" "SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'tenants';" | grep -q tenants; then
    sqlite3 "$DOPE_DATA_DIR/daemon.sqlite" 'SELECT COUNT(*) AS tenant_count FROM tenants;'
  fi
  if sqlite3 "$DOPE_DATA_DIR/daemon.sqlite" "SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'r39_tenants';" | grep -q r39_tenants; then
    sqlite3 "$DOPE_DATA_DIR/daemon.sqlite" 'SELECT COUNT(*) AS r39_fixture_tenant_count FROM r39_tenants;'
  fi
fi
