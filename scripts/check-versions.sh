#!/usr/bin/env bash
# Verifies that every version declaration across the stack agrees on a
# single version. Prints the agreed version on stdout and exits non-zero
# on any mismatch. Used by CI (contract checks) and by the release
# workflow before packaging.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

version_from() {
  # $1: file, $2: sed expression extracting exactly one version
  sed -n "${2}" "$1" | head -n1
}

daemon_version=$(version_from "$ROOT_DIR/daemon/cmd/synca/main.go" 's/.*Version: "\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\)".*/\1/p')
cargo_version=$(version_from "$ROOT_DIR/desktop/src-tauri/Cargo.toml" 's/^version = "\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\)"/\1/p')
npm_version=$(version_from "$ROOT_DIR/desktop/package.json" 's/.*"version": "\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\)".*/\1/p')
tauri_version=$(version_from "$ROOT_DIR/desktop/src-tauri/tauri.conf.json" 's/.*"version": "\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\)".*/\1/p')

fail=0
for name_value in \
  "daemon:${daemon_version}" \
  "cargo:${cargo_version}" \
  "npm:${npm_version}" \
  "tauri:${tauri_version}"; do
  name="${name_value%%:*}"
  value="${name_value#*:}"
  if [ -z "${value}" ]; then
    echo "version missing in ${name}" >&2
    fail=1
  elif [ "${value}" != "${daemon_version}" ]; then
    echo "version mismatch: ${name}=${value}, daemon=${daemon_version}" >&2
    fail=1
  fi
done

# The Windows installer name embeds the version; a stale hardcoded name
# must fail the gate.
if ! grep -q "Synca_${daemon_version}_x64-setup.exe" "$ROOT_DIR/Makefile"; then
  echo "Makefile installer name does not contain version ${daemon_version}" >&2
  fail=1
fi

if [ "${fail}" -ne 0 ]; then
  exit 1
fi

echo "${daemon_version}"
