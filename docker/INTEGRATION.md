# Traefik with Hetzner Cloud DNS Provider Integration

## Purpose

This directory contains a **custom build of Traefik v3.5.3** with Hetzner Cloud DNS provider support for ACME DNS-01 challenges.

### Why Custom Build?

**Problem**: Official Traefik uses `go-acme/lego` which only supports the **old** Hetzner DNS API (dns.hetzner.com) via the "hetzner" provider. This requires a separate `HETZNER_DNS_TOKEN`.

**Solution**: Custom Traefik with `JonesH/lego-hcloud` fork adds support for the **new** Hetzner Cloud DNS API (api.hetzner.cloud/v1/zones). This allows using a **single** `HCLOUD_API_TOKEN` for:
- Terraform DNS zone management
- Traefik ACME DNS-01 challenges

### Architecture Benefits

**Before (Two Tokens)**:
```
Terraform DNS ← HCLOUD_API_TOKEN (api.hetzner.cloud/v1/zones)
Traefik ACME  ← HETZNER_DNS_TOKEN (dns.hetzner.com API)
```

**After (Single Token)**:
```
Terraform DNS ← HCLOUD_API_TOKEN
Traefik ACME  ← HCLOUD_API_TOKEN (same token!)
```

## Project Integration

### Location

```
nuroot/
├── docker/
│   └── traefik-hcloud/          # THIS DIRECTORY
│       ├── Dockerfile           # Multi-stage build
│       ├── build.sh            # Local build script
│       ├── push.sh             # Registry push script
│       ├── .env.example        # Registry configuration
│       └── INTEGRATION.md      # This file
├── terraform/
│   └── modules/
│       └── cloud-init-traefik-test/
│           ├── cloud-init.yml  # Uses custom image
│           └── variables.tf    # Registry variables
└── ansible/
    └── roles/
        └── traefik/
            └── templates/
                └── docker-compose.traefik.yml.j2  # Production deployment
```

### How It Integrates

1. **Build**: `./build.sh` creates local image `traefik-hcloud:v3.5.3-hcloud`
2. **Push**: `./push.sh` uploads to GHCR (or custom registry)
3. **Test**: Terraform cloud-init pulls from registry for testing
4. **Production**: Ansible role pulls from registry for deployment

## Build Process

### Local Build

```bash
cd docker/traefik-hcloud
./build.sh
```

**What it does**:
1. Clones Traefik v3.5.3 source
2. Replaces `go-acme/lego` with `JonesH/lego-hcloud` fork
3. Builds with Go 1.24 (required by lego fork)
4. Creates minimal Alpine runtime image (~216MB)

**Output**: `traefik-hcloud:v3.5.3-hcloud` and `traefik-hcloud:latest`

### Registry Push

```bash
# Configure registry (optional, defaults to GHCR)
cp .env.example .env
# Edit .env with your registry URL

# Login to GHCR (one-time)
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Push images
./push.sh
```

**Registry Configuration** (.env):
```bash
REGISTRY_URL=ghcr.io/jonesh       # Or your custom registry
TRAEFIK_IMAGE_NAME=traefik-hcloud
TRAEFIK_VERSION=v3.5.3-hcloud
```

## Testing Deployment

The custom image is used in test deployments via `terraform/modules/cloud-init-traefik-test/`:

```bash
cd terraform
terraform apply -var-file="environments/testing/terraform.tfvars" \
  -target=module.traefik_test \
  -auto-approve
```

**What it does**:
1. Creates Hetzner Cloud server with cloud-init
2. Cloud-init pulls custom Traefik image from registry
3. Configures Traefik with hcloud DNS provider
4. Tests ACME DNS-01 challenges with HCLOUD_API_TOKEN

**Test verification**:
```bash
# SSH to test server
ssh root@<test-server-ip>

# Check Traefik
docker logs traefik-test

# Verify hcloud provider loaded
docker logs traefik-test | grep '"provider":"hcloud"'

# Check ACME challenges
docker logs traefik-test | grep -i "acme\|challenge"
```

## Production Deployment

For production, update `ansible/roles/traefik/templates/docker-compose.traefik.yml.j2`:

```yaml
services:
  traefik:
    image: {{ traefik_registry }}/{{ traefik_image }}:{{ traefik_version }}
    # ... rest of config
    environment:
      - HCLOUD_TOKEN={{ hcloud_api_token }}
```

**Ansible variables** (in playbook or role defaults):
```yaml
traefik_registry: "{{ container_registry | default('ghcr.io/jonesh') }}"
traefik_image: "{{ traefik_image_name | default('traefik-hcloud') }}"
traefik_version: "{{ traefik_image_version | default('v3.5.3-hcloud') }}"
```

## Environment Variables

### In Traefik Container

**HCLOUD_TOKEN** (not HETZNER_API_KEY):
```yaml
environment:
  - HCLOUD_TOKEN=${hcloud_api_token}
```

**In Traefik static config** (traefik.yml):
```yaml
certificatesResolvers:
  letsencrypt:
    acme:
      dnsChallenge:
        provider: hcloud  # NOT "hetzner"
```

## CI/CD Integration

GitHub Actions automatically builds and pushes on:
- Push to `main` branch
- Changes in `docker/traefik-hcloud/**`
- Manual workflow dispatch

See `.github/workflows/traefik-hcloud.yml` for details.

## Maintenance

### Updating Traefik Version

1. Update Dockerfile `--branch` version:
   ```dockerfile
   RUN git clone --depth 1 --branch v3.5.4 https://github.com/traefik/traefik.git
   ```

2. Update version in build:
   ```dockerfile
   -ldflags="-s -w -X github.com/traefik/traefik/v3/pkg/version.Version=v3.5.4-hcloud"
   ```

3. Update `.env.example` and workflow defaults

4. Rebuild and push:
   ```bash
   ./build.sh
   ./push.sh
   ```

### Updating lego-hcloud Fork

If the fork gets updated:

1. Get new commit hash:
   ```bash
   # Visit: https://github.com/JonesH/lego-hcloud/commits/main
   ```

2. Update Dockerfile replace directive:
   ```dockerfile
   RUN go mod edit -replace github.com/go-acme/lego/v4=github.com/JonesH/lego-hcloud/v4@<new-commit-hash>
   ```

3. Rebuild

## Troubleshooting

### Build Failures

**"missing go.sum entry"**:
- Solution: `GOFLAGS="-mod=mod"` allows go.sum updates during build (already configured)

**"go version mismatch"**:
- Traefik v3.x requires Go 1.22
- lego-hcloud fork requires Go 1.24+
- Solution: Use Go 1.24 (configured in Dockerfile)

### Registry Issues

**"unauthorized" when pushing**:
```bash
# For GHCR
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# For custom registry
docker login registry.example.com
```

**"manifest unknown" when pulling**:
- Image not pushed yet, or wrong tag
- Check: `docker images | grep traefik-hcloud`

### ACME Failures

**401 Unauthorized with hcloud provider**:
- Check HCLOUD_TOKEN is set in container
- Verify token has DNS write permissions
- Test: `curl -H "Authorization: Bearer $TOKEN" https://api.hetzner.cloud/v1/zones`

**DNS propagation timeouts**:
- Increase `delayBeforeCheck` in traefik.yml
- Default 30s should be sufficient for Hetzner Cloud DNS

## References

- **Traefik ACME**: https://doc.traefik.io/traefik/https/acme/
- **lego-hcloud fork**: https://github.com/JonesH/lego-hcloud
- **Hetzner Cloud DNS API**: https://docs.hetzner.cloud/reference/cloud#dns
- **hcloud provider v1.54.0**: https://github.com/hetznercloud/terraform-provider-hcloud/releases/tag/v1.54.0
