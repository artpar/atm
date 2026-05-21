#!/usr/bin/env sh
set -eu

repo="artpar/atm"
install_dir="${ATM_INSTALL_DIR:-"$HOME/.local/bin"}"
version="${ATM_VERSION:-latest}"

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "atm install: missing required command: $1" >&2
    exit 1
  fi
}

need curl
need tar

os="$(uname -s)"
case "$os" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
  *)
    echo "atm install: unsupported OS: $os" >&2
    exit 1
    ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "atm install: unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

if [ "$version" = "latest" ]; then
  version="$(
    curl -fsSL -H "Accept: application/vnd.github+json" "https://api.github.com/repos/$repo/releases/latest" |
      sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' |
      head -n 1
  )"
  if [ -z "$version" ]; then
    echo "atm install: could not resolve latest release" >&2
    exit 1
  fi
fi

case "$version" in
  v*) tag="$version"; semver="${version#v}" ;;
  *) tag="v$version"; semver="$version" ;;
esac

tmp="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT INT TERM

asset="atm_${semver}_${os}_${arch}.tar.gz"
base_url="https://github.com/$repo/releases/download/$tag"

echo "Installing atm $tag for $os/$arch..."
curl -fsSLo "$tmp/$asset" "$base_url/$asset"
curl -fsSLo "$tmp/checksums.txt" "$base_url/checksums.txt"

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$tmp" && grep "  $asset\$" checksums.txt | sha256sum -c - >/dev/null)
elif command -v shasum >/dev/null 2>&1; then
  expected="$(grep "  $asset\$" "$tmp/checksums.txt" | awk '{print $1}')"
  actual="$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')"
  if [ "$expected" != "$actual" ]; then
    echo "atm install: checksum mismatch for $asset" >&2
    exit 1
  fi
else
  echo "atm install: sha256sum or shasum is required to verify downloads" >&2
  exit 1
fi

tar -xzf "$tmp/$asset" -C "$tmp"
mkdir -p "$install_dir"
if command -v install >/dev/null 2>&1; then
  install -m 0755 "$tmp/atm" "$install_dir/atm"
else
  cp "$tmp/atm" "$install_dir/atm"
  chmod 0755 "$install_dir/atm"
fi

echo "Installed atm to $install_dir/atm"
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) echo "Add $install_dir to PATH to run atm from any shell." ;;
esac
