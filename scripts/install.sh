#!/usr/bin/env sh
set -eu
repo="IbodovElbek/tunnel-client"
version="${TUNNEL_VERSION:-latest}"
if [ "$version" = latest ]; then
  version="$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" | awk -F'"' '/tag_name/{print $4; exit}')"
fi
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in x86_64) arch=amd64;; arm64|aarch64) arch=arm64;; esac
asset="tunnel_${version#v}_${os}_${arch}.tar.gz"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
curl -fsSL "https://github.com/$repo/releases/download/$version/$asset" -o "$tmp/$asset"
tar -xzf "$tmp/$asset" -C "$tmp"
if [ -w /usr/local/bin ]; then install -m 0755 "$tmp/tunnel" /usr/local/bin/tunnel; else sudo install -m 0755 "$tmp/tunnel" /usr/local/bin/tunnel; fi
echo "Installed tunnel $version"
