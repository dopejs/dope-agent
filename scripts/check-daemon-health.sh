#!/usr/bin/env bash

set -euo pipefail

URL="${1:-}"
ATTEMPTS="${2:-5}"
SLEEP_SECONDS="${3:-1}"

if [[ -z "${URL}" ]]; then
  echo "usage: $0 <health-url> [attempts] [sleep-seconds]" >&2
  exit 1
fi

attempt=1
while (( attempt <= ATTEMPTS )); do
  if curl --noproxy '*' -fsS "${URL}"; then
    exit 0
  fi
  if (( attempt == ATTEMPTS )); then
    break
  fi
  sleep "${SLEEP_SECONDS}"
  attempt=$((attempt + 1))
done

echo "daemon health check failed for ${URL} after ${ATTEMPTS} attempts" >&2
exit 1
