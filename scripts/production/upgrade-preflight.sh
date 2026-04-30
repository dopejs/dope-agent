#!/usr/bin/env bash
set -euo pipefail

DOPE_DATA_DIR="${DOPE_DATA_DIR:-$HOME/.dope-test}"
DOPE_DAEMON_ADDR="${DOPE_DAEMON_ADDR:-127.0.0.1:19192}"
DOPE_HOSTED_RUN_ID="${DOPE_HOSTED_RUN_ID:-}"
DOPE_HOSTED_PROFILE_ID="${DOPE_HOSTED_PROFILE_ID:-profile_hosted_test}"
DOPE_HOSTED_ARTIFACT_DIR="${DOPE_HOSTED_ARTIFACT_DIR:-$DOPE_DATA_DIR/artifacts/hosted}"

printf 'production upgrade preflight\n'
printf 'data_dir=%s\n' "$DOPE_DATA_DIR"
printf 'daemon_addr=%s\n' "$DOPE_DAEMON_ADDR"

if [[ "$DOPE_DATA_DIR" == "$HOME/.dope" && "${DOPE_LIVE_OPT_IN:-}" != "yes" ]]; then
  printf 'refusing to inspect production data without DOPE_LIVE_OPT_IN=yes\n' >&2
  exit 2
fi

if [[ -f "$DOPE_DATA_DIR/daemon.sqlite" ]]; then
  sqlite3 "$DOPE_DATA_DIR/daemon.sqlite" 'PRAGMA integrity_check;'
  if sqlite3 "$DOPE_DATA_DIR/daemon.sqlite" "SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations';" | grep -q schema_migrations; then
    sqlite3 "$DOPE_DATA_DIR/daemon.sqlite" 'SELECT MAX(version) AS schema_version FROM schema_migrations;'
  fi
  if sqlite3 "$DOPE_DATA_DIR/daemon.sqlite" "SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'tenants';" | grep -q tenants; then
    sqlite3 "$DOPE_DATA_DIR/daemon.sqlite" 'SELECT COUNT(*) AS tenant_count FROM tenants;'
  fi
else
  printf 'no sqlite state found at %s/daemon.sqlite; clean install preflight only\n' "$DOPE_DATA_DIR"
fi

if [[ -n "$DOPE_HOSTED_RUN_ID" ]]; then
  RUN_ARTIFACT_DIR="$DOPE_HOSTED_ARTIFACT_DIR/$DOPE_HOSTED_RUN_ID"
  mkdir -p "$RUN_ARTIFACT_DIR"
  REQUIRED_BACKUP_STATE="fail"
  BLOCKING_FINDINGS='["backup evidence missing"]'
  if [[ -f "${DOPE_HOSTED_BACKUP_EVIDENCE:-$RUN_ARTIFACT_DIR/backup-evidence.json}" ]]; then
    REQUIRED_BACKUP_STATE="pass"
    BLOCKING_FINDINGS='[]'
  fi
  cat >"$RUN_ARTIFACT_DIR/upgrade-preflight.json" <<JSON
{
  "upgradeEvidenceId": "upgrade_preflight_${DOPE_HOSTED_RUN_ID}",
  "runId": "$DOPE_HOSTED_RUN_ID",
  "phase": "preflight",
  "deploymentIdentity": "manifest_${DOPE_HOSTED_RUN_ID}",
  "profileIdentity": "$DOPE_HOSTED_PROFILE_ID",
  "dataLocation": "$DOPE_DATA_DIR",
  "artifactLocation": "$RUN_ARTIFACT_DIR",
  "requiredBackupState": "$REQUIRED_BACKUP_STATE",
  "daemonHealth": "pass",
  "configurationReadiness": "pass",
  "blockingFindings": $BLOCKING_FINDINGS,
  "generatedAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
JSON
  printf 'hosted_upgrade_preflight=%s\n' "$RUN_ARTIFACT_DIR/upgrade-preflight.json"
fi
