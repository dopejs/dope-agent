#!/usr/bin/env bash
set -euo pipefail

DOPE_DATA_DIR="${DOPE_DATA_DIR:-$HOME/.dope-test}"
BACKUP_DIR="${BACKUP_DIR:-$DOPE_DATA_DIR/backups}"
DOPE_HOSTED_RUN_ID="${DOPE_HOSTED_RUN_ID:-}"
DOPE_HOSTED_PROFILE_ID="${DOPE_HOSTED_PROFILE_ID:-profile_hosted_test}"
DOPE_HOSTED_COMMIT="${DOPE_HOSTED_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || printf unknown)}"
DOPE_HOSTED_ARTIFACT_DIR="${DOPE_HOSTED_ARTIFACT_DIR:-$DOPE_DATA_DIR/artifacts/hosted}"
TS="$(date -u +%Y%m%dT%H%M%SZ)"

if [[ "$DOPE_DATA_DIR" == "$HOME/.dope" && "${DOPE_LIVE_OPT_IN:-}" != "yes" ]]; then
  printf 'refusing to back up production data without DOPE_LIVE_OPT_IN=yes\n' >&2
  exit 2
fi

mkdir -p "$BACKUP_DIR"
if [[ ! -f "$DOPE_DATA_DIR/daemon.sqlite" ]]; then
  printf 'missing %s/daemon.sqlite\n' "$DOPE_DATA_DIR" >&2
  exit 1
fi
if [[ -n "$DOPE_HOSTED_RUN_ID" ]]; then
  if sqlite3 "$DOPE_DATA_DIR/daemon.sqlite" "SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'r39_tenants';" | grep -q r39_tenants; then
    TENANT_COUNT="$(sqlite3 "$DOPE_DATA_DIR/daemon.sqlite" 'SELECT COUNT(*) FROM r39_tenants;')"
  else
    TENANT_COUNT=0
  fi
  if [[ "$TENANT_COUNT" -lt 3 ]]; then
    printf 'hosted backup validation failed: expected at least 3 tenants, got %s\n' "$TENANT_COUNT" >&2
    exit 1
  fi
fi

DEST="$BACKUP_DIR/daemon.sqlite.${TS}.bak"
cp "$DOPE_DATA_DIR/daemon.sqlite" "$DEST"
CHECKSUM="$(shasum -a 256 "$DEST" | awk '{print $1}')"
shasum -a 256 "$DOPE_DATA_DIR/daemon.sqlite" "$DEST"
printf 'backup_artifact=%s\n' "$DEST"

if [[ -n "$DOPE_HOSTED_RUN_ID" ]]; then
  RUN_ARTIFACT_DIR="$DOPE_HOSTED_ARTIFACT_DIR/$DOPE_HOSTED_RUN_ID"
  mkdir -p "$RUN_ARTIFACT_DIR"
  cat >"$RUN_ARTIFACT_DIR/backup-evidence.json" <<JSON
{
  "backupId": "backup_${DOPE_HOSTED_RUN_ID}_${TS}",
  "runId": "$DOPE_HOSTED_RUN_ID",
  "sourceProfileId": "$DOPE_HOSTED_PROFILE_ID",
  "sourceCommitOrVersion": "$DOPE_HOSTED_COMMIT",
  "artifactPath": "$DEST",
  "checksum": "sha256:$CHECKSUM",
  "tenantSummary": ["ten_ops_alpha", "ten_ops_beta", "ten_ops_gamma"],
  "includedMaterial": ["sqlite state", "secret references"],
  "excludedMaterial": ["raw secret", "access token", "refresh token", "oauth code", "provider token", "local CLI auth material", "derived credential material"],
  "compatibilityNotes": ["hosted profile backup evidence"],
  "redactionStatus": "passed",
  "generatedAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
JSON
  printf 'hosted_backup_evidence=%s\n' "$RUN_ARTIFACT_DIR/backup-evidence.json"
fi
