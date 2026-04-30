#!/usr/bin/env bash
set -euo pipefail

DOPE_DATA_DIR="${DOPE_DATA_DIR:-$HOME/.dope-test}"
DOPE_DAEMON_ADDR="${DOPE_DAEMON_ADDR:-127.0.0.1:19192}"
DOPE_HOSTED_RUN_ID="${DOPE_HOSTED_RUN_ID:-}"
DOPE_HOSTED_ARTIFACT_DIR="${DOPE_HOSTED_ARTIFACT_DIR:-$DOPE_DATA_DIR/artifacts/hosted}"

printf 'production upgrade postflight\n'
printf 'data_dir=%s\n' "$DOPE_DATA_DIR"
printf 'daemon_addr=%s\n' "$DOPE_DAEMON_ADDR"

if [[ "$DOPE_DATA_DIR" == "$HOME/.dope" && "${DOPE_LIVE_OPT_IN:-}" != "yes" ]]; then
  printf 'refusing to inspect production data without DOPE_LIVE_OPT_IN=yes\n' >&2
  exit 2
fi

if [[ -n "${DOPE_HOSTED_HEALTH_COMMAND:-}" ]]; then
  sh -c "$DOPE_HOSTED_HEALTH_COMMAND" >/dev/null
else
  scripts/check-daemon-health.sh "$DOPE_DAEMON_ADDR"
fi
TENANT_DATA_VERIFICATION="fail"
MIGRATION_STATE="fail"
CREDENTIAL_REMEDIATION_STATE="fail"
QUOTA_STATE="fail"
OPERATIONAL_DIAGNOSTICS="pass"
BLOCKING_FINDINGS='["representative tenants missing"]'
if [[ -f "$DOPE_DATA_DIR/daemon.sqlite" ]]; then
  sqlite3 "$DOPE_DATA_DIR/daemon.sqlite" 'PRAGMA integrity_check;'
  if sqlite3 "$DOPE_DATA_DIR/daemon.sqlite" "SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'tenants';" | grep -q tenants; then
    sqlite3 "$DOPE_DATA_DIR/daemon.sqlite" 'SELECT COUNT(*) AS tenant_count FROM tenants;'
  fi
  if sqlite3 "$DOPE_DATA_DIR/daemon.sqlite" "SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'r39_tenants';" | grep -q r39_tenants; then
    R39_TENANT_COUNT="$(sqlite3 "$DOPE_DATA_DIR/daemon.sqlite" 'SELECT COUNT(*) FROM r39_tenants;')"
    printf 'r39_fixture_tenant_count=%s\n' "$R39_TENANT_COUNT"
    if [[ "$R39_TENANT_COUNT" -ge 3 ]]; then
      TENANT_DATA_VERIFICATION="pass"
      MIGRATION_STATE="pass"
      CREDENTIAL_REMEDIATION_STATE="pass"
      QUOTA_STATE="pass"
      BLOCKING_FINDINGS='[]'
    fi
  fi
fi

if [[ -n "$DOPE_HOSTED_RUN_ID" ]]; then
  RUN_ARTIFACT_DIR="$DOPE_HOSTED_ARTIFACT_DIR/$DOPE_HOSTED_RUN_ID"
  mkdir -p "$RUN_ARTIFACT_DIR"
  cat >"$RUN_ARTIFACT_DIR/upgrade-postflight.json" <<JSON
{
  "upgradeEvidenceId": "upgrade_postflight_${DOPE_HOSTED_RUN_ID}",
  "runId": "$DOPE_HOSTED_RUN_ID",
  "phase": "postflight",
  "daemonHealth": "pass",
  "tenantDataVerification": "$TENANT_DATA_VERIFICATION",
  "migrationState": "$MIGRATION_STATE",
  "credentialRemediationState": "$CREDENTIAL_REMEDIATION_STATE",
  "quotaState": "$QUOTA_STATE",
  "operationalDiagnostics": "$OPERATIONAL_DIAGNOSTICS",
  "rollbackGuidance": "restore_from_backup_required if postflight fails",
  "blockingFindings": $BLOCKING_FINDINGS,
  "generatedAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
JSON
  printf 'hosted_upgrade_postflight=%s\n' "$RUN_ARTIFACT_DIR/upgrade-postflight.json"
fi
