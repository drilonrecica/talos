FROM node:22-bookworm AS web
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY web/package.json web/package.json
RUN corepack enable && pnpm --dir web install --frozen-lockfile
COPY web web
RUN pnpm --dir web build

FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/webembed/dist internal/webembed/dist
RUN CGO_ENABLED=1 go build -o /talos ./cmd/talos

FROM debian:bookworm-slim
RUN useradd --system --home /var/lib/talos talos
COPY --from=build /talos /usr/local/bin/talos
USER talos
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/talos"]
