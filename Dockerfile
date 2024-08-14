FROM caddy:2.8.4-builder-alpine AS builder

ARG VERSION

RUN xcaddy build \
    --with github.com/Elegant996/scgi-transport@v1.0.3

FROM caddy:2.8.4-alpine

COPY --from=builder /usr/bin/caddy /usr/bin/caddy

LABEL org.opencontainers.image.description="SCGI reverse proxy transport module for Caddy"
LABEL org.opencontainers.image.licenses="Apache-2.0"
LABEL org.opencontainers.image.source="https://github.com/Elegant996/scgi-transport"
LABEL org.opencontainers.image.version=${VERSION}