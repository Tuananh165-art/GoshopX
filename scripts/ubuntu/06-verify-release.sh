#!/usr/bin/env bash
set -euo pipefail

namespace=${NAMESPACE:-goshopx}
kubectl -n "$namespace" get deploy,pods,svc
kubectl -n "$namespace" rollout status deployment/kong --timeout=5m
kubectl -n "$namespace" rollout status deployment/graphql --timeout=5m

pod=$(kubectl -n "$namespace" get pod -l app.kubernetes.io/component=kong -o jsonpath='{.items[0].metadata.name}')
kubectl -n "$namespace" exec "$pod" -- wget -q -O - http://127.0.0.1:8000/health
kubectl -n argocd get applications.argoproj.io goshopx-lab
echo "Verify Prometheus targets, Grafana datasource and Loki query through authenticated port-forward separately."
