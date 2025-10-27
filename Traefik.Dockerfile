# Multi-stage build: Build lego first
FROM golang:1-alpine as lego-builder

RUN apk --no-cache --no-progress add make git

WORKDIR /go/lego

ENV GO111MODULE=on

# Download go modules
COPY go.mod .
COPY go.sum .
RUN go mod download

# Build lego with hetznerhcloud provider
COPY . .
RUN make build

# Final image: Traefik with lego integration
FROM traefik:v3.5

# Copy lego binary from builder
COPY --from=lego-builder /go/lego/dist/lego /usr/local/bin/lego

# Ensure lego is executable
RUN chmod +x /usr/local/bin/lego

# Add ca-certificates for SSL verification
USER root
RUN apk --no-cache add ca-certificates

# Switch back to traefik user
USER traefik

# Traefik entrypoint remains default
ENTRYPOINT ["/entrypoint.sh"]
CMD ["traefik"]
