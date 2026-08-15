#!/usr/bin/env bash
# miso installer
# Usage: curl -fsSL https://misojs.dev/install | bash

set -e

REPO="ekkolyth/miso"
INSTALL_DIR="${MISO_INSTALL_DIR:-$HOME/.local/bin}"

# ── Helpers ───────────────────────────────────────────────────────────────────

info()  { printf '\033[1;34m[miso]\033[0m %s\n' "$*"; }
ok()    { printf '\033[1;32m[miso]\033[0m %s\n' "$*"; }
fail()  { printf '\033[1;31m[miso]\033[0m error: %s\n' "$*" >&2; exit 1; }

# Stage alongside the destination then rename, so the swap is a real rename and
# never a copy over the live inode. Overwriting in place reuses the inode, which
# invalidates the kernel's cached ad-hoc signature on macOS and gets the binary
# SIGKILLed on next exec.
install_atomic() {
  src=$1
  dest=$2
  tmp=$(mktemp "$(dirname "$dest")/.miso.XXXXXX") || fail "Could not stage $dest"
  cp "$src" "$tmp" || { rm -f "$tmp"; fail "Could not stage $dest"; }
  chmod 755 "$tmp" || { rm -f "$tmp"; fail "Could not stage $dest"; }
  mv -f "$tmp" "$dest" || { rm -f "$tmp"; fail "Could not install $dest"; }
}

# ── Detect OS ─────────────────────────────────────────────────────────────────

OS="$(uname -s)"
case "$OS" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux"  ;;
  *)      fail "Unsupported operating system: $OS (Windows users: use npm install -g @ekkolyth/miso)" ;;
esac

# ── Detect arch ───────────────────────────────────────────────────────────────

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64 | amd64)   ARCH="amd64" ;;
  arm64  | aarch64) ARCH="arm64" ;;
  *)                fail "Unsupported architecture: $ARCH" ;;
esac

# ── Fetch latest version ──────────────────────────────────────────────────────

info "Fetching latest version..."

LATEST_URL="https://api.github.com/repos/${REPO}/releases/latest"

if command -v curl >/dev/null 2>&1; then
  TAG=$(curl -fsSL "$LATEST_URL" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
elif command -v wget >/dev/null 2>&1; then
  TAG=$(wget -qO- "$LATEST_URL" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
else
  fail "Neither curl nor wget is available. Please install one and try again."
fi

if [ -z "$TAG" ]; then
  fail "Could not determine the latest version. Check your internet connection."
fi

# Strip leading 'v'
VERSION="${TAG#v}"
info "Installing miso v${VERSION} (${OS}/${ARCH})..."

# ── Download archive ──────────────────────────────────────────────────────────

ARCHIVE="miso_${VERSION}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"

TMP_DIR="$(mktemp -d)"
TMP_ARCHIVE="${TMP_DIR}/${ARCHIVE}"

cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

info "Downloading ${DOWNLOAD_URL}..."

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$DOWNLOAD_URL" -o "$TMP_ARCHIVE"
else
  wget -qO "$TMP_ARCHIVE" "$DOWNLOAD_URL"
fi

# ── Extract binary ────────────────────────────────────────────────────────────

tar -xzf "$TMP_ARCHIVE" -C "$TMP_DIR"

# The binary inside the archive is named after the platform (e.g. miso-darwin-arm64)
EXTRACTED_BINARY="${TMP_DIR}/miso-${OS}-${ARCH}"
if [ ! -f "$EXTRACTED_BINARY" ]; then
  fail "Could not find binary 'miso-${OS}-${ARCH}' in archive. Archive contents: $(ls "$TMP_DIR")"
fi

# ── Install ───────────────────────────────────────────────────────────────────

mkdir -p "$INSTALL_DIR"
install_atomic "$EXTRACTED_BINARY" "${INSTALL_DIR}/miso"

# ── Download + install misox (standalone npx stand-in) ────────────────────────

MISOX_ARCHIVE="misox_${VERSION}_${OS}_${ARCH}.tar.gz"
MISOX_URL="https://github.com/${REPO}/releases/download/${TAG}/${MISOX_ARCHIVE}"
MISOX_ARCHIVE_PATH="${TMP_DIR}/${MISOX_ARCHIVE}"

info "Downloading ${MISOX_URL}..."

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$MISOX_URL" -o "$MISOX_ARCHIVE_PATH"
else
  wget -qO "$MISOX_ARCHIVE_PATH" "$MISOX_URL"
fi

tar -xzf "$MISOX_ARCHIVE_PATH" -C "$TMP_DIR"

MISOX_BINARY="${TMP_DIR}/misox-${OS}-${ARCH}"
if [ ! -f "$MISOX_BINARY" ]; then
  fail "Could not find binary 'misox-${OS}-${ARCH}' in archive. Archive contents: $(ls "$TMP_DIR")"
fi

install_atomic "$MISOX_BINARY" "${INSTALL_DIR}/misox"

# ── PATH hint ─────────────────────────────────────────────────────────────────

ok "miso v${VERSION} installed to ${INSTALL_DIR}/miso"
ok "misox v${VERSION} installed to ${INSTALL_DIR}/misox"

case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;  # already on PATH
  *)
    printf '\n'
    info "${INSTALL_DIR} is not in your PATH."
    info "Add the following to your shell config (~/.bashrc, ~/.zshrc, etc.):"
    printf '\n    export PATH="%s:$PATH"\n\n' "$INSTALL_DIR"
    ;;
esac

ok "Run 'miso --help' to get started."
