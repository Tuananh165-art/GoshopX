#!/usr/bin/env bash
set -euo pipefail

secrets_file=${1:?Usage: $0 /secure/path/goshopx-secrets.env}
[[ -f "$secrets_file" ]] || { echo "Secret file not found: $secrets_file" >&2; exit 1; }
[[ $(stat -c '%a' "$secrets_file") -le 600 ]] || { echo "Secret file must be mode 600 or stricter." >&2; exit 1; }

required=(SECRET_KEY POSTGRES_USER POSTGRES_PASSWORD ACCOUNT_DATABASE_URL ORDER_DATABASE_URL PAYMENT_DATABASE_URL INVENTORY_DATABASE_URL NOTIFICATION_DATABASE_URL PRODUCT_DATABASE_URL MINIO_ACCESS_KEY MINIO_SECRET_KEY)
set -a
# shellcheck disable=SC1090
source "$secrets_file"
set +a
for key in "${required[@]}"; do [[ -n ${!key:-} ]] || { echo "Missing required secret: $key" >&2; exit 1; }; done

kubectl create namespace goshopx --dry-run=client -o yaml | kubectl apply -f -
kubectl -n goshopx create secret generic goshopx-runtime --from-env-file="$secrets_file" --dry-run=client -o yaml | kubectl apply -f -
echo "Applied goshopx-runtime without printing secret values."
