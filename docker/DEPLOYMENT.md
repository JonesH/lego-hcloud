# Deploying Custom Traefik with hcloud DNS

## Overview

This guide covers deploying Traefik v3.5.3 with hcloud DNS provider to replace the legacy DNS token dependency.

## Benefits

**Before** (Hybrid):
- Terraform: `HCLOUD_API_TOKEN` (Cloud API)
- Traefik ACME: `HETZNER_DNS_TOKEN` (DNS API)
- **2 tokens required**

**After** (Unified):
- Terraform: `HCLOUD_API_TOKEN` (Cloud API)
- Traefik ACME: `HCLOUD_API_TOKEN` (Cloud API)
- **1 token only** ✅

## Build Steps

```bash
cd /Users/jonah/nuroot/docker/traefik-hcloud

# Build custom image (5-10 minutes)
./build.sh

# Verify
docker run --rm traefik-hcloud:v3.5.3-hcloud version
```

## Update Ansible Role

### 1. Update docker-compose Template

```yaml
# ansible/roles/traefik/templates/docker-compose.traefik.yml.j2

services:
  traefik:
    # OLD: image: traefik:v3.0
    image: {{ traefik_custom_image | default('traefik:v3.0') }}

    environment:
      # OLD: HETZNER_API_KEY for dns.hetzner.com
      # NEW: HCLOUD_TOKEN for api.hetzner.cloud
      HCLOUD_TOKEN: "${HCLOUD_TOKEN}"
```

### 2. Update Static Config Template

```yaml
# ansible/roles/traefik/templates/traefik.yml.j2

certificatesResolvers:
  letsencrypt:
    acme:
      email: {{ acme_email }}
      storage: /data/acme.json
      dnsChallenge:
        provider: hcloud  # NEW: was 'hetzner'
        delayBeforeCheck: 30s
```

### 3. Update Secrets Loading

```yaml
# ansible/roles/traefik/tasks/main.yml

# Remove old DNS token loading
# - name: Load HETZNER_DNS_TOKEN
#   ...

# Use Cloud token for both infrastructure and ACME
- name: Set HCLOUD_TOKEN environment
  set_fact:
    hcloud_token: "{{ lookup('env', 'HCLOUD_API_TOKEN') }}"
```

## Deployment Workflow

### Test Build Locally

```bash
cd docker/traefik-hcloud
./build.sh

# Test version
docker run --rm traefik-hcloud:v3.5.3-hcloud version

# Should show: Traefik version v3.5.3
```

### Deploy to Testing (Cloud Server)

```bash
# Update astrojones.guru with custom image
cd ansible
ansible-playbook -i inventory-cloud.yml site.yml \
  -e "traefik_custom_image=traefik-hcloud:v3.5.3-hcloud" \
  -e "domain=astrojones.guru" \
  --tags=traefik

# Verify ACME works
curl -I https://test.astrojones.guru
# Should get valid SSL cert
```

### Deploy to Production (Root Server)

```bash
# After testing succeeds
ansible-playbook -i inventory-root.yml site.yml \
  -e "traefik_custom_image=traefik-hcloud:v3.5.3-hcloud" \
  -e "domain=jonaheidsick.de" \
  --tags=traefik

# Verify
curl -I https://traefik.jonaheidsick.de
```

## Token Cleanup

After successful deployment with hcloud provider:

```bash
# 1. Remove old DNS token from secrets
rm secrets/prod/hetzner-dns-token.enc  # No longer needed!

# 2. Update .env
# Remove: HETZNER_DNS_TOKEN=...
# Keep only: HCLOUD_API_TOKEN=...

# 3. Update Ansible inventory
# Remove all hetzner_dns_token references
```

## Rollback Plan

If ACME fails with custom build:

```bash
# Revert to official Traefik
ansible-playbook -i inventory-root.yml site.yml \
  -e "traefik_custom_image=traefik:v3.0" \
  -e "use_old_dns_api=true" \
  --tags=traefik

# Uses old HETZNER_DNS_TOKEN again
```

## Monitoring

```bash
# Watch ACME challenges
docker logs -f traefik | grep -i "acme\|hcloud\|certificate"

# Check certificate storage
docker exec traefik ls -la /data/acme.json

# Verify DNS provider
docker exec traefik traefik version
```

## CI/CD Integration (Future)

```yaml
# .github/workflows/build-traefik.yml
name: Build Custom Traefik

on:
  push:
    paths:
      - 'docker/traefik-hcloud/**'
  workflow_dispatch:

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Build image
        run: |
          cd docker/traefik-hcloud
          docker build -t ghcr.io/jonesh/traefik-hcloud:v3.5.3 .

      - name: Push to GHCR
        run: |
          echo "${{ secrets.GITHUB_TOKEN }}" | docker login ghcr.io -u ${{ github.actor }} --password-stdin
          docker push ghcr.io/jonesh/traefik-hcloud:v3.5.3
```

## Success Criteria

- ✅ Custom image builds successfully
- ✅ Traefik starts without errors
- ✅ ACME DNS-01 challenge completes
- ✅ Certificates issued via hcloud API
- ✅ Only HCLOUD_API_TOKEN needed
- ✅ No references to HETZNER_DNS_TOKEN

## Timeline

1. **Build** (10 min): Create custom image locally
2. **Test** (1 hour): Deploy to astrojones.guru, verify certificates
3. **Deploy** (5 min): Deploy to jonaheidsick.de production
4. **Cleanup** (5 min): Remove old DNS token

**Total**: ~1.5 hours to complete migration
