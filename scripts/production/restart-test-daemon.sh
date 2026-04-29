#!/usr/bin/env bash
set -euo pipefail

printf 'restart requested for test daemon on 127.0.0.1:19192\n'
if pgrep -f 'dope.*19192' >/dev/null 2>&1; then
  pkill -TERM -f 'dope.*19192'
  sleep 2
fi
make daemon-run-test
make daemon-test-status
