#!/usr/bin/env bash
set -euo pipefail

REPO="zhaodengfeng/DTSW"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="dtsw"

log() {
  printf '[dtsw-install] %s\n' "$*"
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

need_cmd uname
need_cmd curl
need_cmd mktemp

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

if [[ "$os" != "linux" ]]; then
  echo "DTSW installer currently supports Linux only (detected: $os)" >&2
  exit 1
fi

case "$arch" in
  x86_64|amd64)
    asset="dtsw-linux-amd64"
    ;;
  aarch64|arm64)
    asset="dtsw-linux-arm64"
    ;;
  *)
    echo "unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

url="https://github.com/${REPO}/releases/latest/download/${asset}"
out="$tmpdir/$BINARY_NAME"

log "downloading ${url}"
curl -fsSL "$url" -o "$out"
chmod +x "$out"

install_cmd=(install -m 0755 "$out" "$INSTALL_DIR/$BINARY_NAME")
if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
  need_cmd sudo
  log "installing to $INSTALL_DIR with sudo"
  sudo "${install_cmd[@]}"
else
  log "installing to $INSTALL_DIR"
  "${install_cmd[@]}"
fi

log "installed ${INSTALL_DIR}/${BINARY_NAME}"
log "next step: sudo dtsw setup"
