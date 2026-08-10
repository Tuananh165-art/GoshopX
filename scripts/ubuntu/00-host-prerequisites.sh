#!/usr/bin/env bash
set -euo pipefail

if [[ $EUID -eq 0 ]]; then
  echo "Run as a sudo-capable non-root user, not root." >&2
  exit 1
fi

source /etc/os-release
[[ ${ID:-} == "ubuntu" ]] || { echo "Ubuntu is required; detected ${ID:-unknown}." >&2; exit 1; }

sudo apt-get update
sudo apt-get install -y --no-install-recommends ca-certificates curl git jq gnupg openssl
sudo ufw status verbose 2>/dev/null || true

cat <<'EOF'
Host prerequisites installed.
Before exposing ingress, restrict the VPS firewall/security group to TCP 22,80,443.
Keep K3s API (6443), Kong Admin/Manager and all NodePorts private.
EOF
