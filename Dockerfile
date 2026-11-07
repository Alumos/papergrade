# syntax=docker/dockerfile:1.7
# Version tags are intentional; release automation should record image digests.
FROM node:24-bookworm-slim AS web-build
WORKDIR /src/web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm run build

FROM golang:1.25-bookworm AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY migrations/ ./migrations/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/papergrade ./cmd/papergrade

FROM debian:bookworm-slim AS app
RUN apt-get update && apt-get install --no-install-recommends -y ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && groupadd -g 10001 papergrade && useradd -u 10001 -g papergrade -d /app -s /usr/sbin/nologin papergrade \
 && mkdir -p /app/data /app/tmp && chown -R papergrade:papergrade /app
WORKDIR /app
COPY --from=go-build /out/papergrade /usr/local/bin/papergrade
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/papergrade"]
CMD ["api"]

FROM caddy:2.10-alpine AS gateway
COPY --from=web-build /src/web/dist /srv
COPY deploy/Caddyfile /etc/caddy/Caddyfile
