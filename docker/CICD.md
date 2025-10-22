# CI/CD Pipeline for Custom Traefik Build

## Overview

Automated GitHub Actions workflow to build and push the custom Traefik image with Hetzner Cloud DNS provider support to GitHub Container Registry (GHCR).

## Workflow: `.github/workflows/docker-build.yml`

### Triggers

1. **Automatic Build**
   - Pushes to `master` branch when files in `docker/` change
   - Ensures image stays up-to-date with code changes

2. **Manual Dispatch**
   - Navigate to Actions → Build and Push Docker Image → Run workflow
   - Optional: Specify custom tag version

3. **Release Tags**
   - Automatically triggered when a new release is published
   - Tags image with the release version

### What It Does

1. **Multi-Platform Build**
   - Builds for `linux/amd64` and `linux/arm64`
   - Ensures compatibility across different architectures

2. **Push to GHCR**
   - Registry: `ghcr.io/jonesh/traefik-hcloud`
   - No additional secrets needed (uses `GITHUB_TOKEN`)

3. **Automatic Tagging**
   - `v3.5.3-hcloud` (default version)
   - `latest` (always points to most recent build)
   - `master-<sha>` (commit-specific for traceability)
   - Custom tags via manual dispatch

4. **Build Attestation**
   - Generates provenance attestation
   - Verifiable supply chain security
   - Automatic signature with GitHub's signing infrastructure

5. **Layer Caching**
   - Uses GitHub Actions cache
   - Significantly faster subsequent builds
   - Reduces build time from ~10min to ~3min

## Usage

### Automated Workflow

Just push changes to `docker/` directory:

```bash
# Make changes to Dockerfile or build scripts
vim docker/Dockerfile

# Commit and push
git add docker/Dockerfile
git commit -m "build: update Traefik to v3.5.4"
git push origin master

# Workflow automatically triggers and builds new image
```

### Manual Workflow Dispatch

Run workflow manually with custom version:

1. Go to: https://github.com/jonesh/lego-hcloud/actions/workflows/docker-build.yml
2. Click "Run workflow"
3. Optional: Enter custom tag (e.g., `v3.5.4-hcloud`)
4. Click "Run workflow"

### Using Built Images

#### Pull from GHCR

```bash
# Pull specific version
docker pull ghcr.io/jonesh/traefik-hcloud:v3.5.3-hcloud

# Pull latest
docker pull ghcr.io/jonesh/traefik-hcloud:latest

# Pull specific commit (for debugging)
docker pull ghcr.io/jonesh/traefik-hcloud:master-a1b2c3d
```

#### Update docker-compose.yml

```yaml
services:
  traefik:
    image: ghcr.io/jonesh/traefik-hcloud:v3.5.3-hcloud
    environment:
      - HCLOUD_TOKEN=${HCLOUD_TOKEN}
    # ... rest of config
```

#### Update Terraform/Ansible

**Terraform cloud-init:**
```hcl
variable "traefik_image" {
  default = "ghcr.io/jonesh/traefik-hcloud:v3.5.3-hcloud"
}
```

**Ansible role:**
```yaml
traefik_image: "ghcr.io/jonesh/traefik-hcloud:v3.5.3-hcloud"
```

## Image Tags Explained

| Tag | Purpose | Example | When to Use |
|-----|---------|---------|-------------|
| `v3.5.3-hcloud` | Specific version | `ghcr.io/jonesh/traefik-hcloud:v3.5.3-hcloud` | Production (pinned version) |
| `latest` | Most recent build | `ghcr.io/jonesh/traefik-hcloud:latest` | Testing, development |
| `master-<sha>` | Commit-specific | `ghcr.io/jonesh/traefik-hcloud:master-a1b2c3d` | Debugging, traceability |
| Custom | Manual dispatch | `ghcr.io/jonesh/traefik-hcloud:experimental` | Testing new features |

## Build Provenance

Every image includes build attestation for supply chain security:

```bash
# Verify attestation (requires GitHub CLI)
gh attestation verify oci://ghcr.io/jonesh/traefik-hcloud:v3.5.3-hcloud \
  --owner jonesh
```

Attestation includes:
- Build timestamp and commit SHA
- Workflow that built the image
- Source repository information
- Cryptographic signature

## Monitoring Builds

### View Workflow Runs

1. Go to: https://github.com/jonesh/lego-hcloud/actions/workflows/docker-build.yml
2. See all build history, logs, and artifacts

### Build Summary

Each workflow run includes a summary with:
- Image name and registry
- All tags applied
- Image digest (SHA256)
- Usage examples

### Notifications

GitHub Actions sends notifications on:
- Build failures (via email)
- Successful builds (optional, configure in Settings → Notifications)

## Troubleshooting

### Build Fails

**Check workflow logs:**
```bash
gh run list --workflow=docker-build.yml
gh run view <run-id> --log
```

**Common issues:**
- Go version mismatch → Update Dockerfile Go version
- Dependency fetch fails → Check lego-hcloud fork availability
- Out of disk space → Workflow includes disk cleanup

### Push to Registry Fails

**Authentication issue:**
- Workflow uses `GITHUB_TOKEN` automatically
- Ensure "Read and write permissions" enabled in Settings → Actions → General → Workflow permissions

**Quota exceeded:**
- GHCR has storage limits
- Delete old image versions: Settings → Packages → traefik-hcloud → Manage versions

### Image Pull Fails

**Make package public:**
1. Go to: https://github.com/users/jonesh/packages/container/traefik-hcloud
2. Package settings → Change visibility → Public

**Or authenticate for private:**
```bash
echo $GITHUB_TOKEN | docker login ghcr.io -u jonesh --password-stdin
```

## Upgrading Traefik Version

1. **Update Dockerfile:**
   ```dockerfile
   RUN git clone --depth 1 --branch v3.5.4 https://github.com/traefik/traefik.git
   ```

2. **Update version in ldflags:**
   ```dockerfile
   -ldflags="-s -w -X github.com/traefik/traefik/v3/pkg/version.Version=v3.5.4-hcloud"
   ```

3. **Update workflow env variable:**
   ```yaml
   env:
     TRAEFIK_VERSION: v3.5.4-hcloud
   ```

4. **Commit and push:**
   ```bash
   git add docker/Dockerfile .github/workflows/docker-build.yml
   git commit -m "build: upgrade Traefik to v3.5.4"
   git push origin master
   ```

5. **Workflow automatically builds new version**

## Local Development

Still want to build locally? You can:

```bash
cd docker
./build.sh

# Or build with Docker directly
docker build -t traefik-hcloud:local .

# Test locally before pushing
docker run --rm traefik-hcloud:local version
```

Local builds don't affect the CI/CD pipeline.

## Security Best Practices

- ✅ Build attestation enabled
- ✅ Multi-platform support
- ✅ Uses official GitHub Actions
- ✅ No custom secrets needed
- ✅ Reproducible builds
- ✅ Layer caching for efficiency

## Cost Considerations

- GitHub Actions: 2,000 minutes/month free (private repos)
- GHCR Storage: 500MB free, then $0.25/GB/month
- Workflow duration: ~3-10 minutes per build
- Storage per image: ~150MB (two platforms = ~300MB)

**Recommendation:** Delete old image versions to stay within free tier.

## Future Enhancements

Potential improvements:

1. **Automated Version Bumps**
   - Dependabot for Traefik updates
   - Auto-create PRs for new versions

2. **Multi-Registry Push**
   - Push to Docker Hub and GHCR
   - Redundancy for high availability

3. **Automated Testing**
   - Container structure tests
   - ACME challenge integration tests
   - Security scanning (Trivy, Snyk)

4. **Release Automation**
   - Auto-tag releases when Traefik updates
   - Generate changelogs
   - Update documentation

5. **Scheduled Rebuilds**
   - Weekly builds for security updates
   - Keep base images fresh

## Related Documentation

- [Docker Build README](./README.md) - Local build instructions
- [Deployment Guide](./DEPLOYMENT.md) - Using the image in production
- [Integration Guide](./INTEGRATION.md) - Project structure and usage
- [GitHub Actions Docs](https://docs.github.com/en/actions) - Official documentation
