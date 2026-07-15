SHELL := /bin/bash

GO ?= go
GOFMT ?= gofmt
PNPM ?= pnpm
DOCKER ?= docker
BINNACLE_BIN ?= bin/binnacle
VERSION ?= dev
LDFLAGS ?= -s -w -X main.version=$(VERSION)
GO_SOURCE_FILES := $(shell find cmd internal -type f -name '*.go' -print)

.DEFAULT_GOAL := help

.PHONY: help dev dev-demo dev-host test check build image image-multi format-check go-test web-test web-check go-vet benchmark benchmark-matrix

help: ## Show the supported local development commands.

	@awk 'BEGIN { FS = ":.*##"; printf "Binnacle development commands:\n" } /^[a-zA-Z0-9_-]+:.*##/ { printf "  %-16s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

dev: build ## Start the demo development environment (backend + Vite dev server).

	@echo "Starting Binnacle demo backend and Vite dev server..."
	@export BINNACLE_SETUP_TOKEN=$$(openssl rand -hex 32); \
	BINNACLE_LISTEN_ADDRESS=127.0.0.1:8080 BINNACLE_DATA_DIR=$$(mktemp -d) $(BINNACLE_BIN) --demo &
	@BINNACLE_PID=$$!; trap 'kill $$BINNACLE_PID 2>/dev/null || true' EXIT; $(PNPM) --dir web dev

dev-demo: build ## Run the deterministic demo environment with a fresh temporary database.

	@export BINNACLE_SETUP_TOKEN=$$(openssl rand -hex 32); \
	BINNACLE_LISTEN_ADDRESS=127.0.0.1:8080 BINNACLE_DATA_DIR=$$(mktemp -d) $(BINNACLE_BIN) --demo

dev-host: build ## Run against local host/Docker interfaces (requires appropriate permissions).

	@echo "Run with host/Docker access. Ensure the current user can read /proc, /sys, and the Docker socket."
	@export BINNACLE_SETUP_TOKEN=$$(openssl rand -hex 32); \
	BINNACLE_LISTEN_ADDRESS=127.0.0.1:8080 BINNACLE_DATA_DIR=$$(mktemp -d) $(BINNACLE_BIN)

test: go-test web-test ## Run Go and frontend unit tests.

check: format-check go-vet test web-check ## Run the local CI-quality subset.

build: ## Build the production frontend and CGO-enabled Binnacle binary.

	$(PNPM) --dir web build
	mkdir -p $(dir $(BINNACLE_BIN))
	CGO_ENABLED=1 $(GO) build -ldflags='$(LDFLAGS)' -o $(BINNACLE_BIN) ./cmd/binnacle

image: build ## Build a local container image for the current platform.

	$(DOCKER) build --build-arg VERSION=$(VERSION) -f packaging/docker/Dockerfile -t ghcr.io/drilonrecica/binnacle:local .

image-multi: ## Build a multi-arch container image (requires buildx and a registry push).

	$(DOCKER) buildx build --platform linux/amd64,linux/arm64 -f packaging/docker/Dockerfile -t ghcr.io/drilonrecica/binnacle:$(VERSION) --push .

vuln: ## Run dependency vulnerability scans (requires govulncheck and pnpm).

	scripts/govulncheck.sh
	$(PNPM) --dir web audit --audit-level moderate

licenses: ## Check Go dependency licenses (requires go-licenses v2).

	$(GO) run github.com/google/go-licenses/v2@latest check ./... --allowed_licenses=MIT,BSD-2-Clause,BSD-3-Clause,Apache-2.0,ISC,MPL-2.0,AGPL-3.0

sbom: ## Generate an SPDX SBOM for the container image (requires syft).

	$(DOCKER) build -f packaging/docker/Dockerfile -t ghcr.io/drilonrecica/binnacle:sbom .
	syft ghcr.io/drilonrecica/binnacle:sbom -o spdx-json=binnacle.spdx.json
	syft ghcr.io/wollomatic/socket-proxy:1.12.3@sha256:9e781fbe79315355d08901832f639119aa332ac27ee6157fc7f2fab5193c8600 -o spdx-json=socket-proxy.spdx.json

scan: ## Scan the container image for vulnerabilities (requires trivy).

	$(DOCKER) build -f packaging/docker/Dockerfile -t ghcr.io/drilonrecica/binnacle:scan .
	trivy image ghcr.io/drilonrecica/binnacle:scan
	trivy image ghcr.io/wollomatic/socket-proxy:1.12.3@sha256:9e781fbe79315355d08901832f639119aa332ac27ee6157fc7f2fab5193c8600

format-check: ## Check Go and frontend formatting without modifying source.

	@if test -n "$(GO_SOURCE_FILES)"; then \
		unformatted="$$($(GOFMT) -l $(GO_SOURCE_FILES))"; \
		if test -n "$$unformatted"; then \
			printf '%s\n' "Go files need formatting:" >&2; \
			printf '%s\n' "$$unformatted" >&2; \
			exit 1; \
		fi; \
	fi
	$(PNPM) --dir web format

go-test: ## Run all Go tests.

	$(GO) test ./...

web-test: ## Run frontend unit tests.

	$(PNPM) --dir web test:run

web-check: ## Run Svelte type checking and linting.

	$(PNPM) --dir web check
	$(PNPM) --dir web lint

go-vet: ## Run Go vet.

	$(GO) vet ./...

benchmark: ## Run a short deterministic benchmark with 30 synthetic containers.

	python3 scripts/benchmark.py --containers 30 --duration 60 --output benchmark-report.json

benchmark-matrix: ## Run the 10/30/50/100 container benchmark matrix.

	BENCHMARK_DURATION=60 ./scripts/benchmark-matrix.sh
