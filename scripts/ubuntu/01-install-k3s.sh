#!/usr/bin/env bash
set -euo pipefail

: "${K3S_VERSION:?Set an explicitly reviewed K3s release, e.g. v1.31.4+k3s1}"
installer=$(mktemp)
trap 'rm -f "$installer"' EXIT
curl --fail --show-error --silent --location --proto '=https' --tlsv1.2 \
  https://get.k3s.io -o "$installer"

if [[ -n ${K3S_INSTALLER_SHA256:-} ]]; then
  echo "${K3S_INSTALLER_SHA256}  ${installer}" | sha256sum --check --status
fi

sudo INSTALL_K3S_VERSION="$K3S_VERSION" INSTALL_K3S_EXEC='server --disable servicelb --disable metrics-server --write-kubeconfig-mode 600 --kubelet-arg=image-gc-high-threshold=70 --kubelet-arg=image-gc-low-threshold=55' \
  sh "$installer"

sudo install -d -m 0700 -o "$(id -u)" -g "$(id -g)" "$HOME/.kube"
sudo cp /etc/rancher/k3s/k3s.yaml "$HOME/.kube/config"
sudo chown "$(id -u):$(id -g)" "$HOME/.kube/config"
export KUBECONFIG="$HOME/.kube/config"
kubectl get nodes
