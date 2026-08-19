#!/usr/bin/env bash
#
# Kura one-line bootstrap.
#
#   curl -fsSL https://raw.githubusercontent.com/dopejs/kura/main/deploy/get.sh | bash
#
# Clones (or updates) the repo into a cache dir and runs deploy/install.sh.
# Requires: git, Rust (cargo). All install.sh flags/env pass through, e.g.:
#
#   curl -fsSL .../get.sh | DOPE_ENV=test bash
#   curl -fsSL .../get.sh | bash -s -- --no-service
#
# NOTE: there is no prebuilt-binary release channel yet, so this builds from
# source. Once CI publishes release artifacts, this script should prefer
# downloading a verified binary for the host platform and skip the toolchain.
#
set -euo pipefail

REPO_URL="${DOPE_REPO_URL:-https://github.com/dopejs/kura.git}"
REF="${DOPE_REF:-main}"
CACHE_DIR="${DOPE_SRC_DIR:-${HOME}/.cache/dope-agent/src}"

command -v git >/dev/null 2>&1 || { echo "error: git is required" >&2; exit 1; }

if [[ -d "${CACHE_DIR}/.git" ]]; then
  echo "==> updating source in ${CACHE_DIR}"
  git -C "${CACHE_DIR}" fetch --depth 1 origin "${REF}"
  git -C "${CACHE_DIR}" checkout -q FETCH_HEAD
else
  echo "==> cloning ${REPO_URL} (${REF}) -> ${CACHE_DIR}"
  mkdir -p "$(dirname "${CACHE_DIR}")"
  git clone --depth 1 --branch "${REF}" "${REPO_URL}" "${CACHE_DIR}"
fi

exec bash "${CACHE_DIR}/deploy/install.sh" "$@"
