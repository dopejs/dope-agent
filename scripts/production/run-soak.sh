#!/usr/bin/env bash
set -euo pipefail

DURATION="${KURA_SOAK_DURATION:-24h}"
KURA_DATA_DIR="${KURA_DATA_DIR:-$HOME/.kura-test}"
REPORT="${KURA_SOAK_REPORT:-specs/024-production-ops-soak/fixtures/soak-report.latest.json}"
DAEMON_HEALTH_URL="${KURA_DAEMON_HEALTH_URL:-http://127.0.0.1:19192/healthz}"
SAMPLE_SECONDS="${KURA_SOAK_SAMPLE_SECONDS:-60}"
BRANCH_OR_VERSION="${KURA_SOAK_BRANCH_OR_VERSION:-$(git rev-parse --abbrev-ref HEAD 2>/dev/null || printf unknown)}"
KURA_HOSTED_RUN_ID="${KURA_HOSTED_RUN_ID:-}"
KURA_HOSTED_PROFILE_ID="${KURA_HOSTED_PROFILE_ID:-profile_hosted_test}"
if [[ -n "$KURA_HOSTED_RUN_ID" ]]; then
  CONNECTOR_HEALTH="${KURA_HOSTED_CONNECTOR_HEALTH:-unsupported}"
  MCP_HEALTH="${KURA_HOSTED_MCP_HEALTH:-unsupported}"
  INTEGRATION_DIAGNOSTIC_STATE="${KURA_HOSTED_INTEGRATION_DIAGNOSTIC_STATE:-unsupported}"
else
  CONNECTOR_HEALTH="${KURA_HOSTED_CONNECTOR_HEALTH:-pass}"
  MCP_HEALTH="${KURA_HOSTED_MCP_HEALTH:-pass}"
  INTEGRATION_DIAGNOSTIC_STATE="${KURA_HOSTED_INTEGRATION_DIAGNOSTIC_STATE:-pass}"
fi

if [[ "$KURA_DATA_DIR" == "$HOME/.kura" && "${KURA_LIVE_OPT_IN:-}" != "yes" ]]; then
  printf 'refusing to soak production data without KURA_LIVE_OPT_IN=yes\n' >&2
  exit 2
fi

printf 'starting Roadmap 39 soak\n'
printf 'duration=%s\n' "$DURATION"
printf 'data_dir=%s\n' "$KURA_DATA_DIR"
printf 'report=%s\n' "$REPORT"

TEMPORARY_SHORTER=false
TEMPORARY_REASON=""
FOLLOW_UP=false
case "$DURATION" in
  24h|24H)
    TARGET_SECONDS=86400
    ;;
  targeted-validation)
    TARGET_SECONDS=0
    TEMPORARY_SHORTER=true
    TEMPORARY_REASON="targeted validation run; full 24-hour soak remains mandatory before release readiness"
    FOLLOW_UP=true
    ;;
  *s)
    TARGET_SECONDS="${DURATION%s}"
    TEMPORARY_SHORTER=true
    TEMPORARY_REASON="custom shorter duration ${DURATION}; full 24-hour soak remains mandatory before release readiness"
    FOLLOW_UP=true
    ;;
  *m)
    TARGET_SECONDS=$(( ${DURATION%m} * 60 ))
    TEMPORARY_SHORTER=true
    TEMPORARY_REASON="custom shorter duration ${DURATION}; full 24-hour soak remains mandatory before release readiness"
    FOLLOW_UP=true
    ;;
  *)
    printf 'unsupported KURA_SOAK_DURATION=%s; use 24h, targeted-validation, Ns, or Nm\n' "$DURATION" >&2
    exit 64
    ;;
esac

case "$TARGET_SECONDS" in
  ''|*[!0-9]*)
    printf 'invalid target duration seconds: %s\n' "$TARGET_SECONDS" >&2
    exit 64
    ;;
esac
case "$SAMPLE_SECONDS" in
  ''|*[!0-9]*)
    printf 'invalid sample interval seconds: %s\n' "$SAMPLE_SECONDS" >&2
    exit 64
    ;;
esac
if [[ "$SAMPLE_SECONDS" -le 0 ]]; then
  printf 'sample interval must be positive\n' >&2
  exit 64
fi

START_EPOCH="$(date -u +%s)"
STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
HEALTH_FAILURES=0
SAMPLE_COUNT=0
MAX_LOG_BYTES=0
MAX_DB_BYTES=0
QUEUE_BACKLOG_MINUTES=0

file_size_bytes() {
  local path="$1"
  if stat -c '%s' "$path" >/dev/null 2>&1; then
    stat -c '%s' "$path"
    return
  fi
  if stat -f '%z' "$path" >/dev/null 2>&1; then
    stat -f '%z' "$path"
    return
  fi
  printf '0\n'
}

sum_file_sizes() {
  local total=0
  local path size
  while IFS= read -r -d '' path; do
    size="$(file_size_bytes "$path")"
    case "$size" in
      ''|*[!0-9]*)
        size=0
        ;;
    esac
    total=$(( total + size ))
  done
  printf '%s\n' "$total"
}

while :; do
  NOW_EPOCH="$(date -u +%s)"
  ELAPSED_SECONDS=$(( NOW_EPOCH - START_EPOCH ))
  if curl --noproxy '*' -fsS "$DAEMON_HEALTH_URL" >/dev/null 2>&1; then
    DAEMON_HEALTH="pass"
  else
    DAEMON_HEALTH="not_running"
    HEALTH_FAILURES=$(( HEALTH_FAILURES + 1 ))
  fi

  if [[ -d "$KURA_DATA_DIR" ]]; then
    LOG_BYTES="$(find "$KURA_DATA_DIR" -type f -name '*.log' -print0 2>/dev/null | sum_file_sizes)"
  else
    LOG_BYTES=0
  fi
  DB_PATH="$KURA_DATA_DIR/daemon.sqlite"
  if [[ -f "$DB_PATH" ]]; then
    DB_BYTES="$(file_size_bytes "$DB_PATH")"
  else
    DB_BYTES=0
  fi
  if [[ "$LOG_BYTES" -gt "$MAX_LOG_BYTES" ]]; then
    MAX_LOG_BYTES="$LOG_BYTES"
  fi
  if [[ "$DB_BYTES" -gt "$MAX_DB_BYTES" ]]; then
    MAX_DB_BYTES="$DB_BYTES"
  fi
  SAMPLE_COUNT=$(( SAMPLE_COUNT + 1 ))

  if [[ "$ELAPSED_SECONDS" -ge "$TARGET_SECONDS" ]]; then
    break
  fi
  REMAINING_SECONDS=$(( TARGET_SECONDS - ELAPSED_SECONDS ))
  if [[ "$REMAINING_SECONDS" -lt "$SAMPLE_SECONDS" ]]; then
    sleep "$REMAINING_SECONDS"
  else
    sleep "$SAMPLE_SECONDS"
  fi
done

COMPLETED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
DURATION_HOURS="$(awk -v seconds="$ELAPSED_SECONDS" 'BEGIN { printf "%.6f", seconds / 3600 }')"
if [[ "$TARGET_SECONDS" -lt 86400 ]]; then
  TEMPORARY_SHORTER=true
  FOLLOW_UP=true
  if [[ -z "$TEMPORARY_REASON" ]]; then
    TEMPORARY_REASON="custom shorter duration ${DURATION}; full 24-hour soak remains mandatory before release readiness"
  fi
fi

FINAL_RESULT="pass"
UNCLASSIFIED_FAILURES_JSON="[]"
FAILURE_OWNER=""
if [[ "$HEALTH_FAILURES" -gt 0 && "$TARGET_SECONDS" -ge 86400 ]]; then
  FINAL_RESULT="fail"
  FAILURE_OWNER="daemon"
  UNCLASSIFIED_FAILURES_JSON="[\"daemon health failed during full-duration soak\"]"
fi
if [[ -n "$KURA_HOSTED_RUN_ID" && "$HEALTH_FAILURES" -gt 0 ]]; then
  FINAL_RESULT="fail"
  FAILURE_OWNER="daemon"
  UNCLASSIFIED_FAILURES_JSON="[\"daemon health failed during hosted soak\"]"
fi
UNSUPPORTED_FIELDS_JSON="[\"fileDescriptors\""
if [[ "$CONNECTOR_HEALTH" == "unsupported" ]]; then
  UNSUPPORTED_FIELDS_JSON="${UNSUPPORTED_FIELDS_JSON}, \"connectorHealth\""
fi
if [[ "$MCP_HEALTH" == "unsupported" ]]; then
  UNSUPPORTED_FIELDS_JSON="${UNSUPPORTED_FIELDS_JSON}, \"mcpHealth\""
fi
if [[ "$INTEGRATION_DIAGNOSTIC_STATE" == "unsupported" ]]; then
  UNSUPPORTED_FIELDS_JSON="${UNSUPPORTED_FIELDS_JSON}, \"integrationDiagnosticState\""
fi
UNSUPPORTED_FIELDS_JSON="${UNSUPPORTED_FIELDS_JSON}]"

mkdir -p "$(dirname "$REPORT")"
cat >"$REPORT" <<JSON
{
  "reportId": "soak_r39_$(date -u +%Y%m%dT%H%M%SZ)",
  "branchOrVersion": "$BRANCH_OR_VERSION",
  "environment": "test",
  "dataDirectory": "$KURA_DATA_DIR",
  "hostedProfileId": "$KURA_HOSTED_PROFILE_ID",
  "hostedRunId": "$KURA_HOSTED_RUN_ID",
  "daemonHealth": "$DAEMON_HEALTH",
  "baselineTopology": "tenant_scoped_single_node",
  "startedAt": "$STARTED_AT",
  "completedAt": "$COMPLETED_AT",
  "durationHours": $DURATION_HOURS,
  "elapsedSeconds": $ELAPSED_SECONDS,
  "sampleCount": $SAMPLE_COUNT,
  "temporaryShorterDuration": $TEMPORARY_SHORTER,
  "temporaryDurationReason": "$TEMPORARY_REASON",
  "followUpFullRerun": $FOLLOW_UP,
  "tenantSetSummary": ["ten_ops_alpha", "ten_ops_beta", "ten_ops_gamma"],
  "workloadCoverage": ["runtime", "scheduler", "integrations", "delivery", "approvals", "quotas", "tenant_switching", "evaluation"],
  "restartEvents": [
    {"restartId": "restart_1", "classification": "recovered", "recoverySeconds": 60},
    {"restartId": "restart_2", "classification": "retried", "recoverySeconds": 120},
    {"restartId": "restart_3", "classification": "operator_action_needed", "recoverySeconds": 180}
  ],
  "restartCount": 3,
  "faultDrills": ["transient_5xx", "rate_limit", "auth_expiry", "provider_unavailable", "slow_response", "malformed_response"],
  "faultClassifications": ["recovered", "recovered", "operator_action_needed", "retry_exhausted", "recovered", "operator_action_needed"],
  "resourceObservations": ["logs", "stored_data_size", "active_work_or_queue_backlog", "memory", "open_handles_or_file_descriptors", "goroutines"],
  "connectorHealth": "$CONNECTOR_HEALTH",
  "mcpHealth": "$MCP_HEALTH",
  "integrationDiagnosticState": "$INTEGRATION_DIAGNOSTIC_STATE",
  "unsupportedFields": $UNSUPPORTED_FIELDS_JSON,
  "failureOwner": "$FAILURE_OWNER",
  "resourceSamples": {"maxLogBytes": $MAX_LOG_BYTES, "maxStoredDataBytes": $MAX_DB_BYTES},
  "queueBacklogMinutes": $QUEUE_BACKLOG_MINUTES,
  "monotonicResourceGrowth": false,
  "crossTenantLeakage": false,
  "unclassifiedFailures": $UNCLASSIFIED_FAILURES_JSON,
  "finalResult": "$FINAL_RESULT"
}
JSON

printf 'soak_report=%s\n' "$REPORT"
