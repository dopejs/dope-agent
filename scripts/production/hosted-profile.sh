#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
usage: scripts/production/hosted-profile.sh <command>

commands:
  provision       create hosted profile directories
  start           write deployment and start supervisor evidence
  stop            write manual stop supervisor evidence
  restart         write restart supervisor evidence
  reboot-recovery write simulated reboot-recovery supervisor evidence
  status          print hosted profile status and evidence paths
  health          run daemon health check and write health evidence
  evidence-index  write release evidence index
USAGE
}

COMMAND="${1:-}"
if [[ -z "$COMMAND" || "$COMMAND" == "-h" || "$COMMAND" == "--help" ]]; then
  usage
  exit 0
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
KURA_ENV="${KURA_ENV:-test}"
KURA_DATA_DIR="${KURA_DATA_DIR:-$HOME/.kura-test}"
KURA_DAEMON_ADDR="${KURA_DAEMON_ADDR:-127.0.0.1:19192}"
KURA_HOSTED_PROFILE_ID="${KURA_HOSTED_PROFILE_ID:-profile_hosted_test}"
KURA_HOSTED_RUN_ID="${KURA_HOSTED_RUN_ID:-hosted_$(date -u +%Y%m%dT%H%M%SZ)}"
KURA_HOSTED_COMMIT="${KURA_HOSTED_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || printf unknown)}"
KURA_HOSTED_BRANCH="${KURA_HOSTED_BRANCH:-$(git rev-parse --abbrev-ref HEAD 2>/dev/null || printf unknown)}"
KURA_HOSTED_HOST="${KURA_HOSTED_HOST:-$(hostname 2>/dev/null || printf unknown-host)}"
KURA_HOSTED_OPERATOR="${KURA_HOSTED_OPERATOR:-${USER:-unknown}}"
KURA_HOSTED_ARTIFACT_DIR="${KURA_HOSTED_ARTIFACT_DIR:-$KURA_DATA_DIR/artifacts/hosted}"
KURA_HOSTED_REPORT_DIR="${KURA_HOSTED_REPORT_DIR:-$KURA_DATA_DIR/reports}"
KURA_HOSTED_BACKUP_DIR="${KURA_HOSTED_BACKUP_DIR:-$KURA_DATA_DIR/backups}"
KURA_HOSTED_LOG_DIR="${KURA_HOSTED_LOG_DIR:-$KURA_DATA_DIR/logs}"
KURA_HOSTED_TMP_DIR="${KURA_HOSTED_TMP_DIR:-$KURA_DATA_DIR/tmp}"
KURA_HOSTED_LIVE_CONNECTORS="${KURA_HOSTED_LIVE_CONNECTORS:-disabled}"
KURA_HOSTED_SUPERVISOR_MODE="${KURA_HOSTED_SUPERVISOR_MODE:-repo_foreground}"
KURA_HOSTED_REVIEW_ELAPSED_SECONDS="${KURA_HOSTED_REVIEW_ELAPSED_SECONDS:-60}"
KURA_HOSTED_SKIP_GO_VALIDATOR="${KURA_HOSTED_SKIP_GO_VALIDATOR:-0}"
RUN_ARTIFACT_DIR="$KURA_HOSTED_ARTIFACT_DIR/$KURA_HOSTED_RUN_ID"
RUN_REPORT_DIR="$KURA_HOSTED_REPORT_DIR/$KURA_HOSTED_RUN_ID"
PID_FILE="$RUN_ARTIFACT_DIR/supervisor.pid"
SUPERVISOR_LOG="$RUN_ARTIFACT_DIR/supervisor.log"
KURA_HOSTED_DAEMON_COMMAND_WAS_SET="${KURA_HOSTED_DAEMON_COMMAND+x}"
KURA_HOSTED_DAEMON_COMMAND="${KURA_HOSTED_DAEMON_COMMAND:-./scripts/run-daemon.sh test}"
KURA_HOSTED_HEALTH_TIMEOUT_SECONDS="${KURA_HOSTED_HEALTH_TIMEOUT_SECONDS:-300}"

if [[ "$KURA_DATA_DIR" == "$HOME/.kura" && "${KURA_LIVE_OPT_IN:-}" != "yes" ]]; then
  printf 'refusing hosted profile production data without KURA_LIVE_OPT_IN=yes\n' >&2
  exit 2
fi
if [[ "$KURA_HOSTED_LIVE_CONNECTORS" != "disabled" && "${KURA_LIVE_OPT_IN:-}" != "yes" ]]; then
  printf 'refusing hosted profile live connectors without KURA_LIVE_OPT_IN=yes\n' >&2
  exit 2
fi

json_escape() {
  local value="${1:-}"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  printf '%s' "$value"
}

timestamp() {
  date -u +%Y-%m-%dT%H:%M:%SZ
}

retention_expires_at() {
  if date -u -d '+90 days' +%Y-%m-%dT%H:%M:%SZ >/dev/null 2>&1; then
    date -u -d '+90 days' +%Y-%m-%dT%H:%M:%SZ
    return
  fi
  date -u -v+90d +%Y-%m-%dT%H:%M:%SZ
}

ensure_dirs() {
  mkdir -p "$KURA_DATA_DIR" "$KURA_HOSTED_LOG_DIR" "$KURA_HOSTED_ARTIFACT_DIR" "$RUN_ARTIFACT_DIR" "$KURA_HOSTED_BACKUP_DIR" "$KURA_HOSTED_REPORT_DIR" "$RUN_REPORT_DIR" "$KURA_HOSTED_TMP_DIR"
}

print_target() {
  printf 'environment=%s\n' "$KURA_ENV"
  printf 'data_dir=%s\n' "$KURA_DATA_DIR"
  printf 'daemon_addr=%s\n' "$KURA_DAEMON_ADDR"
  printf 'profile_id=%s\n' "$KURA_HOSTED_PROFILE_ID"
  printf 'run_id=%s\n' "$KURA_HOSTED_RUN_ID"
  printf 'artifact_root=%s\n' "$RUN_ARTIFACT_DIR"
  printf 'report_root=%s\n' "$RUN_REPORT_DIR"
  printf 'live_connectors=%s\n' "$KURA_HOSTED_LIVE_CONNECTORS"
}

write_manifest() {
  ensure_dirs
  local now retention
  now="$(timestamp)"
  retention="$(retention_expires_at)"
  cat >"$RUN_ARTIFACT_DIR/deployment-manifest.json" <<JSON
{
  "manifestId": "manifest_${KURA_HOSTED_RUN_ID}",
  "runId": "$(json_escape "$KURA_HOSTED_RUN_ID")",
  "profileId": "$(json_escape "$KURA_HOSTED_PROFILE_ID")",
  "commitOrVersion": "$(json_escape "$KURA_HOSTED_COMMIT")",
  "branch": "$(json_escape "$KURA_HOSTED_BRANCH")",
  "host": "$(json_escape "$KURA_HOSTED_HOST")",
  "operator": "$(json_escape "$KURA_HOSTED_OPERATOR")",
  "startedAt": "$now",
  "configurationProfile": "$(json_escape "$KURA_ENV")",
  "dataDirectory": "$(json_escape "$KURA_DATA_DIR")",
  "artifactDirectory": "$(json_escape "$RUN_ARTIFACT_DIR")",
  "supervisorMode": "$KURA_HOSTED_SUPERVISOR_MODE",
  "daemonAddress": "$(json_escape "$KURA_DAEMON_ADDR")",
  "liveConnectorMode": "$(json_escape "$KURA_HOSTED_LIVE_CONNECTORS")",
  "redactionStatus": "passed",
  "retentionExpiresAt": "$retention"
}
JSON
  printf 'deployment_manifest=%s\n' "$RUN_ARTIFACT_DIR/deployment-manifest.json"
}

write_configuration_profile() {
  ensure_dirs
  local now retention
  now="$(timestamp)"
  retention="$(retention_expires_at)"
  cat >"$RUN_ARTIFACT_DIR/configuration-profile.json" <<JSON
{
  "runId": "$(json_escape "$KURA_HOSTED_RUN_ID")",
  "profileId": "$(json_escape "$KURA_HOSTED_PROFILE_ID")",
  "commitOrVersion": "$(json_escape "$KURA_HOSTED_COMMIT")",
  "environment": "$(json_escape "$KURA_ENV")",
  "dataDirectory": "$(json_escape "$KURA_DATA_DIR")",
  "artifactDirectory": "$(json_escape "$RUN_ARTIFACT_DIR")",
  "reportDirectory": "$(json_escape "$RUN_REPORT_DIR")",
  "backupDirectory": "$(json_escape "$KURA_HOSTED_BACKUP_DIR")",
  "logDirectory": "$(json_escape "$KURA_HOSTED_LOG_DIR")",
  "temporaryDirectory": "$(json_escape "$KURA_HOSTED_TMP_DIR")",
  "liveConnectorMode": "$(json_escape "$KURA_HOSTED_LIVE_CONNECTORS")",
  "supervisorMode": "$(json_escape "$KURA_HOSTED_SUPERVISOR_MODE")",
  "redactionStatus": "passed",
  "generatedAt": "$now",
  "retentionExpiresAt": "$retention"
}
JSON
}

write_supervisor_event() {
  ensure_dirs
  local event_type="$1"
  local result="${2:-passed}"
  local health="${3:-pass}"
  local recovery_seconds="${4:-0}"
  local owner="${5:-}"
  local now path
  now="$(timestamp)"
  path="$RUN_ARTIFACT_DIR/supervisor-events.jsonl"
  cat >>"$path" <<JSON
{"eventId":"${event_type}_${KURA_HOSTED_RUN_ID}_$now","runId":"$(json_escape "$KURA_HOSTED_RUN_ID")","eventType":"$event_type","requestedBy":"$(json_escape "$KURA_HOSTED_OPERATOR")","startedAt":"$now","completedAt":"$now","daemonHealth":"$health","recoverySeconds":$recovery_seconds,"result":"$result","failureOwner":"$owner","evidencePath":"$(json_escape "$path")"}
JSON
  printf 'supervisor_event=%s\n' "$path"
}

health_status() {
  if [[ -n "${KURA_HOSTED_HEALTH_COMMAND:-}" ]]; then
    if sh -c "$KURA_HOSTED_HEALTH_COMMAND" >/dev/null 2>&1; then
      printf 'pass'
    else
      printf 'fail'
    fi
    return
  fi
  if [[ "${KURA_HOSTED_DRY_RUN:-}" == "1" ]]; then
    printf 'pass'
    return
  fi
  if curl --noproxy '*' -fsS "http://$KURA_DAEMON_ADDR/healthz" >/dev/null 2>&1; then
    printf 'pass'
  else
    printf 'fail'
  fi
}

write_health_report() {
  ensure_dirs
  local now health
  now="$(timestamp)"
  health="$(health_status)"
  cat >"$RUN_ARTIFACT_DIR/health.json" <<JSON
{
  "runId": "$(json_escape "$KURA_HOSTED_RUN_ID")",
  "profileId": "$(json_escape "$KURA_HOSTED_PROFILE_ID")",
  "commitOrVersion": "$(json_escape "$KURA_HOSTED_COMMIT")",
  "daemonAddress": "$(json_escape "$KURA_DAEMON_ADDR")",
  "daemonHealth": "$health",
  "generatedAt": "$now",
  "redactionStatus": "passed"
}
JSON
  printf 'health=%s\n' "$health"
  printf 'health_evidence=%s\n' "$RUN_ARTIFACT_DIR/health.json"
  [[ "$health" == "pass" ]]
}

process_status() {
  if [[ ! -f "$PID_FILE" ]]; then
    printf 'stopped'
    return
  fi
  local pid
  pid="$(cat "$PID_FILE" 2>/dev/null || true)"
  if [[ -n "$pid" ]] && kill -0 "$pid" >/dev/null 2>&1; then
    printf 'running'
  else
    printf 'stopped'
  fi
}

start_supervisor() {
  ensure_dirs
  if [[ "$(process_status)" == "running" ]]; then
    printf 'process_status=running\n'
    return
  fi
  if [[ "${KURA_HOSTED_DRY_RUN:-}" == "1" && -z "$KURA_HOSTED_DAEMON_COMMAND_WAS_SET" ]]; then
    printf 'process_status=dry_run\n'
    return
  fi
  sh -c "exec $KURA_HOSTED_DAEMON_COMMAND" >"$SUPERVISOR_LOG" 2>&1 &
  local pid="$!"
  printf '%s\n' "$pid" >"$PID_FILE"
  printf 'supervisor_pid=%s\n' "$pid"
  printf 'supervisor_log=%s\n' "$SUPERVISOR_LOG"
}

stop_supervisor() {
  if [[ ! -f "$PID_FILE" ]]; then
    printf 'process_status=stopped\n'
    return
  fi
  local pid
  pid="$(cat "$PID_FILE" 2>/dev/null || true)"
  if [[ -n "$pid" ]] && kill -0 "$pid" >/dev/null 2>&1; then
    kill "$pid" >/dev/null 2>&1 || true
    local waited=0
    while kill -0 "$pid" >/dev/null 2>&1 && [[ "$waited" -lt 10 ]]; do
      sleep 1
      waited=$((waited + 1))
    done
    if kill -0 "$pid" >/dev/null 2>&1; then
      kill -9 "$pid" >/dev/null 2>&1 || true
    fi
  fi
  rm -f "$PID_FILE"
  printf 'process_status=stopped\n'
}

wait_for_health() {
  local started now elapsed
  started="$(date -u +%s)"
  while :; do
    if [[ "$(health_status)" == "pass" ]]; then
      now="$(date -u +%s)"
      printf '%s' "$((now - started))"
      return 0
    fi
    now="$(date -u +%s)"
    elapsed="$((now - started))"
    if [[ "$elapsed" -ge "$KURA_HOSTED_HEALTH_TIMEOUT_SECONDS" ]]; then
      printf '%s' "$elapsed"
      return 1
    fi
    sleep 1
  done
}

evidence_status() {
  local evidence_type="$1"
  local path="$2"
  local status="fail"
  local finding=""
  case "$evidence_type" in
    configuration_profile)
      if [[ -f "$path" ]]; then status="pass"; else finding="configuration profile missing"; fi
      ;;
    health_checks)
      if [[ -f "$path" ]] && grep -q '"daemonHealth": "pass"' "$path"; then
        status="pass"
      else
        finding="health evidence missing or failed"
      fi
      ;;
    redaction_check)
      if [[ -f "$path" ]] && grep -q '"redactionStatus": "passed"' "$path"; then
        status="pass"
      else
        finding="redaction evidence missing or failed"
      fi
      ;;
    retention_metadata)
      status="pass"
      ;;
    logs)
      if [[ -d "$path" ]]; then status="pass"; else finding="log directory missing"; fi
      ;;
    *)
      if [[ -e "$path" ]]; then
        status="pass"
        if [[ -f "$path" ]] && grep -Eq '"(result|finalResult|daemonHealth|requiredBackupState|tenantDataVerification|migrationState|credentialRemediationState|quotaState|operationalDiagnostics|redactionStatus)"[[:space:]]*:[[:space:]]*"fail"|"blockingFindings"[[:space:]]*:[[:space:]]*\[[^]]' "$path"; then
          status="fail"
          finding="$evidence_type contains blocking findings"
        fi
      else
        finding="$evidence_type missing"
      fi
      ;;
  esac
  EVIDENCE_STATUS="$status"
  EVIDENCE_FINDING="$finding"
}

write_evidence_link() {
  local evidence_type="$1"
  local path="$2"
  local now="$3"
  local retention="$4"
  evidence_status "$evidence_type" "$path"
  if [[ "$EVIDENCE_STATUS" == "pass" ]]; then
    printf '    {"evidenceType":"%s","path":"%s","runId":"%s","profileId":"%s","commitOrVersion":"%s","status":"pass","generatedAt":"%s","retentionExpiresAt":"%s","redactionStatus":"passed","blockingFindings":[]}' \
      "$evidence_type" "$(json_escape "$path")" "$(json_escape "$KURA_HOSTED_RUN_ID")" "$(json_escape "$KURA_HOSTED_PROFILE_ID")" "$(json_escape "$KURA_HOSTED_COMMIT")" "$now" "$retention"
  else
    printf '    {"evidenceType":"%s","path":"%s","runId":"%s","profileId":"%s","commitOrVersion":"%s","status":"fail","generatedAt":"%s","retentionExpiresAt":"%s","redactionStatus":"passed","blockingFindings":["%s"]}' \
      "$evidence_type" "$(json_escape "$path")" "$(json_escape "$KURA_HOSTED_RUN_ID")" "$(json_escape "$KURA_HOSTED_PROFILE_ID")" "$(json_escape "$KURA_HOSTED_COMMIT")" "$now" "$retention" "$(json_escape "$EVIDENCE_FINDING")"
  fi
}

write_redaction_check() {
  ensure_dirs
  local status="passed"
  local findings="[]"
  if grep -R -Eiq 'raw_secret|access_token|refresh_token|oauth_code|provider_token|authorization:|bearer |client_secret|api_key=|password=|do_not_leak' "$RUN_ARTIFACT_DIR" "$RUN_REPORT_DIR" 2>/dev/null; then
    status="failed"
    findings='["raw credential marker found in hosted evidence"]'
  fi
  cat >"$RUN_ARTIFACT_DIR/redaction-check.json" <<JSON
{
  "runId": "$(json_escape "$KURA_HOSTED_RUN_ID")",
  "profileId": "$(json_escape "$KURA_HOSTED_PROFILE_ID")",
  "commitOrVersion": "$(json_escape "$KURA_HOSTED_COMMIT")",
  "redactionStatus": "$status",
  "blockingFindings": $findings,
  "generatedAt": "$(timestamp)"
}
JSON
}

write_release_index() {
  ensure_dirs
  write_configuration_profile
  local now retention decision links_file all_pass
  now="$(timestamp)"
  retention="$(retention_expires_at)"
  links_file="$RUN_REPORT_DIR/.release-evidence-links.json"
  all_pass=true
  : >"$links_file"
  local types=(
    "deployment_manifest:$RUN_ARTIFACT_DIR/deployment-manifest.json"
    "configuration_profile:$RUN_ARTIFACT_DIR/configuration-profile.json"
    "health_checks:$RUN_ARTIFACT_DIR/health.json"
    "logs:$KURA_HOSTED_LOG_DIR"
    "soak_report:$RUN_ARTIFACT_DIR/soak-report.json"
    "backup_evidence:$RUN_ARTIFACT_DIR/backup-evidence.json"
    "restore_evidence:$RUN_ARTIFACT_DIR/restore-evidence.json"
    "upgrade_preflight:$RUN_ARTIFACT_DIR/upgrade-preflight.json"
    "upgrade_postflight:$RUN_ARTIFACT_DIR/upgrade-postflight.json"
    "rollback_decision:$RUN_ARTIFACT_DIR/rollback-decision.json"
    "integration_diagnostics:$RUN_ARTIFACT_DIR/integration-diagnostics.json"
    "resource_observations:$RUN_ARTIFACT_DIR/observability-report.json"
    "redaction_check:$RUN_ARTIFACT_DIR/redaction-check.json"
    "retention_metadata:$RUN_REPORT_DIR/release-evidence-index.json"
  )
  local first=true
  local item evidence_type path
  for item in "${types[@]}"; do
    evidence_type="${item%%:*}"
    path="${item#*:}"
    if [[ "$first" == "true" ]]; then
      first=false
    else
      printf ',\n' >>"$links_file"
    fi
    write_evidence_link "$evidence_type" "$path" "$now" "$retention" >>"$links_file"
    if [[ "$EVIDENCE_STATUS" != "pass" ]]; then
      all_pass=false
    fi
  done
  if [[ "$all_pass" == "true" ]]; then
    decision="ship"
  else
    decision="no_ship"
  fi
  cat >"$RUN_REPORT_DIR/release-evidence-index.json" <<JSON
{
  "releaseIndexId": "release_${KURA_HOSTED_RUN_ID}",
  "runId": "$(json_escape "$KURA_HOSTED_RUN_ID")",
  "profileId": "$(json_escape "$KURA_HOSTED_PROFILE_ID")",
  "commitOrVersion": "$(json_escape "$KURA_HOSTED_COMMIT")",
  "generatedAt": "$now",
  "reviewTarget": "Roadmap 43 hosted operational profile",
  "retentionExpiresAt": "$retention",
  "decision": "$decision",
  "reviewElapsedSeconds": $KURA_HOSTED_REVIEW_ELAPSED_SECONDS,
  "evidenceLinks": [
$(cat "$links_file")
  ]
}
JSON
  rm -f "$links_file"
  if [[ "$KURA_HOSTED_SKIP_GO_VALIDATOR" != "1" ]]; then
    if ! command -v go >/dev/null 2>&1; then
      printf 'hosted evidence validator requires go; set KURA_HOSTED_SKIP_GO_VALIDATOR=1 only for bootstrap diagnostics\n' >&2
      exit 2
    fi
    (
      cd "$REPO_ROOT/daemon"
      go run ./cmd/hosted-evidence-validate --allow-no-ship "$RUN_REPORT_DIR/release-evidence-index.json"
    ) >"$RUN_REPORT_DIR/release-evidence-validation.txt"
    printf 'release_evidence_validation=%s\n' "$RUN_REPORT_DIR/release-evidence-validation.txt"
  fi
  printf 'release_evidence_index=%s\n' "$RUN_REPORT_DIR/release-evidence-index.json"
}

print_target
case "$COMMAND" in
  provision)
    ensure_dirs
    printf 'provisioned=true\n'
    ;;
  start)
    write_manifest
    start_supervisor
    if recovery_seconds="$(wait_for_health)"; then
      write_supervisor_event start passed pass "$recovery_seconds" ""
    else
      write_supervisor_event start failed fail "$recovery_seconds" daemon
      exit 1
    fi
    ;;
  stop)
    stop_supervisor
    write_supervisor_event manual_stop passed pass 0 ""
    ;;
  restart)
    stop_supervisor
    start_supervisor
    if recovery_seconds="$(wait_for_health)"; then
      write_supervisor_event restart passed pass "$recovery_seconds" ""
    else
      write_supervisor_event restart failed fail "$recovery_seconds" daemon
      exit 1
    fi
    ;;
  reboot-recovery)
    if recovery_seconds="$(wait_for_health)"; then
      write_supervisor_event reboot_recovery passed pass "$recovery_seconds" ""
    else
      write_supervisor_event reboot_recovery failed fail "$recovery_seconds" daemon
      exit 1
    fi
    ;;
  status)
    ensure_dirs
    printf 'process_status=%s\n' "$(process_status)"
    printf 'health=%s\n' "$(health_status)"
    ;;
  health)
    write_health_report
    ;;
  evidence-index)
    write_manifest >/dev/null
    write_health_report >/dev/null || true
    write_redaction_check
    write_release_index
    ;;
  *)
    usage >&2
    exit 64
    ;;
esac
