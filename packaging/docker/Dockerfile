# syntax=docker/dockerfile:1

# Multi-stage build for the TALOS monitoring service.
# Supports linux/amd64 and linux/arm64 via buildx.

FROM node:24-bookworm AS web
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY web/package.json web/package.json
RUN corepack enable && pnpm --dir web install --frozen-lockfile
COPY web/ web/
RUN pnpm --dir web build

FROM golang:1.26-bookworm AS build
WORKDIR /src
RUN apt-get update \
 && apt-get install -y --no-install-recommends gcc libc6-dev libsqlite3-dev \
 && rm -rf /var/lib/apt/lists/*
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/webembed/dist internal/webembed/dist
RUN CGO_ENABLED=1 go build -ldflags='-s -w' -o /talos ./cmd/talos

FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates libsqlite3-0 \
 && rm -rf /var/lib/apt/lists/* \
 && useradd --system --home-dir /var/lib/talos --create-home talos

COPY --from=build /talos /usr/local/bin/talos

USER talos
WORKDIR /var/lib/talos
VOLUME /var/lib/talos
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD ["/usr/local/bin/talos", "--healthcheck"]

ENTRYPOINT ["/usr/local/bin/talos"]
