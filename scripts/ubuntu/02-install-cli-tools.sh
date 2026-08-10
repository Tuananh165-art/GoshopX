#!/usr/bin/env bash
set -euo pipefail

: "${HELM_VERSION:=v3.16.4}"
: "${ARGOCD_VERSION:=v2.13.3}"
arch=$(dpkg --print-architecture)
case "$arch" in amd64) helm_arch=amd64; argocd_arch=linux-amd64;; arm64) helm_arch=arm64; argocd_arch=linux-arm64;; *) echo "Unsupported architecture: $arch" >&2; exit 1;; esac
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

curl --fail --show-error --silent --location --proto '=https' --tlsv1.2 \
  "https://get.helm.sh/helm-${HELM_VERSION}-linux-${helm_arch}.tar.gz" -o "$tmpdir/helm.tgz"
tar -xzf "$tmpdir/helm.tgz" -C "$tmpdir"
sudo install -m 0755 "$tmpdir/linux-${helm_arch}/helm" /usr/local/bin/helm
curl --fail --show-error --silent --location --proto '=https' --tlsv1.2 \
  "https://github.com/argoproj/argo-cd/releases/download/${ARGOCD_VERSION}/argocd-${argocd_arch}" -o "$tmpdir/argocd"
sudo install -m 0755 "$tmpdir/argocd" /usr/local/bin/argocd
helm version --short
argocd version --client --short
