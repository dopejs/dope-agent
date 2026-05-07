#!/usr/bin/env bash

set -euo pipefail

DAEMON_URL="${DAEMON_URL:-http://127.0.0.1:19192}"
DB_PATH="${DOPE_TEST_DB_PATH:-${HOME}/.dope-test/daemon.sqlite}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

api_get() {
  local path="$1"
  curl --noproxy '*' -fsS "${DAEMON_URL}${path}" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "X-Dope-Tenant-ID: ${TENANT_ID}"
}

api_post() {
  local path="$1"
  curl --noproxy '*' -fsS -X POST "${DAEMON_URL}${path}" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "X-Dope-Tenant-ID: ${TENANT_ID}"
}

require_jq() {
  local input="$1"
  local filter="$2"
  local label="$3"
  if ! printf '%s' "${input}" | jq -e "${filter}" >/dev/null; then
    echo "phase47 walkthrough assertion failed: ${label}" >&2
    printf '%s\n' "${input}" >&2
    exit 1
  fi
}

require_cmd curl
require_cmd jq
require_cmd sqlite3

curl --noproxy '*' -fsS "${DAEMON_URL}/healthz" >/dev/null || {
  echo "daemon is not healthy at ${DAEMON_URL}; start it with: make daemon-run-test" >&2
  exit 1
}

if [[ ! -f "${DB_PATH}" ]]; then
  echo "test database not found at ${DB_PATH}; start the test daemon first" >&2
  exit 1
fi

PAIRING="$(curl --noproxy '*' -fsS -X POST "${DAEMON_URL}/v1/auth/pairings/start" \
  -H 'Content-Type: application/json' \
  -d '{"mode":"local","label":"phase47-public-quota-walkthrough"}')"
PAIRING_ID="$(printf '%s' "${PAIRING}" | jq -r '.pairing.pairingId')"
PAIRING_CODE="$(printf '%s' "${PAIRING}" | jq -r '.pairingCode')"
COMPLETE="$(curl --noproxy '*' -fsS -X POST "${DAEMON_URL}/v1/auth/pairings/${PAIRING_ID}/complete" \
  -H 'Content-Type: application/json' \
  -d "{\"code\":\"${PAIRING_CODE}\"}")"
TOKEN="$(printf '%s' "${COMPLETE}" | jq -r '.accessToken')"
TENANT_ID="$(curl --noproxy '*' -fsS "${DAEMON_URL}/v1/auth/me" \
  -H "Authorization: Bearer ${TOKEN}" | jq -r '.currentTenant.tenantId')"

NOW="$(sqlite3 ':memory:' "select strftime('%Y-%m-%dT%H:%M:%SZ','now');")"
STARTED_AT="$(sqlite3 ':memory:' "select strftime('%Y-%m-%dT%H:%M:%SZ','now','-1 hour');")"
EXPIRES_AT="$(sqlite3 ':memory:' "select strftime('%Y-%m-%dT%H:%M:%SZ','now','+1 day');")"
CURRENT_MONTH_START="$(sqlite3 ':memory:' "select strftime('%Y-%m-01T00:00:00Z','now');")"
NEXT_MONTH_START="$(sqlite3 ':memory:' "select strftime('%Y-%m-01T00:00:00Z','now','start of month','+1 month');")"
PREVIOUS_MONTH_START="$(sqlite3 ':memory:' "select strftime('%Y-%m-01T00:00:00Z','now','start of month','-1 month');")"
OTHER_TENANT_ID="ten_phase47_walkthrough_other"

sqlite3 "${DB_PATH}" <<SQL
insert or replace into billing_tenant_plans(plan_id, tenant_id, plan_key, status, enforcement_mode, effective_at, superseded_at, assigned_by_principal_id, assignment_reason, document_json)
values('plan_phase47_walkthrough', '${TENANT_ID}', 'finite', 'active', 'enforced', '${STARTED_AT}', null, 'prn_phase47', 'phase47 walkthrough', '{}');
insert or replace into billing_tenant_plans(plan_id, tenant_id, plan_key, status, enforcement_mode, effective_at, superseded_at, assigned_by_principal_id, assignment_reason, document_json)
values('plan_phase47_other', '${OTHER_TENANT_ID}', 'finite', 'active', 'enforced', '${STARTED_AT}', null, 'prn_phase47', 'phase47 walkthrough other tenant', '{}');
insert or replace into billing_quota_periods(quota_period_id, tenant_id, category, period_kind, period_start, period_end, carryover_from_period_id, status)
values('period_phase47_run_current', '${TENANT_ID}', 'run_launches', 'monthly', '${CURRENT_MONTH_START}', '${NEXT_MONTH_START}', null, 'open');
insert or replace into billing_quota_periods(quota_period_id, tenant_id, category, period_kind, period_start, period_end, carryover_from_period_id, status)
values('period_phase47_run_previous', '${TENANT_ID}', 'run_launches', 'monthly', '${PREVIOUS_MONTH_START}', '${CURRENT_MONTH_START}', null, 'closed');
insert or replace into billing_quota_periods(quota_period_id, tenant_id, category, period_kind, period_start, period_end, carryover_from_period_id, status)
values('period_phase47_runtime_current', '${TENANT_ID}', 'runtime_tool_calls', 'daily', strftime('%Y-%m-%dT00:00:00Z','now'), strftime('%Y-%m-%dT00:00:00Z','now','+1 day'), null, 'open');
insert or replace into billing_usage_counters(usage_counter_id, tenant_id, category, quota_period_id, committed_amount, reserved_amount, adjusted_amount, carryover_amount, updated_at)
values('counter_phase47_run_current', '${TENANT_ID}', 'run_launches', 'period_phase47_run_current', 8, 0, 0, 0, '${NOW}');
insert or replace into billing_usage_counters(usage_counter_id, tenant_id, category, quota_period_id, committed_amount, reserved_amount, adjusted_amount, carryover_amount, updated_at)
values('counter_phase47_run_previous', '${TENANT_ID}', 'run_launches', 'period_phase47_run_previous', 5, 0, 0, 0, '${CURRENT_MONTH_START}');
insert or replace into billing_usage_counters(usage_counter_id, tenant_id, category, quota_period_id, committed_amount, reserved_amount, adjusted_amount, carryover_amount, updated_at)
values('counter_phase47_runtime_current', '${TENANT_ID}', 'runtime_tool_calls', 'period_phase47_runtime_current', 3, 0, 0, 0, '${NOW}');
insert or replace into billing_quota_overrides(quota_override_id, tenant_id, category, limit_amount, carryover_enabled, carryover_max, effective_at, expires_at, reason, created_by_principal_id)
values('override_phase47_run', '${TENANT_ID}', 'run_launches', 10, null, null, '${STARTED_AT}', null, 'phase47 walkthrough override', 'prn_phase47');
insert or replace into billing_abuse_restrictions(restriction_id, tenant_id, status, affected_category, recovery_action, visible_reason_code, source_audit_ref, support_contact_allowed, started_at, expires_at, document_json)
values('restriction_phase47_runtime', '${TENANT_ID}', 'active', 'runtime_tool_calls', 'contact_support', 'abuse_restriction:temporary', 'audit_phase47_runtime', 1, '${STARTED_AT}', '${EXPIRES_AT}', '{"detectionSignals":"not exported"}');
insert or replace into billing_quota_denials(denial_id, tenant_id, category, quota_period_id, operation_key, reason_code, requested_amount, remaining_amount, guarded_entry_point, created_at)
values('denial_phase47_run', '${TENANT_ID}', 'run_launches', 'period_phase47_run_current', 'tenant:${TENANT_ID}:run:phase47', 'quota_denied:run_launches_exhausted', 1, 0, 'POST /v1/runs', '${NOW}');
insert or replace into billing_quota_denials(denial_id, tenant_id, category, quota_period_id, operation_key, reason_code, requested_amount, remaining_amount, guarded_entry_point, created_at)
values('denial_phase47_abuse', '${TENANT_ID}', 'runtime_tool_calls', 'period_phase47_runtime_current', 'tenant:${TENANT_ID}:tool_call:phase47', 'abuse_restriction:temporary', 1, 0, 'tool call creation before invocation', '${NOW}');
insert or replace into billing_quota_denials(denial_id, tenant_id, category, quota_period_id, operation_key, reason_code, requested_amount, remaining_amount, guarded_entry_point, created_at)
values('denial_phase47_other', '${OTHER_TENANT_ID}', 'run_launches', null, 'tenant:${OTHER_TENANT_ID}:run:phase47', 'quota_denied:run_launches_exhausted', 1, 0, 'POST /v1/runs', '${NOW}');
insert or ignore into billing_usage_events(usage_event_id, tenant_id, category, quota_period_id, operation_key, event_kind, amount, reason_code, reason, actor_principal_id, outcome, created_at, document_json)
values('usage_event_phase47_denial', '${TENANT_ID}', 'run_launches', 'period_phase47_run_current', 'tenant:${TENANT_ID}:run:phase47', 'denial', 1, 'quota_denied:run_launches_exhausted', 'phase47 walkthrough', 'prn_phase47', 'denied', '${NOW}', '{}');
insert or ignore into billing_usage_events(usage_event_id, tenant_id, category, quota_period_id, operation_key, event_kind, amount, reason_code, reason, actor_principal_id, outcome, created_at, document_json)
values('usage_event_phase47_abuse', '${TENANT_ID}', 'runtime_tool_calls', 'period_phase47_runtime_current', 'tenant:${TENANT_ID}:tool_call:phase47', 'denial', 1, 'abuse_restriction:temporary', 'phase47 walkthrough abuse restriction', 'prn_phase47', 'denied', '${NOW}', '{}');
SQL

DASHBOARD="$(api_get '/v1/billing/quota-dashboard')"
if ! printf '%s' "${DASHBOARD}" | jq -e --arg tenant "${TENANT_ID}" '
    .tenantId == $tenant and
    ([.sections[].items[]] | length) >= 7 and
    ([.sections[].items[] | select(.category=="run_launches")][0] |
      .status == "near_limit" and .nearLimit == true and .previousPeriod.consumedAmount == 5 and .override.reason == "phase47 walkthrough override") and
    ([.sections[].items[] | select(.category=="runtime_tool_calls")][0] |
      .status == "restricted" and .restriction.visibleReasonCode == "abuse_restriction:temporary" and .restriction.sourceAuditRef == "audit_phase47_runtime")
  ' >/dev/null; then
  echo "phase47 walkthrough assertion failed: quota dashboard projection" >&2
  printf '%s\n' "${DASHBOARD}" >&2
  exit 1
fi

RUN_DETAIL="$(api_get '/v1/billing/denials/denial_phase47_run')"
require_jq "${RUN_DETAIL}" \
  '.classification == "quota_exhaustion" and (.recoveryActions | index("request_override")) and .operationRef == "run:phase47"' \
  "ordinary quota denial detail"

ABUSE_DETAIL="$(api_get '/v1/billing/denials/denial_phase47_abuse')"
require_jq "${ABUSE_DETAIL}" \
  '.classification == "abuse_restriction" and .restriction.sourceAuditRef == "audit_phase47_runtime" and .restriction.expiresAt != null' \
  "abuse restriction denial detail"

RUN_EXPORT="$(api_post '/v1/billing/denials/denial_phase47_run/evidence-export')"
require_jq "${RUN_EXPORT}" \
  '.denial.denialId == "denial_phase47_run" and (.redactions | length) >= 4 and (.auditRefs | length) >= 2 and .effectiveLimitState.quota.override.reason == "phase47 walkthrough override"' \
  "ordinary quota evidence export"

ABUSE_EXPORT="$(api_post '/v1/billing/denials/denial_phase47_abuse/evidence-export')"
require_jq "${ABUSE_EXPORT}" \
  '.denial.denialId == "denial_phase47_abuse" and (.redactions | length) >= 4 and (.auditRefs | index("audit_phase47_runtime")) and .effectiveLimitState.quota.restriction.sourceAuditRef == "audit_phase47_runtime"' \
  "abuse restriction evidence export"

CROSS_TENANT_CODE="$(curl --noproxy '*' -sS -o /dev/null -w '%{http_code}' "${DAEMON_URL}/v1/billing/denials/denial_phase47_other" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "X-Dope-Tenant-ID: ${TENANT_ID}")"
if [[ "${CROSS_TENANT_CODE}" != "404" ]]; then
  echo "phase47 walkthrough assertion failed: cross-tenant denial hidden, got HTTP ${CROSS_TENANT_CODE}" >&2
  exit 1
fi

UNAUTH_BODY="$(mktemp)"
UNAUTH_CODE="$(curl --noproxy '*' -sS -o "${UNAUTH_BODY}" -w '%{http_code}' "${DAEMON_URL}/v1/billing/quota-dashboard")"
if [[ "${UNAUTH_CODE}" != "401" && "${UNAUTH_CODE}" != "403" ]]; then
  echo "phase47 walkthrough assertion failed: unauthorized dashboard denied, got HTTP ${UNAUTH_CODE}" >&2
  cat "${UNAUTH_BODY}" >&2
  rm -f "${UNAUTH_BODY}"
  exit 1
fi
if jq -e 'has("tenantId") or has("sections") or has("plan")' "${UNAUTH_BODY}" >/dev/null; then
  echo "phase47 walkthrough assertion failed: unauthorized dashboard leaked partial data" >&2
  cat "${UNAUTH_BODY}" >&2
  rm -f "${UNAUTH_BODY}"
  exit 1
fi
rm -f "${UNAUTH_BODY}"

jq -n \
  --arg tenantId "${TENANT_ID}" \
  --arg runStatus "$(printf '%s' "${DASHBOARD}" | jq -r '[.sections[].items[] | select(.category=="run_launches")][0].status')" \
  --arg restrictionStatus "$(printf '%s' "${DASHBOARD}" | jq -r '[.sections[].items[] | select(.category=="runtime_tool_calls")][0].status')" \
  --arg runClass "$(printf '%s' "${RUN_DETAIL}" | jq -r '.classification')" \
  --arg abuseClass "$(printf '%s' "${ABUSE_DETAIL}" | jq -r '.classification')" \
  --argjson runRedactions "$(printf '%s' "${RUN_EXPORT}" | jq '.redactions | length')" \
  --argjson abuseRedactions "$(printf '%s' "${ABUSE_EXPORT}" | jq '.redactions | length')" \
  '{ok: true, tenantId: $tenantId, dashboard: {runStatus: $runStatus, restrictionStatus: $restrictionStatus}, denials: {ordinary: $runClass, abuse: $abuseClass}, evidenceExport: {ordinaryRedactions: $runRedactions, abuseRedactions: $abuseRedactions}}'
