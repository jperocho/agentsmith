#!/bin/sh
# agentsmith installer.
#
#   curl -fsSL https://raw.githubusercontent.com/jperocho/agentsmith/main/install.sh | sh
#
# Downloads the release asset matching this OS/arch, verifies its sha256 against
# the release checksums.txt, and installs the `agentsmith` binary onto PATH.
#
# Env overrides:
#   AGENTSMITH_VERSION   tag to install (default: latest release)
#   AGENTSMITH_BIN_DIR   install dir (default: /usr/local/bin, else ~/.local/bin)
set -eu

REPO="jperocho/agentsmith"

err() { echo "install: $*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

# --- detect platform ---------------------------------------------------------
os=$(uname -s)
arch=$(uname -m)
case "$os" in
  Linux)  os=linux ;;
  Darwin) os=darwin ;;
  *) err "unsupported OS: $os (use the Go install or a release archive)" ;;
esac
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) err "unsupported arch: $arch" ;;
esac

# --- pick a downloader -------------------------------------------------------
if have curl; then
  dl() { curl -fsSL "$1" -o "$2"; }
  fetch() { curl -fsSL "$1"; }
elif have wget; then
  dl() { wget -qO "$2" "$1"; }
  fetch() { wget -qO- "$1"; }
else
  err "need curl or wget"
fi
have tar || err "need tar"
have sha256sum || have shasum || err "need sha256sum or shasum"

# --- resolve version ---------------------------------------------------------
tag="${AGENTSMITH_VERSION:-}"
if [ -z "$tag" ]; then
  tag=$(fetch "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name":' | head -1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
  [ -n "$tag" ] || err "could not resolve latest release tag (no releases yet?)"
fi
ver=${tag#v} # archive names drop the leading v

asset="agentsmith_${ver}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$tag"

# --- download + verify -------------------------------------------------------
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
echo "Downloading $asset ($tag) ..."
dl "$base/$asset" "$tmp/$asset" || err "download failed: $base/$asset"
dl "$base/checksums.txt" "$tmp/checksums.txt" || err "checksums.txt download failed"

want=$(grep " $asset\$" "$tmp/checksums.txt" | awk '{print $1}')
[ -n "$want" ] || err "no checksum entry for $asset"
if have sha256sum; then
  got=$(sha256sum "$tmp/$asset" | awk '{print $1}')
else
  got=$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')
fi
[ "$want" = "$got" ] || err "checksum mismatch for $asset (want $want, got $got)"

# --- install -----------------------------------------------------------------
tar -xzf "$tmp/$asset" -C "$tmp"
[ -f "$tmp/agentsmith" ] || err "archive missing agentsmith binary"
chmod +x "$tmp/agentsmith"

bindir="${AGENTSMITH_BIN_DIR:-}"
if [ -z "$bindir" ]; then
  if [ -w /usr/local/bin ] 2>/dev/null; then
    bindir=/usr/local/bin
  else
    bindir="$HOME/.local/bin"
  fi
fi
mkdir -p "$bindir"
mv "$tmp/agentsmith" "$bindir/agentsmith"

echo "Installed agentsmith $tag -> $bindir/agentsmith"
case ":$PATH:" in
  *":$bindir:"*) ;;
  *) echo "note: $bindir is not on PATH — add it:  export PATH=\"$bindir:\$PATH\"" ;;
esac
