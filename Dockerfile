ARG CADDY_VERSION
ARG TAG

FROM caddy:${CADDY_VERSION}-builder-alpine AS builder

ARG TAG
ENV TAG ${TAG}

RUN xcaddy build \
    --with github.com/Elegant996/scgi-transport@v${TAG}

FROM caddy:${CADDY_VERSION}-alpine

COPY --from=builder /usr/bin/caddy /usr/bin/caddy