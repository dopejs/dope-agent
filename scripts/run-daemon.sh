#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
RS_DIR="${ROOT_DIR}/crates"
ENV_NAME="${1:-test}"

case "${ENV_NAME}" in
  test)
    export KURA_ENV=test
    export KURA_CONNECTORS_DISCORD_ENABLED="${KURA_CONNECTORS_DISCORD_ENABLED:-false}"
    ;;
  prod)
    export KURA_ENV=prod
    ;;
  *)
    echo "usage: $0 [test|prod]" >&2
    exit 1
    ;;
esac

cargo build --release -p kura-cli --manifest-path "${RS_DIR}/Cargo.toml"
exec "${RS_DIR}/target/release/kura" daemon run
