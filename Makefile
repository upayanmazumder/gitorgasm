SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

CORE := core
WEB := web
SCORING := scoring
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
tools: ## Install pinned tools (golangci-lint, kubeconform, govulncheck, cargo-deny)
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
	go install github.com/yannh/kubeconform/cmd/kubeconform@v0.8.0
	go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
	cargo install --locked cargo-deny@0.20.2

.env.local:
	@umask 077 && key="$$(openssl rand -base64 32)" && \
	  awk -v key="$$key" '/^TOKEN_ENCRYPTION_KEY=$$/ {print "TOKEN_ENCRYPTION_KEY=" key; next} {print}' .env.example > $@
	@echo "created $@ (GitHub App group empty: auth routes return 503 until you fill it)"

.PHONY: env
env: .env.local ## Create .env.local from .env.example with a fresh dev key (never overwrites)

.PHONY: up
up: .env.local ## Start postgres, run migrations, start the API (http://localhost:8080)
	docker compose up -d --build --wait postgres
	docker compose up -d --build api
	@for i in $$(seq 1 60); do curl -fsS localhost:8080/readyz >/dev/null 2>&1 && break; sleep 1; done
	@curl -fsS localhost:8080/readyz && echo

.PHONY: up-github
up-github: .env.local ## Also start the worker (needs the GITHUB_* group in .env.local)
	docker compose --profile github up -d --build
	@for i in $$(seq 1 60); do curl -fsS localhost:8080/readyz >/dev/null 2>&1 && curl -fsS localhost:9091/readyz >/dev/null 2>&1 && break; sleep 1; done
	@curl -fsS localhost:8080/readyz && echo
	@curl -fsS localhost:9091/readyz && echo "worker: ready"

.PHONY: down
down: ## Stop the local stack (keeps the database volume)
	docker compose --profile github down

.PHONY: db
db: .env.local ## Start only postgres (used by tests)
	docker compose up -d --wait postgres

.PHONY: run-api
run-api: db ## Run migrations, then the API on the host (go run)
	set -a && . ./.env.local && set +a && cd $(CORE) && go run ./cmd/gitorgasm migrate && go run ./cmd/gitorgasm api

.PHONY: run-worker
run-worker: db ## Run the worker on the host (go run; needs the GITHUB_* group)
	set -a && . ./.env.local && set +a && cd $(CORE) && METRICS_ADDR=:9091 go run ./cmd/gitorgasm worker

.PHONY: smoke
smoke: up ## Start the stack and check health, readiness, and an authenticated route
	curl -fsS localhost:8080/healthz
	curl -fsS localhost:8080/readyz
	test "$$(curl -s -o /dev/null -w '%{http_code}' localhost:8080/v1/organizations)" = 401
	@echo "smoke: ok"

.PHONY: fmt
fmt: ## Format all code (each language with the config next to its code)
	cd $(CORE) && $(GOLANGCI) fmt ./...
	cd $(BENCH)/go && $(GOLANGCI) fmt ./...
	cd $(BENCH)/rust && cargo fmt
	cd $(SCORING) && cargo fmt
	cd $(WEB) && pnpm format

.PHONY: lint
lint: ## Lint and format-check all code
	cd $(CORE) && $(GOLANGCI) fmt --diff ./... && go vet ./... && $(GOLANGCI) run ./...
	cd $(BENCH)/go && $(GOLANGCI) fmt --diff ./... && go vet ./...
	cd $(BENCH)/rust && cargo fmt --check && cargo clippy --all-targets --locked -- -D warnings
	cd $(SCORING) && cargo fmt --check && cargo clippy --all-targets --locked -- -D warnings
	cd $(WEB) && pnpm lint && pnpm typecheck && pnpm format:check
	go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12

.PHONY: test-go
test-go: db ## Go tests against an isolated database per test
	cd $(SCORING) && cargo build --locked --bin score
	cd $(CORE) && SCORING_BIN="$(abspath $(SCORING))/target/debug/score" go test -race -count=1 ./...

.PHONY: test
test: test-go ## Run all tests
	cd $(BENCH)/rust && cargo test --locked
	cd $(SCORING) && cargo test --locked
	cd $(WEB) && SKIP_ENV_VALIDATION=true pnpm test

.PHONY: build
build: ## Build binaries, the web app, and the container image
	cd $(CORE) && CGO_ENABLED=0 go build -trimpath -o bin/gitorgasm ./cmd/gitorgasm
	cd $(BENCH)/go && go build ./...
	cd $(BENCH)/rust && cargo build --release --locked
	cd $(SCORING) && cargo build --release --locked
	cd $(WEB) && SKIP_ENV_VALIDATION=true pnpm build
	docker build -f $(CORE)/Dockerfile -t gitorgasm-core:dev .

.PHONY: score-bench
score-bench: ## Measure 100,000 scored contributions within 60 seconds
	cd $(SCORING) && cargo run --release --locked --bin score-bench -- --limit-seconds 60

.PHONY: manifests
manifests: ## Render and schema-validate Kubernetes manifests
	kustomize build deploy/kustomize/overlays/production | $(KUBECONFORM) -strict -summary -kubernetes-version 1.34.0 -

.PHONY: vuln
vuln: ## Check dependencies for known vulnerabilities and license policy
	cd $(CORE) && $(GOVULNCHECK) ./...
	cd $(SCORING) && cargo deny check

.PHONY: web-install
web-install: ## Install root tooling (git hooks) and web dependencies
	pnpm install --frozen-lockfile
	cd $(WEB) && pnpm install --frozen-lockfile

.PHONY: verify
verify: web-install lint test build manifests vuln ## Everything CI runs
	@echo "verify: ok"
