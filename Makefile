SHELL := /bin/bash

GO ?= go
GOFMT ?= gofmt
PNPM ?= pnpm
DOCKER ?= docker
TALOS_BIN ?= bin/talos
VERSION ?= dev
GO_SOURCE_FILES := $(shell find cmd internal -type f -name '*.go' -print)

.DEFAULT_GOAL := help

.PHONY: help dev dev-demo dev-host test check build image image-multi format-check go-test web-test web-check go-vet

help: ## Show the supported local development commands.

	@awk 'BEGIN { FS = ":.*##"; printf "TALOS development commands:\n" } /^[a-zA-Z0-9_-]+:.*##/ { printf "  %-16s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

dev: ## Start the demo development environment (requires T015).

	@printf '%s\n' 'make dev is unavailable: complete T015 (deterministic demo collector) first.' >&2
	@printf '%s\n' 'Those tasks will use a fresh temporary TALOS_DATA_DIR for each development run.' >&2
	@exit 1

dev-demo: ## Run the deterministic demo environment (requires T015).

	@printf '%s\n' 'make dev-demo is unavailable: complete T015 (deterministic demo collector) first.' >&2
	@printf '%s\n' 'The future command will use a fresh temporary TALOS_DATA_DIR and no Docker socket.' >&2
	@exit 1

dev-host: ## Run against local host/Docker interfaces (requires collectors).

	@printf '%s\n' 'make dev-host is unavailable: complete T031 and T043 before using real host or Docker interfaces.' >&2
	@printf '%s\n' 'The future command will require explicit host paths, Docker permissions, and a temporary TALOS_DATA_DIR.' >&2
	@exit 1

test: go-test web-test ## Run Go and frontend unit tests.

check: format-check go-vet test web-check ## Run the local CI-quality subset.

build: ## Build the production frontend and CGO-enabled TALOS binary.

	$(PNPM) --dir web build
	mkdir -p $(dir $(TALOS_BIN))
	CGO_ENABLED=1 $(GO) build -o $(TALOS_BIN) ./cmd/talos

image: build ## Build a local container image for the current platform.

	$(DOCKER) build -f packaging/docker/Dockerfile -t ghcr.io/drilonrecica/talos:local .

image-multi: ## Build a multi-arch container image (requires buildx and a registry push).

	$(DOCKER) buildx build --platform linux/amd64,linux/arm64 -f packaging/docker/Dockerfile -t ghcr.io/drilonrecica/talos:$(VERSION) --push .

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
