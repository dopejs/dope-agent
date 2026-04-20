#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
usage: install-mcp-catalog-entry.sh <catalog-entry-id> [options]

Install a bundled MCP catalog entry through the daemon API. Defaults to DOPE_ENV=test.

Options:
  --server-id <id>            Override the installed MCP server id
  --display-name <name>       Override the installed display name
  --sandbox-profile-id <id>   Override the sandbox profile id
  --command <command>         Override the stdio command
  --arg <value>               Append one stdio argument (repeatable)
  --endpoint <url>            Override the remote streamable-http endpoint
  --working-dir <dir>         Override the working directory
  --secret-ref <ref>          Append one secret ref (repeatable)
  --enable                    Force enabled=true
  --disable                   Force enabled=false
  --base-url <url>            Override the daemon base URL
  --token <token>             Use an existing bearer token instead of local pairing
  --help                      Show this help

Environment:
  DOPE_ENV                    Defaults to test
  DOPE_BASE_URL               Optional daemon base URL override
  DOPE_TOKEN                  Optional existing bearer token
  DOPE_PAIR_LABEL             Optional local pairing label
  DOPE_MCP_INSTALL_RAW_RESPONSE=1  Print the daemon JSON response after the summary
EOF
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "required command not found: $1" >&2
    exit 1
  fi
}

json_get() {
  local payload="$1"
  local path="$2"
  python3 - "$payload" "$path" <<'PY'
import json
import sys

doc = json.loads(sys.argv[1])
cur = doc
for part in sys.argv[2].split("."):
    if isinstance(cur, dict) and part in cur:
        cur = cur[part]
    else:
        sys.exit(1)
if isinstance(cur, str):
    print(cur)
else:
    print(json.dumps(cur, separators=(",", ":")))
PY
}

json_array() {
  python3 - "$@" <<'PY'
import json
import sys

print(json.dumps(sys.argv[1:], separators=(",", ":")))
PY
}

build_install_body() {
  local args_json="$1"
  local secret_refs_json="$2"
  python3 - \
    "${SERVER_ID}" \
    "${DISPLAY_NAME}" \
    "${ENABLED_VALUE}" \
    "${SANDBOX_PROFILE_ID}" \
    "${COMMAND_VALUE}" \
    "${ENDPOINT_VALUE}" \
    "${WORKING_DIR}" \
    "${args_json}" \
    "${secret_refs_json}" <<'PY'
import json
import sys

server_id, display_name, enabled_value, sandbox_profile_id, command_value, endpoint_value, working_dir, args_json, secret_refs_json = sys.argv[1:]

body = {}
if server_id:
    body["serverId"] = server_id
if display_name:
    body["displayName"] = display_name
if enabled_value:
    body["enabled"] = enabled_value == "true"
if sandbox_profile_id:
    body["sandboxProfileId"] = sandbox_profile_id
if command_value:
    body["command"] = command_value
args = json.loads(args_json)
if args:
    body["args"] = args
if endpoint_value:
    body["endpoint"] = endpoint_value
if working_dir:
    body["workingDir"] = working_dir
secret_refs = json.loads(secret_refs_json)
if secret_refs:
    body["secretRefs"] = secret_refs
print(json.dumps(body, separators=(",", ":")))
PY
}

request_json() {
  local method="$1"
  local url="$2"
  local body="${3:-}"
  local auth_token="${4:-}"
  local tmp
  tmp="$(mktemp)"
  local -a curl_args
  curl_args=(--noproxy "*" -sS -o "${tmp}" -w '%{http_code}' -X "${method}" -H 'Accept: application/json')
  if [[ -n "${body}" ]]; then
    curl_args+=(-H 'Content-Type: application/json' --data "${body}")
  fi
  if [[ -n "${auth_token}" ]]; then
    curl_args+=(-H "Authorization: Bearer ${auth_token}")
  fi
  RESPONSE_CODE="$(curl "${curl_args[@]}" "${url}")"
  RESPONSE_BODY="$(cat "${tmp}")"
  rm -f "${tmp}"
}

pair_local_token() {
  local pair_label="${DOPE_PAIR_LABEL:-mcp-catalog-script}"
  request_json POST "${BASE_URL}/v1/auth/pairings/start" "{\"mode\":\"local\",\"label\":\"${pair_label}\"}"
  if [[ "${RESPONSE_CODE}" != "201" ]]; then
    echo "pairing start failed with HTTP ${RESPONSE_CODE}: ${RESPONSE_BODY}" >&2
    exit 1
  fi
  local pairing_id pairing_code
  pairing_id="$(json_get "${RESPONSE_BODY}" "pairing.pairingId")"
  pairing_code="$(json_get "${RESPONSE_BODY}" "pairingCode")"
  request_json POST "${BASE_URL}/v1/auth/pairings/${pairing_id}/complete" "{\"code\":\"${pairing_code}\"}"
  if [[ "${RESPONSE_CODE}" != "200" ]]; then
    echo "pairing completion failed with HTTP ${RESPONSE_CODE}: ${RESPONSE_BODY}" >&2
    exit 1
  fi
  DOPE_TOKEN="$(json_get "${RESPONSE_BODY}" "accessToken")"
}

ENTRY_ID="${1:-}"
if [[ -z "${ENTRY_ID}" || "${ENTRY_ID}" == "--help" ]]; then
  usage
  exit 0
fi
shift

ENV_NAME="${DOPE_ENV:-test}"
case "${ENV_NAME}" in
  test)
    DEFAULT_BASE_URL="http://127.0.0.1:19192"
    ;;
  prod)
    DEFAULT_BASE_URL="http://127.0.0.1:19191"
    ;;
  *)
    echo "unsupported DOPE_ENV: ${ENV_NAME}" >&2
    exit 1
    ;;
esac

BASE_URL="${DOPE_BASE_URL:-${DEFAULT_BASE_URL}}"
DOPE_TOKEN="${DOPE_TOKEN:-}"
SERVER_ID=""
DISPLAY_NAME=""
SANDBOX_PROFILE_ID=""
COMMAND_VALUE=""
ENDPOINT_VALUE=""
WORKING_DIR=""
ENABLED_VALUE=""
ARGS=()
SECRET_REFS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --server-id)
      SERVER_ID="${2:-}"
      shift 2
      ;;
    --display-name)
      DISPLAY_NAME="${2:-}"
      shift 2
      ;;
    --sandbox-profile-id)
      SANDBOX_PROFILE_ID="${2:-}"
      shift 2
      ;;
    --command)
      COMMAND_VALUE="${2:-}"
      shift 2
      ;;
    --arg)
      ARGS+=("${2:-}")
      shift 2
      ;;
    --endpoint)
      ENDPOINT_VALUE="${2:-}"
      shift 2
      ;;
    --working-dir)
      WORKING_DIR="${2:-}"
      shift 2
      ;;
    --secret-ref)
      SECRET_REFS+=("${2:-}")
      shift 2
      ;;
    --enable)
      ENABLED_VALUE="true"
      shift
      ;;
    --disable)
      ENABLED_VALUE="false"
      shift
      ;;
    --base-url)
      BASE_URL="${2:-}"
      shift 2
      ;;
    --token)
      DOPE_TOKEN="${2:-}"
      shift 2
      ;;
    --help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

require_command curl
require_command python3

if [[ -z "${DOPE_TOKEN}" ]]; then
  pair_local_token
fi

request_json GET "${BASE_URL}/v1/mcp/catalog/${ENTRY_ID}" "" "${DOPE_TOKEN}"
if [[ "${RESPONSE_CODE}" != "200" ]]; then
  echo "catalog lookup failed for ${ENTRY_ID} with HTTP ${RESPONSE_CODE}: ${RESPONSE_BODY}" >&2
  exit 1
fi
SCRIPT_SUPPORTED="$(json_get "${RESPONSE_BODY}" "installSupport.scriptSupported")"
if [[ "${SCRIPT_SUPPORTED}" != "true" ]]; then
  echo "catalog entry ${ENTRY_ID} is not script-installable" >&2
  exit 1
fi

if (( ${#ARGS[@]} > 0 )); then
  ARGS_JSON="$(json_array "${ARGS[@]}")"
else
  ARGS_JSON="[]"
fi
if (( ${#SECRET_REFS[@]} > 0 )); then
  SECRET_REFS_JSON="$(json_array "${SECRET_REFS[@]}")"
else
  SECRET_REFS_JSON="[]"
fi
INSTALL_BODY="$(build_install_body "${ARGS_JSON}" "${SECRET_REFS_JSON}")"

request_json POST "${BASE_URL}/v1/mcp/catalog/${ENTRY_ID}/install" "${INSTALL_BODY}" "${DOPE_TOKEN}"

STATUS=""
CATALOG_ENTRY_ID=""
SERVER_ID_RESULT=""
AVAILABILITY_STATUS=""
AVAILABILITY_REASON=""
if STATUS="$(json_get "${RESPONSE_BODY}" "status" 2>/dev/null)"; then :; else STATUS="unknown"; fi
if CATALOG_ENTRY_ID="$(json_get "${RESPONSE_BODY}" "catalogEntryId" 2>/dev/null)"; then :; else CATALOG_ENTRY_ID="${ENTRY_ID}"; fi
if SERVER_ID_RESULT="$(json_get "${RESPONSE_BODY}" "serverId" 2>/dev/null)"; then :; else SERVER_ID_RESULT=""; fi
if AVAILABILITY_STATUS="$(json_get "${RESPONSE_BODY}" "availabilityStatus" 2>/dev/null)"; then :; else AVAILABILITY_STATUS=""; fi
if AVAILABILITY_REASON="$(json_get "${RESPONSE_BODY}" "availabilityReason" 2>/dev/null)"; then :; else AVAILABILITY_REASON=""; fi

echo "status=${STATUS}"
echo "catalogEntryId=${CATALOG_ENTRY_ID}"
if [[ -n "${SERVER_ID_RESULT}" ]]; then
  echo "serverId=${SERVER_ID_RESULT}"
fi
echo "httpStatus=${RESPONSE_CODE}"
if [[ -n "${AVAILABILITY_STATUS}" ]]; then
  echo "availabilityStatus=${AVAILABILITY_STATUS}"
fi
if [[ -n "${AVAILABILITY_REASON}" ]]; then
  echo "availabilityReason=${AVAILABILITY_REASON}"
fi

if [[ "${DOPE_MCP_INSTALL_RAW_RESPONSE:-0}" == "1" ]]; then
  echo "${RESPONSE_BODY}"
fi

if [[ "${RESPONSE_CODE}" -ge 400 || "${STATUS}" != "installed" ]]; then
  exit 1
fi
