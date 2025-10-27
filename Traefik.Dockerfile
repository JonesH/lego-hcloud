# Use official upstream lego v4.27.0 with new Hetzner Cloud DNS support
FROM goacme/lego:v4.27.0 as lego-source

# Final image: Traefik with lego integration
FROM traefik:v3.5

# Copy lego binary from official upstream image
COPY --from=lego-source /lego /usr/local/bin/lego

# Ensure lego is executable and add ca-certificates
RUN chmod +x /usr/local/bin/lego && \
    apk --no-cache add ca-certificates

# Traefik entrypoint remains default
ENTRYPOINT ["/entrypoint.sh"]
CMD ["traefik"]
