#!/usr/bin/env bash
#
# DopeAgent installer.
#
# Builds the daemon from source, installs the `dope` binary, initializes the
# data directory, and (unless --no-service) registers a background service:
#   - macOS -> launchd user agent  (~/Library/LaunchAgents)
#   - Linux -> systemd service     (user by default, or --system with sudo)
#
# Run from anywhere; it locates the repo via its own path. Re-running is safe
# (idempotent): it rebuilds, refreshes the service, and never overwrites an
# existing config.json or data.
#
# Common overrides (env or flags):
#   DOPE_ENV=prod|test          --env <e>        default: prod
#   DOPE_BIN_DIR=<dir>          --bin-dir <d>    default: ~/.local/bin
#   DOPE_DATA_DIR=<dir>         --data-dir <d>   default: ~/.dope (prod) / ~/.dope-test (test)
#   DOPE_BIND_ADDR=<host:port>  --bind <a>       default: 127.0.0.1:19191 (prod) / :19192 (test)
#   DOPE_LOG_LEVEL=<level>                       default: info
#                               --system         Linux: install a system-wide systemd unit (sudo)
#                               --no-service     install binary + data dir only, skip service
#                               --uninstall      delegate to uninstall.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
RS_DIR="${REPO_ROOT}/crates"
SERVICE_LABEL="com.dopejs.dope-agent"

# ---- pretty output ---------------------------------------------------------
if [[ -t 1 ]]; then BOLD=$'\033[1m'; RED=$'\033[31m'; GRN=$'\033[32m'; YLW=$'\033[33m'; RST=$'\033[0m'
else BOLD=""; RED=""; GRN=""; YLW=""; RST=""; fi
info()  { printf '%s==>%s %s\n' "${GRN}" "${RST}" "$*"; }
warn()  { printf '%swarn:%s %s\n' "${YLW}" "${RST}" "$*" >&2; }
die()   { printf '%serror:%s %s\n' "${RED}" "${RST}" "$*" >&2; exit 1; }

# ---- args ------------------------------------------------------------------
SYSTEM_MODE=0
NO_SERVICE=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --env)      DOPE_ENV="$2"; shift 2 ;;
    --bin-dir)  DOPE_BIN_DIR="$2"; shift 2 ;;
    --data-dir) DOPE_DATA_DIR="$2"; shift 2 ;;
    --bind)     DOPE_BIND_ADDR="$2"; shift 2 ;;
    --system)   SYSTEM_MODE=1; shift ;;
    --no-service) NO_SERVICE=1; shift ;;
    --uninstall) exec "${SCRIPT_DIR}/uninstall.sh" "${@:2}" ;;
    -h|--help)  sed -n '3,33p' "${BASH_SOURCE[0]}"; exit 0 ;;
    *) die "unknown argument: $1 (try --help)" ;;
  esac
done

# ---- resolve config --------------------------------------------------------
DOPE_ENV="${DOPE_ENV:-prod}"
case "${DOPE_ENV}" in
  prod) DEFAULT_DATA="${HOME}/.dope";      DEFAULT_BIND="127.0.0.1:19191" ;;
  test) DEFAULT_DATA="${HOME}/.dope-test"; DEFAULT_BIND="127.0.0.1:19192" ;;
  *) die "DOPE_ENV must be 'prod' or 'test', got '${DOPE_ENV}'" ;;
esac
DOPE_BIN_DIR="${DOPE_BIN_DIR:-${HOME}/.local/bin}"
DOPE_DATA_DIR="${DOPE_DATA_DIR:-${DEFAULT_DATA}}"
DOPE_BIND_ADDR="${DOPE_BIND_ADDR:-${DEFAULT_BIND}}"
DOPE_LOG_LEVEL="${DOPE_LOG_LEVEL:-info}"
# Live connectors default off so the daemon boots without external creds and
# does not crash-loop on a stale/invalid token in config.json. Opt in with
# DOPE_CONNECTORS_DISCORD_ENABLED=true (and a valid token in config.json).
DOPE_DISCORD_ENABLED="${DOPE_CONNECTORS_DISCORD_ENABLED:-false}"
DOPE_LOG_DIR="${DOPE_DATA_DIR}/logs"
DOPE_BIN="${DOPE_BIN_DIR}/dope"

OS="$(uname -s)"
[[ "${OS}" == "Darwin" || "${OS}" == "Linux" ]] || die "unsupported OS: ${OS} (use Docker — see deploy/docker)"
[[ "${SYSTEM_MODE}" == "1" && "${OS}" == "Darwin" ]] && die "--system is Linux-only; macOS uses a launchd user agent"

# Health URL: the daemon may bind 0.0.0.0, but we probe via loopback.
HEALTH_HOST="${DOPE_BIND_ADDR%:*}"; [[ "${HEALTH_HOST}" == "0.0.0.0" || -z "${HEALTH_HOST}" ]] && HEALTH_HOST="127.0.0.1"
HEALTH_PORT="${DOPE_BIND_ADDR##*:}"
HEALTH_URL="http://${HEALTH_HOST}:${HEALTH_PORT}/healthz"

info "${BOLD}DopeAgent install${RST}  env=${DOPE_ENV}  bind=${DOPE_BIND_ADDR}"
echo "    binary   -> ${DOPE_BIN}"
echo "    data dir -> ${DOPE_DATA_DIR}"
echo "    health   -> ${HEALTH_URL}"

# ---- preflight -------------------------------------------------------------
command -v cargo >/dev/null 2>&1 || die "Rust toolchain not found. Install Rust (https://rustup.rs) and re-run."
[[ -f "${RS_DIR}/Cargo.toml" ]] || die "workspace source not found at ${RS_DIR}/Cargo.toml"

# ---- build (Rust release) --------------------------------------------------
DOPE_VERSION="$(git -C "${REPO_ROOT}" describe --tags --always --dirty 2>/dev/null || echo dev)"
info "building daemon (version ${DOPE_VERSION}) ..."
mkdir -p "${DOPE_BIN_DIR}"
TMP_BIN="$(mktemp "${DOPE_BIN_DIR}/.dope.XXXXXX")"
trap 'rm -f "${TMP_BIN}"' EXIT
( cd "${RS_DIR}" && cargo build --release -p dope-cli ) \
  || die "build failed — fix the error above and re-run."
cp -f "${RS_DIR}/target/release/dope-cli" "${TMP_BIN}"
chmod 0755 "${TMP_BIN}"
mv -f "${TMP_BIN}" "${DOPE_BIN}"   # atomic swap; a running service picks it up on restart
trap - EXIT
info "installed binary -> ${DOPE_BIN}"

# ---- data dir (daemon self-initializes config.json on first start) ---------
mkdir -p "${DOPE_DATA_DIR}" "${DOPE_LOG_DIR}"

# ---- service registration --------------------------------------------------
wait_health() {
  info "waiting for daemon health at ${HEALTH_URL} ..."
  for _ in $(seq 1 30); do
    if curl --noproxy '*' -fsS "${HEALTH_URL}" >/dev/null 2>&1; then
      info "${GRN}daemon is healthy.${RST}"; return 0
    fi
    sleep 1
  done
  warn "daemon did not report healthy within 30s. Check logs: ${DOPE_LOG_DIR}/daemon.err.log"
  return 1
}

render() { # render <template> <dest> ; substitutes @TOKEN@
  sed -e "s|@DOPE_BIN@|${DOPE_BIN}|g" \
      -e "s|@DOPE_ENV@|${DOPE_ENV}|g" \
      -e "s|@DOPE_DATA_DIR@|${DOPE_DATA_DIR}|g" \
      -e "s|@DOPE_BIND_ADDR@|${DOPE_BIND_ADDR}|g" \
      -e "s|@DOPE_VERSION@|${DOPE_VERSION}|g" \
      -e "s|@DOPE_LOG_LEVEL@|${DOPE_LOG_LEVEL}|g" \
      -e "s|@DOPE_DISCORD_ENABLED@|${DOPE_DISCORD_ENABLED}|g" \
      -e "s|@DOPE_LOG_DIR@|${DOPE_LOG_DIR}|g" \
      "$1" > "$2"
}

if [[ "${NO_SERVICE}" == "1" ]]; then
  warn "--no-service: skipping service registration."
  info "Run manually:  DOPE_ENV=${DOPE_ENV} DOPE_DATA_DIR=${DOPE_DATA_DIR} DOPE_BIND_ADDR=${DOPE_BIND_ADDR} ${DOPE_BIN}"
elif [[ "${OS}" == "Darwin" ]]; then
  PLIST="${HOME}/Library/LaunchAgents/${SERVICE_LABEL}.plist"
  info "registering launchd agent -> ${PLIST}"
  mkdir -p "${HOME}/Library/LaunchAgents"
  render "${SCRIPT_DIR}/launchd/${SERVICE_LABEL}.plist.template" "${PLIST}"
  launchctl bootout "gui/$(id -u)/${SERVICE_LABEL}" 2>/dev/null || true
  launchctl bootstrap "gui/$(id -u)" "${PLIST}"
  launchctl kickstart -k "gui/$(id -u)/${SERVICE_LABEL}" 2>/dev/null || true
  wait_health || true
else
  TEMPLATE="${SCRIPT_DIR}/systemd/dope-agent.service.template"
  if [[ "${SYSTEM_MODE}" == "1" ]]; then
    UNIT="/etc/systemd/system/dope-agent.service"
    info "registering system systemd unit -> ${UNIT} (sudo)"
    TMP_UNIT="$(mktemp)"
    SYSTEM_USER_BLOCK="User=${USER}\nGroup=$(id -gn)" \
    SYSTEMD_WANTED_BY="multi-user.target" \
      sed -e "s|@SYSTEM_USER_BLOCK@|User=${USER}\nGroup=$(id -gn)|" \
          -e "s|@SYSTEMD_WANTED_BY@|multi-user.target|" \
          "${TEMPLATE}" > "${TMP_UNIT}"
    render "${TMP_UNIT}" "${TMP_UNIT}.r"; mv "${TMP_UNIT}.r" "${TMP_UNIT}"
    sudo install -m 0644 "${TMP_UNIT}" "${UNIT}"; rm -f "${TMP_UNIT}"
    sudo systemctl daemon-reload
    sudo systemctl enable --now dope-agent.service
    wait_health || true
  else
    UNIT_DIR="${HOME}/.config/systemd/user"
    UNIT="${UNIT_DIR}/dope-agent.service"
    info "registering systemd user unit -> ${UNIT}"
    mkdir -p "${UNIT_DIR}"
    sed -e "s|@SYSTEM_USER_BLOCK@||" -e "s|@SYSTEMD_WANTED_BY@|default.target|" "${TEMPLATE}" > "${UNIT}.t"
    render "${UNIT}.t" "${UNIT}"; rm -f "${UNIT}.t"
    systemctl --user daemon-reload
    systemctl --user enable --now dope-agent.service
    # Keep the user service running after logout / across reboots (best effort).
    loginctl enable-linger "${USER}" 2>/dev/null \
      || warn "could not enable linger; service may stop on logout. Run: sudo loginctl enable-linger ${USER}"
    wait_health || true
  fi
fi

# ---- client guidance -------------------------------------------------------
echo
info "${BOLD}Done.${RST} The daemon listens on ${DOPE_BIND_ADDR}."
PATH_HINT=""
case ":${PATH}:" in *":${DOPE_BIN_DIR}:"*) ;; *) PATH_HINT=" (add ${DOPE_BIN_DIR} to PATH)";; esac
cat <<EOF

Connect a client (clients default to the test port 19192 — point them at this daemon):

  TUI:   DOPE_DAEMON_URL=http://${HEALTH_HOST}:${HEALTH_PORT} dope-tui
         (build once: cd ${REPO_ROOT}/crates && cargo build --release -p dope-tui)
  Web:   pnpm --dir ${REPO_ROOT}/web dev   # then set the daemon URL in the UI

$( [[ "${NO_SERVICE}" == "1" ]] && echo "(no service was registered: --no-service)" && exit 0
   echo "Manage the service:"
   [[ "${OS}" == "Darwin" ]] \
    && echo "  status: launchctl print gui/$(id -u)/${SERVICE_LABEL} | grep state" \
    && echo "  logs:   tail -f ${DOPE_LOG_DIR}/daemon.err.log" \
    && echo "  stop:   launchctl bootout gui/$(id -u)/${SERVICE_LABEL}" \
    || { if [[ "${SYSTEM_MODE}" == "1" ]]; then \
           echo "  status: systemctl status dope-agent"; echo "  logs:   journalctl -u dope-agent -f"; \
         else \
           echo "  status: systemctl --user status dope-agent"; echo "  logs:   journalctl --user -u dope-agent -f"; \
         fi; } )
  health: curl ${HEALTH_URL}
  remove: ${SCRIPT_DIR}/uninstall.sh
${PATH_HINT:+
note:${PATH_HINT}}
EOF
