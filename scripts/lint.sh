#!/usr/bin/env bash
set -eo pipefail

GOLANGCI_LINT_VER="v1.37.1"
GOLANGCI_BIN_NAME="golangci-lint"
GOLANGCI_INSTALL_URL="https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh"
RETRY=3
TIMEOUT=300

repo_root="$( cd "$( dirname "${BASH_SOURCE[0]}" )/../" && pwd )"
repo_bin="${repo_root:?}/bin"
export PATH="${PATH}:${repo_bin}"

# install golangci lint if not available in PATH
if ! command -v "${GOLANGCI_BIN_NAME}" &> /dev/null; then
  mkdir -p "${repo_bin}"

  curl --silent --show-error --fail --location --retry "${RETRY}" \
    --connect-timeout "${TIMEOUT}" "${GOLANGCI_INSTALL_URL}" \
    | sh -s -- -b "${repo_bin}" "${GOLANGCI_LINT_VER}"

  export PATH="${PATH}:${repo_bin}"

  # validate that we can access golangci lint
  if ! command -v "${GOLANGCI_BIN_NAME}" &> /dev/null; then
    echo "failed to install golangci lint in '${repo_bin}'"
    exit 2
  fi
fi

cd "${repo_root}"
"${GOLANGCI_BIN_NAME}" run
