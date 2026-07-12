# syntax=docker/dockerfile:1

# Multi-stage build for the Binnacle monitoring service.
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
RUN CGO_ENABLED=1 go build -ldflags='-s -w' -o /binnacle ./cmd/binnacle

FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates libsqlite3-0 \
 && rm -rf /var/lib/apt/lists/* \
 && useradd --system --home-dir /var/lib/binnacle --create-home binnacle

COPY --from=build /binnacle /usr/local/bin/binnacle

USER binnacle
WORKDIR /var/lib/binnacle
VOLUME /var/lib/binnacle
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD ["/usr/local/bin/binnacle", "--healthcheck"]

ENTRYPOINT ["/usr/local/bin/binnacle"]
