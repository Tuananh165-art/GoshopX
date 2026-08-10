#!/usr/bin/env bash
set -euo pipefail

revision=${1:?Usage: $0 HELM_REVISION}
helm rollback goshopx "$revision" --namespace goshopx --wait --timeout 10m
kubectl -n goshopx rollout status deployment/kong --timeout=5m
kubectl -n goshopx rollout status deployment/graphql --timeout=5m
echo "Emergency rollback complete. Revert the corresponding GitOps image-promotion commit immediately."
