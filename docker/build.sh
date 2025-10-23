#!/bin/bash
# Build custom Traefik with hcloud DNS provider support

set -e

VERSION="v3.5.3-hcloud"
IMAGE_NAME="${1:-traefik-hcloud}"

echo "╔════════════════════════════════════════════════════════╗"
echo "║  Building Custom Traefik with hcloud DNS Provider     ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""
echo "Traefik version: v3.5.3"
echo "Custom lego fork: JonesH/lego-hcloud"
echo "Image tag: $IMAGE_NAME:$VERSION"
echo ""

# Build image
echo "Starting build (this will take 5-10 minutes)..."
docker build -t "$IMAGE_NAME:$VERSION" -t "$IMAGE_NAME:latest" .

echo ""
echo "✅ Build complete!"
echo ""
echo "Image details:"
docker images "$IMAGE_NAME:$VERSION" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}"
echo ""
echo "Test the image:"
echo "  docker run --rm $IMAGE_NAME:$VERSION version"
echo ""
echo "Next steps:"
echo "  1. Test locally: docker compose up -d"
echo "  2. Push to registry (optional):"
echo "     docker tag $IMAGE_NAME:$VERSION ghcr.io/jonesh/$IMAGE_NAME:$VERSION"
echo "     docker push ghcr.io/jonesh/$IMAGE_NAME:$VERSION"
echo "  3. Deploy to production via Ansible"
