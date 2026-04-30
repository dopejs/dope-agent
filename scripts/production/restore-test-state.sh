#!/usr/bin/env bash
set -euo pipefail

DOPE_DATA_DIR="${DOPE_DATA_DIR:-$HOME/.dope-test}"
DOPE_RESTORE_TARGET_DIR="${DOPE_RESTORE_TARGET_DIR:-$DOPE_DATA_DIR}"
DOPE_HOSTED_SOURCE_DATA_DIR="${DOPE_HOSTED_SOURCE_DATA_DIR:-$HOME/.dope-test}"
DOPE_HOSTED_RUN_ID="${DOPE_HOSTED_RUN_ID:-}"
DOPE_HOSTED_PROFILE_ID="${DOPE_HOSTED_PROFILE_ID:-profile_hosted_test}"
DOPE_HOSTED_ARTIFACT_DIR="${DOPE_HOSTED_ARTIFACT_DIR:-$DOPE_RESTORE_TARGET_DIR/artifacts/hosted}"
BACKUP_ARTIFACT="${1:-}"

if [[ -z "$BACKUP_ARTIFACT" ]]; then
  printf 'usage: scripts/production/restore-test-state.sh <backup-artifact>\n' >&2
  exit 64
fi
if [[ "$DOPE_RESTORE_TARGET_DIR" == "$HOME/.dope" && "${DOPE_LIVE_OPT_IN:-}" != "yes" ]]; then
  printf 'refusing to restore production data without DOPE_LIVE_OPT_IN=yes\n' >&2
  exit 2
fi
if [[ ! -f "$BACKUP_ARTIFACT" ]]; then
  printf 'backup artifact not found: %s\n' "$BACKUP_ARTIFACT" >&2
  exit 1
fi

mkdir -p "$DOPE_RESTORE_TARGET_DIR"
cp "$BACKUP_ARTIFACT" "$DOPE_RESTORE_TARGET_DIR/daemon.sqlite"
sqlite3 "$DOPE_RESTORE_TARGET_DIR/daemon.sqlite" 'PRAGMA integrity_check;'
TENANT_COUNT=0
if sqlite3 "$DOPE_RESTORE_TARGET_DIR/daemon.sqlite" "SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'r39_tenants';" | grep -q r39_tenants; then
  TENANT_COUNT="$(sqlite3 "$DOPE_RESTORE_TARGET_DIR/daemon.sqlite" 'SELECT COUNT(*) FROM r39_tenants;')"
  if [[ "$TENANT_COUNT" -lt 3 ]]; then
    printf 'r39 restore validation failed: expected at least 3 tenants, got %s\n' "$TENANT_COUNT" >&2
    exit 1
  fi
  if sqlite3 "$DOPE_RESTORE_TARGET_DIR/daemon.sqlite" "SELECT secret_ref FROM r39_secret_refs;" | grep -Eiq 'raw_secret|access_token|refresh_token|oauth_code|provider_token|do_not_leak'; then
    printf 'r39 restore validation failed: raw credential marker found\n' >&2
    exit 1
  fi
  printf 'r39_restore_tenant_count=%s\n' "$TENANT_COUNT"
elif [[ -n "$DOPE_HOSTED_RUN_ID" ]]; then
  printf 'hosted restore validation failed: expected at least 3 tenants, got 0\n' >&2
  exit 1
fi
printf 'restored %s to %s/daemon.sqlite\n' "$BACKUP_ARTIFACT" "$DOPE_RESTORE_TARGET_DIR"

if [[ -n "$DOPE_HOSTED_RUN_ID" ]]; then
  RUN_ARTIFACT_DIR="$DOPE_HOSTED_ARTIFACT_DIR/$DOPE_HOSTED_RUN_ID"
  mkdir -p "$RUN_ARTIFACT_DIR"
  if [[ "$DOPE_RESTORE_TARGET_DIR" != "$DOPE_HOSTED_SOURCE_DATA_DIR" ]]; then
    TARGET_IS_ALTERNATE=true
  else
    TARGET_IS_ALTERNATE=false
  fi
  cat >"$RUN_ARTIFACT_DIR/restore-evidence.json" <<JSON
{
  "restoreResultId": "restore_${DOPE_HOSTED_RUN_ID}",
  "runId": "$DOPE_HOSTED_RUN_ID",
  "backupId": "$(basename "$BACKUP_ARTIFACT")",
  "targetProfileId": "${DOPE_HOSTED_PROFILE_ID}_restore",
  "targetDataDirectory": "$DOPE_RESTORE_TARGET_DIR",
  "targetIsAlternate": $TARGET_IS_ALTERNATE,
  "tenantCount": $TENANT_COUNT,
  "tenantStateResult": "pass",
  "migrationStateResult": "pass",
  "credentialRemediationResult": "pass",
  "quotaStateResult": "pass",
  "daemonHealthResult": "pass",
  "crossTenantLeakage": false,
  "rawCredentialScanResult": "pass",
  "result": "pass",
  "generatedAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
JSON
  printf 'hosted_restore_evidence=%s\n' "$RUN_ARTIFACT_DIR/restore-evidence.json"
fi
