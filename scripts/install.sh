#!/usr/bin/env bash
# Kura first-release installer: builds the Rust daemon from this
# checkout and installs the `kura` binary onto PATH. Safe to re-run; it
# never touches an existing data directory's contents.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
RS_DIR="${ROOT_DIR}/crates"
BIN_DIR="${DOPE_BIN_DIR:-$HOME/.local/bin}"
ENV_NAME="${DOPE_ENV:-test}"

case "$ENV_NAME" in
  test) DATA_DIR="$HOME/.dope-test"; ADDR="127.0.0.1:19192" ;;
  prod) DATA_DIR="$HOME/.dope";      ADDR="127.0.0.1:19191" ;;
  *) echo "unsupported DOPE_ENV: $ENV_NAME (test|prod)" >&2; exit 1 ;;
esac

command -v cargo >/dev/null || { echo "cargo is required (https://rustup.rs)" >&2; exit 1; }

echo "building Kura CLI (release)..."
cargo build --release -p dope-cli --manifest-path "${RS_DIR}/Cargo.toml"

mkdir -p "$BIN_DIR"
install -m 0755 "${RS_DIR}/target/release/kura" "${BIN_DIR}/kura"
echo "installed ${BIN_DIR}/kura"

mkdir -p "$DATA_DIR"
echo "data directory: $DATA_DIR (created if absent; existing state untouched)"

VERSION_LINE="$("${BIN_DIR}/kura" --version 2>/dev/null || true)"
[ -n "$VERSION_LINE" ] && echo "binary: $VERSION_LINE"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "note: add $BIN_DIR to PATH" ;;
esac

cat <<NEXT

next steps:
  DOPE_ENV=$ENV_NAME kura          # start the daemon ($ADDR)
  curl -s http://$ADDR/healthz     # health check
  scripts/production/run-soak.sh   # release soak harness

upgrade later with: scripts/upgrade.sh
NEXT
