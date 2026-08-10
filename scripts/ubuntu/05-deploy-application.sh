#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"
: "${GHCR_IMAGE_REGISTRY:?Set e.g. ghcr.io/owner}"
: "${IMAGE_TAG:?Set an immutable Git commit SHA already published to GHCR}"
: "${RUNTIME_SECRET_NAME:=goshopx-runtime}"
: "${ENABLE_INFRA:=false}"
: "${INGRESS_HOST:=}"
: "${TLS_SECRET_NAME:=}"
: "${ENABLE_NODEPORT_DASHBOARDS:=false}"

kubectl -n goshopx get secret "$RUNTIME_SECRET_NAME" >/dev/null
args=(upgrade --install goshopx ./deploy/helm/goshopx --namespace goshopx --create-namespace
  --values ./deploy/helm/goshopx/values-lab.yaml
  --set "global.imageRegistry=$GHCR_IMAGE_REGISTRY"
  --set "global.runtimeSecretName=$RUNTIME_SECRET_NAME"
  --set "image.tag=$IMAGE_TAG"
  --set "infrastructure.enabled=$ENABLE_INFRA"
  --set "infrastructure.dashboards.nodePort=$ENABLE_NODEPORT_DASHBOARDS"
  --wait --timeout 12m)
if [[ -n "$INGRESS_HOST" ]]; then
  args+=(--set global.ingress.enabled=true --set "global.ingress.host=$INGRESS_HOST")
fi
if [[ -n "$TLS_SECRET_NAME" ]]; then args+=(--set "global.ingress.tlsSecretName=$TLS_SECRET_NAME"); fi
helm "${args[@]}"
kubectl -n goshopx rollout status deployment/kong --timeout=5m
kubectl -n goshopx rollout status deployment/graphql --timeout=5m
kubectl -n goshopx get pods,svc
