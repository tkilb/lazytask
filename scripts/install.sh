#!/usr/bin/env bash
# Installs the latest lazytask release for Linux or macOS.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/tkilb/lazytask/main/scripts/install.sh | sh
#
# Env vars:
#   INSTALL_DIR - where to put the binary (default: $HOME/bin)
#
# Linux and macOS only. See requirements.md Phase 4 — Windows is not,
# and will not be, supported by this script.
set -euo pipefail

REPO="tkilb/lazytask"
INSTALL_DIR="${INSTALL_DIR:-$HOME/bin}"

os="$(uname -s)"
case "$os" in
Linux) goos="linux" ;;
Darwin) goos="darwin" ;;
MINGW* | MSYS* | CYGWIN*)
	cat >&2 <<'EOF'
Sorry, lazytask does not support Windows, and it never will.
Grab yourself a real terminal on Linux or macOS (WSL counts) and try again.
EOF
	exit 1
	;;
*)
	echo "Unsupported OS: $os" >&2
	exit 1
	;;
esac

arch="$(uname -m)"
case "$arch" in
x86_64 | amd64) goarch="amd64" ;;
arm64 | aarch64) goarch="arm64" ;;
*)
	echo "Unsupported architecture: $arch" >&2
	exit 1
	;;
esac

echo "Detected $goos/$goarch."

echo "Looking up latest release..."
release_json="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")"
tag="$(printf '%s' "$release_json" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
if [ -z "$tag" ]; then
	echo "Could not determine the latest release tag." >&2
	exit 1
fi
version="${tag#v}"

archive="lazytask_${version}_${goos}_${goarch}.tar.gz"
url="https://github.com/${REPO}/releases/download/${tag}/${archive}"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

echo "Downloading $url..."
curl -fsSL "$url" -o "$tmp_dir/$archive"

echo "Extracting..."
tar -xzf "$tmp_dir/$archive" -C "$tmp_dir"

mkdir -p "$INSTALL_DIR"
install -m 755 "$tmp_dir/lazytask_${version}_${goos}_${goarch}/lazytask" "$INSTALL_DIR/lazytask"

echo "Installed lazytask $version to $INSTALL_DIR/lazytask"
case ":$PATH:" in
*":$INSTALL_DIR:"*) ;;
*) echo "Note: $INSTALL_DIR is not on your PATH. Add it to your shell profile." ;;
esac
