#!/usr/bin/env bash
#
# get-pdfium.sh — download a prebuilt PDFium shared library.
#
# PDFium is NOT part of the folio module; folio loads it at runtime. This
# script fetches a prebuilt binary from bblanchon/pdfium-binaries and places
# it in <repo>/pdfium/ — the location folio's auto-detection searches.
#
# Usage:
#   scripts/get-pdfium.sh                  # latest release, current platform
#   scripts/get-pdfium.sh chromium/8009    # pin a specific release tag
#
# Environment:
#   PDFIUM_DIR      output directory (default: <repo>/pdfium)
#   PDFIUM_VERSION  release tag (same as the positional argument)
#
# Requires: curl, tar. (Go is optional — used only to honour GOOS/GOARCH.)
#
set -euo pipefail

REPO="bblanchon/pdfium-binaries"

usage() {
  sed -n '2,16p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}
case "${1:-}" in -h|--help) usage 0 ;; esac

for tool in curl tar; do
  command -v "$tool" >/dev/null 2>&1 || { echo "error: '$tool' is required" >&2; exit 1; }
done

# --- detect target platform (GOOS/GOARCH if Go is present, else uname) ------
if command -v go >/dev/null 2>&1; then
  GOOS=$(go env GOOS)
  GOARCH=$(go env GOARCH)
else
  case "$(uname -s)" in
    Darwin) GOOS=darwin ;;
    Linux) GOOS=linux ;;
    MINGW*|MSYS*|CYGWIN*) GOOS=windows ;;
    *) echo "error: unsupported OS: $(uname -s)" >&2; exit 1 ;;
  esac
  case "$(uname -m)" in
    arm64|aarch64) GOARCH=arm64 ;;
    x86_64|amd64) GOARCH=amd64 ;;
    i386|i686) GOARCH=386 ;;
    armv6l|armv7l) GOARCH=arm ;;
    *) echo "error: unsupported arch: $(uname -m)" >&2; exit 1 ;;
  esac
fi

# --- map platform -> asset name + library filename --------------------------
case "${GOOS}/${GOARCH}" in
  darwin/arm64) ASSET="pdfium-mac-arm64.tgz"; LIB="libpdfium.dylib" ;;
  darwin/amd64) ASSET="pdfium-mac-x64.tgz"; LIB="libpdfium.dylib" ;;
  linux/amd64) ASSET="pdfium-linux-x64.tgz"; LIB="libpdfium.so" ;;
  linux/arm64) ASSET="pdfium-linux-arm64.tgz"; LIB="libpdfium.so" ;;
  linux/386) ASSET="pdfium-linux-x86.tgz"; LIB="libpdfium.so" ;;
  linux/arm) ASSET="pdfium-linux-arm.tgz"; LIB="libpdfium.so" ;;
  windows/amd64) ASSET="pdfium-win-x64.tgz"; LIB="pdfium.dll" ;;
  windows/arm64) ASSET="pdfium-win-arm64.tgz"; LIB="pdfium.dll" ;;
  windows/386) ASSET="pdfium-win-x86.tgz"; LIB="pdfium.dll" ;;
  *) echo "error: no prebuilt PDFium for ${GOOS}/${GOARCH}" >&2; exit 1 ;;
esac

# --- resolve release tag ----------------------------------------------------
VERSION="${1:-${PDFIUM_VERSION:-latest}}"
if [ "$VERSION" = "latest" ]; then
  BASE="https://github.com/${REPO}/releases/latest/download"
else
  BASE="https://github.com/${REPO}/releases/download/${VERSION}"
fi
URL="${BASE}/${ASSET}"

# --- output dir (default: <repo>/pdfium, i.e. parent of this script's dir) --
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
OUT_DIR="${PDFIUM_DIR:-${ROOT_DIR}/pdfium}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo ">> target : ${GOOS}/${GOARCH}"
echo ">> asset  : ${ASSET}"
echo ">> source : ${URL}"
echo ">> output : ${OUT_DIR}/"

echo ">> downloading…"
curl -fL --progress-bar --max-time 600 -o "$TMP/p.tgz" "$URL"

echo ">> extracting…"
tar xzf "$TMP/p.tgz" -C "$TMP"

SRC_LIB="$TMP/lib/$LIB"
if [ ! -f "$SRC_LIB" ]; then
  echo "error: $LIB not found in archive" >&2
  exit 1
fi

mkdir -p "$OUT_DIR"
cp "$SRC_LIB" "$OUT_DIR/$LIB"

# Preserve license + attribution (PDFium is BSD 3-Clause).
[ -f "$TMP/LICENSE" ] && cp "$TMP/LICENSE" "$OUT_DIR/"
[ -d "$TMP/licenses" ] && cp -R "$TMP/licenses" "$OUT_DIR/"
[ -f "$TMP/VERSION" ] && cp "$TMP/VERSION" "$OUT_DIR/"

echo ">> done: $(cd "$OUT_DIR" && pwd)/$LIB"
echo ""
echo "folio auto-detects it from ./pdfium/ when run from the repo root."
echo "To point elsewhere:  PDFIUM_LIB=$(cd "$OUT_DIR" && pwd)/$LIB  (or --lib)"
