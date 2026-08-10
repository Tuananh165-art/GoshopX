#!/usr/bin/env bash
set -euo pipefail

# Prefer the protected GitHub Actions workflow for normal releases. This is an
# operator fallback for a controlled Ubuntu builder with Docker Buildx.
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"
: "${GHCR_IMAGE_REGISTRY:?Set e.g. ghcr.io/owner}"
: "${IMAGE_TAG:?Set an immutable tag, normally the Git commit SHA}"
: "${REGISTRY_USERNAME:?Set registry user}"
: "${REGISTRY_TOKEN:?Set registry token with package write permission}"

command -v docker >/dev/null || { echo "Docker is required for manual build." >&2; exit 1; }
printf '%s' "$REGISTRY_TOKEN" | docker login ghcr.io --username "$REGISTRY_USERNAME" --password-stdin
unset REGISTRY_TOKEN

declare -A dockerfiles=(
  [account]=account [product]=product [order]=order [payment]=payment
  [inventory]=inventory [cart]=cart [notification]=notification [admin]=admin
  [graphql]=graphql [web]=web [recommender-server]=recommender
  [recommender-sync]=recommender [recommender-train]=recommender
)
for service in "${!dockerfiles[@]}"; do
  image="${GHCR_IMAGE_REGISTRY}/goshopx-${service}:${IMAGE_TAG}"
  docker buildx build --platform linux/amd64 --pull \
    --file "docker/services/${dockerfiles[$service]}.dockerfile" \
    --tag "$image" --push .
done

echo "Published immutable images with tag ${IMAGE_TAG}. Promote the same tag through GitOps before deployment."
