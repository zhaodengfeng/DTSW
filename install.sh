#!/usr/bin/env bash
set -euo pipefail

REPO="zhaodengfeng/DTSW"
RELEASE_BASE="https://github.com/${REPO}/releases/latest/download"
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

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
    return 0
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
    return 0
  fi
  echo "missing required command: sha256sum or shasum" >&2
  exit 1
}

need_cmd uname
need_cmd curl
need_cmd mktemp
need_cmd awk
need_cmd install

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

binary_url="${RELEASE_BASE}/${asset}"
checksums_url="${RELEASE_BASE}/checksums.txt"
out="$tmpdir/$BINARY_NAME"
checksums="$tmpdir/checksums.txt"

log "downloading ${binary_url}"
curl -fsSL "$binary_url" -o "$out"
log "downloading ${checksums_url}"
curl -fsSL "$checksums_url" -o "$checksums"

expected="$(awk -v name="$asset" '$2 == name || $2 == ("dist/" name) { print $1; exit }' "$checksums")"
if [[ -z "$expected" ]]; then
  echo "failed to find checksum for ${asset}" >&2
  exit 1
fi
actual="$(sha256_file "$out")"
if [[ "$actual" != "$expected" ]]; then
  echo "checksum mismatch for ${asset}" >&2
  echo "expected: $expected" >&2
  echo "actual:   $actual" >&2
  exit 1
fi
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
log "launching interactive setup..."
echo ""

if [[ ! -r /dev/tty ]]; then
  echo "no interactive TTY detected; start DTSW later in a terminal to continue setup" >&2
  exit 1
fi

if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
  sudo "${INSTALL_DIR}/${BINARY_NAME}" setup < /dev/tty > /dev/tty 2> /dev/tty
else
  "${INSTALL_DIR}/${BINARY_NAME}" setup < /dev/tty > /dev/tty 2> /dev/tty
fi
