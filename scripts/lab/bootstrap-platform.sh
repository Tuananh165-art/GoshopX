#!/usr/bin/env bash
set -euo pipefail

: "${GRAFANA_ADMIN_PASSWORD:?set a non-empty Grafana admin password}"
: "${GITOPS_REPO_URL:?set e.g. https://github.com/acme/GoshopX.git}"
: "${GHCR_IMAGE_REGISTRY:?set e.g. ghcr.io/acme}"

# Bootstrap platform services only. Application delivery remains Argo CD driven.
# Run from repository root after kubectl can reach the K3s cluster.
for ns in argocd consul monitoring logging; do
  kubectl create namespace "$ns" --dry-run=client -o yaml | kubectl apply -f -
done

helm repo add argo https://argoproj.github.io/argo-helm
helm repo add hashicorp https://helm.releases.hashicorp.com
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update

helm upgrade --install argocd argo/argo-cd --namespace argocd \
  --set server.replicas=1 --set repoServer.replicas=1 \
  --set controller.replicas=1 --set notifications.enabled=false \
  --set redis-ha.enabled=false --set redis.enabled=true \
  --set server.service.type=ClusterIP --wait --timeout 10m

helm upgrade --install consul hashicorp/consul --namespace consul \
  --values ops/consul/values-lab.yaml --wait --timeout 10m

helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring --values ops/observability/kube-prometheus-stack-values-lab.yaml \
  --set grafana.adminPassword="$GRAFANA_ADMIN_PASSWORD" \
  --wait --timeout 10m
helm upgrade --install loki grafana/loki --namespace logging \
  --values ops/observability/loki-values-lab.yaml --wait --timeout 10m
helm upgrade --install alloy grafana/alloy --namespace logging \
  --values ops/observability/alloy-values-lab.yaml --wait --timeout 10m

# Render placeholders only in memory; never commit a cluster-specific URL.
sed -e "s|https://github.com/REPLACE_OWNER/GoshopX.git|$GITOPS_REPO_URL|" \
    -e "s|ghcr.io/REPLACE_OWNER|$GHCR_IMAGE_REGISTRY|" \
    ops/gitops/argocd/applications/goshopx-lab.yaml | kubectl apply -f -
