#!/usr/bin/env bash
#
# Kura uninstaller. Removes the service and the installed binary.
# Data (the data directory: runs, events, config) is KEPT unless --purge.
#
# Overrides:
#   KURA_BIN_DIR   default ~/.local/bin
#   KURA_DATA_DIR  default ~/.kura (prod) / ~/.kura-test with --env test
#   --env test     target the test install (~/.kura-test)
#   --system       Linux: remove the system-wide systemd unit (sudo)
#   --purge        also delete the data directory  (DESTRUCTIVE, irreversible)
#
set -euo pipefail

SERVICE_LABEL="com.kurajs.kura-agent"
ENVN="prod"; SYSTEM_MODE=0; PURGE=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --env) ENVN="$2"; shift 2 ;;
    --system) SYSTEM_MODE=1; shift ;;
    --purge) PURGE=1; shift ;;
    -h|--help) sed -n '3,16p' "${BASH_SOURCE[0]}"; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 1 ;;
  esac
done

[[ "${ENVN}" == "test" ]] && DEFAULT_DATA="${HOME}/.kura-test" || DEFAULT_DATA="${HOME}/.kura"
KURA_BIN_DIR="${KURA_BIN_DIR:-${HOME}/.local/bin}"
KURA_DATA_DIR="${KURA_DATA_DIR:-${DEFAULT_DATA}}"
OS="$(uname -s)"
info() { printf '==> %s\n' "$*"; }

# ---- stop + remove the service ----
if [[ "${OS}" == "Darwin" ]]; then
  PLIST="${HOME}/Library/LaunchAgents/${SERVICE_LABEL}.plist"
  launchctl bootout "gui/$(id -u)/${SERVICE_LABEL}" 2>/dev/null || true
  [[ -f "${PLIST}" ]] && { rm -f "${PLIST}"; info "removed ${PLIST}"; }
elif [[ "${SYSTEM_MODE}" == "1" ]]; then
  sudo systemctl disable --now kura-agent.service 2>/dev/null || true
  sudo rm -f /etc/systemd/system/kura-agent.service && sudo systemctl daemon-reload || true
  info "removed system systemd unit"
else
  systemctl --user disable --now kura-agent.service 2>/dev/null || true
  rm -f "${HOME}/.config/systemd/user/kura-agent.service"
  systemctl --user daemon-reload 2>/dev/null || true
  info "removed systemd user unit"
fi

# ---- remove binaries ----
for binary in kura kura-tui; do
  [[ -e "${KURA_BIN_DIR}/${binary}" || -L "${KURA_BIN_DIR}/${binary}" ]] \
    && { rm -f "${KURA_BIN_DIR}/${binary}"; info "removed ${KURA_BIN_DIR}/${binary}"; }
done

# ---- data ----
if [[ "${PURGE}" == "1" ]]; then
  if [[ -d "${KURA_DATA_DIR}" ]]; then
    printf 'About to DELETE all data at %s — type the path to confirm: ' "${KURA_DATA_DIR}"
    read -r confirm
    [[ "${confirm}" == "${KURA_DATA_DIR}" ]] || { echo "mismatch; aborting purge."; exit 1; }
    rm -rf "${KURA_DATA_DIR}"; info "purged ${KURA_DATA_DIR}"
  fi
else
  info "data kept at ${KURA_DATA_DIR} (use --purge to delete)"
fi
info "uninstall complete."
