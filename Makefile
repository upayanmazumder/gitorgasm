# Gitorgasm developer commands. Run `make help` for the list.
SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

CORE := core
WEB := apps/web
BENCH := tests/performance/lang-bench
GOBIN := $(shell go env GOPATH)/bin
GOLANGCI := $(GOBIN)/golangci-lint
KUBECONFORM := $(GOBIN)/kubeconform
GOVULNCHECK := $(GOBIN)/govulncheck
TEST_DATABASE_URL ?= postgres://gitorgasm:gitorgasm@127.0.0.1:55432/gitorgasm?sslmode=disable
export TEST_DATABASE_URL

.PHONY: help
help: ## Show targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

.PHONY: tools
tools: ## Install pinned Go tools (golangci-lint, kubeconform, govulncheck)
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
	go install github.com/yannh/kubeconform/cmd/kubeconform@v0.8.0
	go install golang.org/x/vuln/cmd/govulncheck@latest

# ---- local stack -----------------------------------------------------------------------------
.PHONY: up
up: ## Start postgres, run migrations, start the API (http://localhost:8080)
	docker compose up -d --build --wait postgres
	docker compose up -d --build api
	@for i in $$(seq 1 60); do curl -fsS localhost:8080/readyz >/dev/null 2>&1 && break; sleep 1; done
	@curl -fsS localhost:8080/readyz && echo

.PHONY: up-github
up-github: ## Also start the worker (needs GitHub App variables in .env)
	docker compose --profile github up -d --build

.PHONY: down
down: ## Stop the local stack (keeps the database volume)
	docker compose --profile github down

.PHONY: db
db: ## Start only postgres (used by tests)
	docker compose up -d --wait postgres

.PHONY: smoke
smoke: up ## Start the stack and check health, readiness, and an authenticated route
	curl -fsS localhost:8080/healthz
	curl -fsS localhost:8080/readyz
	test "$$(curl -s -o /dev/null -w '%{http_code}' localhost:8080/v1/organizations)" = 401
	@echo "smoke: ok"

# ---- quality gates ---------------------------------------------------------------------------
.PHONY: fmt
fmt: ## Format all code
	cd $(CORE) && $(GOLANGCI) fmt ./...
	cd $(BENCH)/go && gofmt -w .
	cd $(BENCH)/rust && cargo fmt
	cd $(WEB) && pnpm format

.PHONY: lint
lint: ## Lint and format-check all code
	cd $(CORE) && test -z "$$(gofmt -l .)" && go vet ./... && $(GOLANGCI) run ./...
	cd $(BENCH)/go && test -z "$$(gofmt -l .)" && go vet ./...
	cd $(BENCH)/rust && cargo fmt --check && cargo clippy --all-targets --locked -- -D warnings
	cd $(WEB) && pnpm lint && pnpm typecheck && pnpm format:check

.PHONY: test-go
test-go: db ## Go tests against an isolated database per test
	cd $(CORE) && go test -race -count=1 ./...

.PHONY: test
test: test-go ## Run all tests
	cd $(BENCH)/rust && cargo test --locked
	cd $(WEB) && SKIP_ENV_VALIDATION=true pnpm test

.PHONY: build
build: ## Build binaries, the web app, and the container image
	cd $(CORE) && CGO_ENABLED=0 go build -trimpath -o bin/gitorgasm ./cmd/gitorgasm
	cd $(BENCH)/go && go build ./...
	cd $(BENCH)/rust && cargo build --release --locked
	cd $(WEB) && SKIP_ENV_VALIDATION=true pnpm build
	docker build -t gitorgasm-core:dev $(CORE)

.PHONY: manifests
manifests: ## Render and schema-validate Kubernetes manifests
	kustomize build deploy/kustomize/overlays/production | $(KUBECONFORM) -strict -summary -kubernetes-version 1.34.0 -

.PHONY: vuln
vuln: ## Check Go dependencies for known vulnerabilities
	cd $(CORE) && $(GOVULNCHECK) ./...

.PHONY: web-install
web-install:
	cd $(WEB) && pnpm install --frozen-lockfile

.PHONY: verify
verify: web-install lint test build manifests vuln ## Everything CI runs
	@echo "verify: ok"
