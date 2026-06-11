# Makefile for local development (run from this directory: main.go + docker-compose.yml).

SWAG_VERSION := v1.16.6
GOPATH_BIN  := $(shell go env GOPATH 2>/dev/null)/bin
BASE_URL    ?= http://localhost:8080
SCRIPTS_DIR := ../scripts
DOCKER_COMPOSE := $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || echo "docker-compose")

.DEFAULT_GOAL := help

.PHONY: help up down restart ps logs build rebuild swag swag-docker test test-go test-unit test-smoke

help: ## Show available targets
	@printf "Targets (run from %s):\n" "$(CURDIR)"
	@grep -E '^[a-zA-Z0-9_.-]+:.*##' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "  %-16s %s\n", $$1, $$2}'

up: ## Start MongoDB, MinIO and API (docker compose up -d --build)
	$(DOCKER_COMPOSE) up -d --build

down: ## Stop docker compose stack
	$(DOCKER_COMPOSE) down

restart: down up ## Restart full stack

ps: ## Show compose service status
	$(DOCKER_COMPOSE) ps

logs: ## Follow wb-landing-builder service logs
	$(DOCKER_COMPOSE) logs -f wb-landing-builder

build: ## Build Go binary locally
	go build ./...

swag: ## Generate Swagger docs locally (needs Go + network for go install)
	go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)
	$(GOPATH_BIN)/swag init -g main.go -o docs --parseDependency --parseInternal

swag-docker: ## Generate Swagger docs via Docker (Go on host not required)
	docker run --rm \
		-v "$(CURDIR):/app" \
		-w /app \
		golang:1.25.10-alpine \
		sh -c 'apk add --no-cache git && go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION) && $$(go env GOPATH)/bin/swag init -g main.go -o docs --parseDependency --parseInternal'

rebuild: swag up ## Regenerate Swagger and rebuild/restart API container

test-go: build ## Compile all Go packages
	@echo "Go build OK"

test-unit: ## Run Go unit tests (no Docker required)
	go test ./...

test-smoke: ## Full HTTP smoke test: auth → storage → publishing (needs: curl, jq, make up)
	@chmod +x $(SCRIPTS_DIR)/smoke.sh
	BASE_URL=$(BASE_URL) $(SCRIPTS_DIR)/smoke.sh

test: test-go test-unit test-smoke ## Run build + unit tests + smoke test
