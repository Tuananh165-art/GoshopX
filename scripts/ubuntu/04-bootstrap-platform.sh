#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"
: "${GRAFANA_ADMIN_PASSWORD:?Set a unique Grafana administrator password}"
: "${GITOPS_REPO_URL:?Set the HTTPS URL of this Git repository}"
: "${GHCR_IMAGE_REGISTRY:?Set e.g. ghcr.io/owner}"

bash scripts/lab/bootstrap-platform.sh
kubectl -n argocd rollout status deployment/argocd-server --timeout=10m
kubectl -n monitoring get pods
kubectl -n logging get pods
kubectl -n consul get pods
