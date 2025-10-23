# Custom Traefik v3.5.3 with Hetzner Cloud DNS provider
# Uses JonesH/lego-hcloud fork for api.hetzner.cloud DNS API support
#
# Build: docker build -t traefik:v3.5.3-hcloud .
# Size: ~150MB (vs ~100MB official)

FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

# Clone Traefik v3.5.3
WORKDIR /src
RUN git clone --depth 1 --branch v3.5.3 https://github.com/traefik/traefik.git

WORKDIR /src/traefik

# Replace lego with custom fork containing hcloud DNS provider
RUN go mod edit -replace github.com/go-acme/lego/v4=github.com/JonesH/lego-hcloud/v4@fa627743b01e33fae1df8c99f5462654229210e4

# Download dependencies with replace in effect (allow missing sum entries)
ENV GOPRIVATE=github.com/JonesH/*
ENV GOFLAGS="-mod=mod"
RUN go mod download -x

# Build Traefik binary with -mod=mod to allow go.sum updates during build
RUN CGO_ENABLED=0 GOFLAGS="-mod=mod" go build -v -trimpath \
    -ldflags="-s -w -X github.com/traefik/traefik/v3/pkg/version.Version=v3.5.3-hcloud" \
    -o /usr/local/bin/traefik ./cmd/traefik

# Verify binary works
RUN /usr/local/bin/traefik version

# Runtime image - minimal Alpine
FROM alpine:3.19

# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata && \
    mkdir -p /etc/traefik /data

# Copy Traefik binary from builder
COPY --from=builder /usr/local/bin/traefik /usr/local/bin/traefik

# Verify it runs
RUN traefik version

EXPOSE 80 443
VOLUME ["/data", "/etc/traefik"]

ENTRYPOINT ["/usr/local/bin/traefik"]
