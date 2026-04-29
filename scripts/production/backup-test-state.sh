#!/usr/bin/env bash
set -euo pipefail

DOPE_DATA_DIR="${DOPE_DATA_DIR:-$HOME/.dope-test}"
BACKUP_DIR="${BACKUP_DIR:-$DOPE_DATA_DIR/backups}"
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

DEST="$BACKUP_DIR/daemon.sqlite.${TS}.bak"
cp "$DOPE_DATA_DIR/daemon.sqlite" "$DEST"
shasum -a 256 "$DOPE_DATA_DIR/daemon.sqlite" "$DEST"
printf 'backup_artifact=%s\n' "$DEST"
