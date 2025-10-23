# Custom Traefik with Hetzner Cloud DNS Provider

Build Traefik v3.5.3 with support for Hetzner Cloud DNS API (`api.hetzner.cloud`) instead of legacy DNS API (`dns.hetzner.com`).

## Why Custom Build?

**Problem**: Official Traefik uses lego's "hetzner" provider which only supports the OLD Hetzner DNS API.

**Solution**: Use custom lego fork with hcloud DNS provider for Cloud API support.

**Benefits**:
- ✅ Single token (`HCLOUD_API_TOKEN`) for infrastructure + DNS + ACME
- ✅ Modern Cloud DNS API
- ✅ No dependency on legacy DNS API
- ✅ Consolidates all Hetzner operations

## Build Process

### Automated CI/CD (Recommended)

**GitHub Actions automatically builds and pushes images to GHCR.**

Just push changes to the `docker/` directory:
```bash
git add docker/
git commit -m "build: update Traefik configuration"
git push origin master
```

The workflow will:
- Build for multiple platforms (amd64, arm64)
- Push to `ghcr.io/jonesh/traefik-hcloud`
- Tag with version and `latest`
- Generate build attestation

📖 **See [CICD.md](./CICD.md) for complete automation documentation**

### Manual Local Build

If you need to build locally for testing:

**Prerequisites:**
- Docker installed
- 10-15 minutes build time
- ~500MB disk space during build

### Quick Start

```bash
cd /Users/jonah/nuroot/docker/traefik-hcloud

# Build the image
./build.sh

# Test it works
docker run --rm traefik-hcloud:latest version
```

### Expected Output
```
Traefik version v3.5.3
Built with custom lego fork (hcloud DNS support)
```

## Deployment

### Option A: Local Image (Testing)

```yaml
# ansible/roles/traefik/templates/docker-compose.traefik.yml.j2
services:
  traefik:
    image: traefik-hcloud:v3.5.3-hcloud  # Custom build
    # ... rest of config
```

### Option B: Use Pre-Built from GHCR (Production)

**Recommended:** Use automatically built images from GitHub Container Registry:

```yaml
# docker-compose.yml
services:
  traefik:
    image: ghcr.io/jonesh/traefik-hcloud:v3.5.3-hcloud  # Pre-built via CI/CD
    # ... rest of config
```

**Or manually push local build:**

```bash
# Tag for GitHub Container Registry
docker tag traefik-hcloud:v3.5.3-hcloud ghcr.io/jonesh/traefik-hcloud:v3.5.3-hcloud

# Login
echo $GITHUB_TOKEN | docker login ghcr.io -u jonesh --password-stdin

# Push
docker push ghcr.io/jonesh/traefik-hcloud:v3.5.3-hcloud
```

## Configuration Changes

### Update Traefik Static Config

```yaml
# ansible/roles/traefik/templates/traefik.yml.j2
certificatesResolvers:
  letsencrypt:
    acme:
      email: {{ acme_email }}
      storage: /data/acme.json
      dnsChallenge:
        provider: hcloud  # NEW: Use hcloud instead of hetzner
        delayBeforeCheck: 30s
```

### Update Environment Variables

```yaml
# ansible/roles/traefik/templates/docker-compose.traefik.yml.j2
environment:
  # OLD: HETZNER_API_KEY for dns.hetzner.com
  # NEW: HCLOUD_TOKEN for api.hetzner.cloud
  HCLOUD_TOKEN: "${HCLOUD_TOKEN}"
```

### Update Secrets

```bash
# No longer need separate DNS token!
# Remove from docker-compose:
# HETZNER_API_KEY: "${HETZNER_DNS_TOKEN}"

# Just use Cloud token
HCLOUD_TOKEN: "${HCLOUD_API_TOKEN}"
```

## Testing

### Local Test

```bash
# Build image
./build.sh

# Test ACME with your fork
docker run --rm \
  -e HCLOUD_TOKEN="your_token" \
  traefik-hcloud:latest \
  --configFile=/dev/null \
  --providers.docker=false \
  --entrypoints.web.address=:80 \
  --certificatesresolvers.test.acme.email=admin@example.com \
  --certificatesresolvers.test.acme.storage=/tmp/acme.json \
  --certificatesresolvers.test.acme.dnschallenge.provider=hcloud
```

### Verify Provider Available

```bash
# Check hcloud provider is compiled in
docker run --rm traefik-hcloud:latest --help | grep -i hcloud
```

## Migration Path

### Phase 1: Build & Test (Local)
```bash
cd docker/traefik-hcloud
./build.sh
# Test locally
```

### Phase 2: Deploy to Testing (astrojones.guru)
```bash
# Update ansible role to use custom image
cd ansible
ansible-playbook -i inventory-cloud.yml site.yml \
  -e "traefik_image=traefik-hcloud:v3.5.3-hcloud" \
  --tags=traefik
```

### Phase 3: Deploy to Production (jonaheidsick.de)
```bash
# After successful testing
ansible-playbook -i inventory-root.yml site.yml \
  -e "traefik_image=traefik-hcloud:v3.5.3-hcloud" \
  --tags=traefik
```

## Verification

```bash
# Check Traefik version
docker exec traefik traefik version

# Check ACME provider
docker logs traefik | grep -i "hcloud\|acme"

# Verify certificate renewal works
# (Wait for next renewal or force with: rm /opt/docker-volumes/traefik/data/acme.json)
```

## Troubleshooting

### Build Fails

```bash
# Check Go version
docker run --rm golang:1.22-alpine go version

# Manual build for debugging
docker build --progress=plain --no-cache -t traefik-hcloud:debug .
```

### Provider Not Found

```bash
# Verify lego fork was used
docker run --rm traefik-hcloud:latest version
# Should show custom build info
```

### ACME Challenge Fails

```bash
# Check token is correct
docker logs traefik | grep "hcloud"

# Verify API access from container
docker exec traefik wget -qO- --header="Authorization: Bearer $TOKEN" \
  https://api.hetzner.cloud/v1/zones
```

## Maintenance

### Rebuild After lego Updates

```bash
cd docker/traefik-hcloud

# Update to latest lego-hcloud
# Edit Dockerfile: @codex/implement-hetzner-cloud-dns-provider -> @main

# Rebuild
./build.sh
```

### Upgrade Traefik Version

```bash
# Edit Dockerfile: v3.5.3 -> v3.6.0
# Rebuild
./build.sh traefik-hcloud v3.6.0-hcloud
```

## Size Comparison

| Image | Size | ACME Provider |
|-------|------|---------------|
| traefik:v3.0 | 100MB | hetzner (old API) |
| **traefik-hcloud:v3.5.3** | **~150MB** | **hcloud (Cloud API)** ✅ |

+50MB for single-token consolidation is acceptable.
