FROM caddy:2.10.0-builder-alpine AS builder

COPY . ./src

RUN xcaddy build \
    --with github.com/Elegant996/scgi-transport=./src

FROM caddy:2.10.0-alpine

COPY --from=builder /usr/bin/caddy /usr/bin/caddy