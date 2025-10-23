#!/bin/bash
# Push traefik-hcloud image to container registry
# Supports GHCR (default) and custom registries

set -e

# Load registry config
if [ -f ".env" ]; then
    source .env
else
    # Defaults
    REGISTRY_URL="${REGISTRY_URL:-ghcr.io/jonesh}"
    TRAEFIK_IMAGE_NAME="${TRAEFIK_IMAGE_NAME:-traefik-hcloud}"
    TRAEFIK_VERSION="${TRAEFIK_VERSION:-v3.5.3-hcloud}"
fi

FULL_IMAGE="${REGISTRY_URL}/${TRAEFIK_IMAGE_NAME}"

echo "Pushing to registry: ${REGISTRY_URL}"
echo "Image: ${FULL_IMAGE}:${TRAEFIK_VERSION}"
echo ""

# Check if logged in (for GHCR)
if [[ "${REGISTRY_URL}" == ghcr.io* ]]; then
    echo "Checking GHCR authentication..."
    if ! docker login ghcr.io 2>&1 | grep -q "Login Succeeded\|already logged in"; then
        echo "Please login to GHCR first:"
        echo "  echo \$GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin"
        echo ""
        echo "Or create a GitHub PAT at: https://github.com/settings/tokens"
        echo "Scopes needed: write:packages, read:packages"
        exit 1
    fi
fi

# Tag if not already tagged
if ! docker images | grep -q "${FULL_IMAGE}"; then
    echo "Tagging images..."
    docker tag traefik-hcloud:${TRAEFIK_VERSION} ${FULL_IMAGE}:${TRAEFIK_VERSION}
    docker tag traefik-hcloud:latest ${FULL_IMAGE}:latest
fi

# Push both tags
echo "Pushing ${TRAEFIK_VERSION}..."
docker push ${FULL_IMAGE}:${TRAEFIK_VERSION}

echo "Pushing latest..."
docker push ${FULL_IMAGE}:latest

echo ""
echo "✓ Images pushed successfully!"
echo ""
echo "To use in deployments:"
echo "  image: ${FULL_IMAGE}:${TRAEFIK_VERSION}"
echo "  image: ${FULL_IMAGE}:latest"
