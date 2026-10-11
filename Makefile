.DEFAULT_GOAL := help

BACKEND_DIR := backend
FRONTEND_DIR := frontend
HELM_DIR := helm/cross-cloudr
BACKEND_IMAGE := cross-cloudr-backend:latest
FRONTEND_IMAGE := cross-cloudr-frontend:latest
NODE_IMAGE := node:20
# Run containers as the host user so generated files stay user-owned.
DOCKER_RUN := docker run --rm -u $(shell id -u):$(shell id -g) -e HOME=/tmp

.PHONY: help build build-backend build-frontend test test-backend test-frontend test-frontend-docker lint fmt dev down docker-build helm-deps clean

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-24s\033[0m %s\n", $$1, $$2}'

build: build-backend build-frontend ## Build backend binary and frontend assets

build-backend: ## Build the backend binary to backend/bin/server
	cd $(BACKEND_DIR) && go build -o bin/server ./cmd/server

build-frontend: ## Install dependencies and build frontend assets
	cd $(FRONTEND_DIR) && npm ci --no-audit --no-fund && npm run build

test: test-backend test-frontend ## Run all tests

test-backend: ## Run backend tests
	cd $(BACKEND_DIR) && go test ./...

test-frontend: ## Run frontend tests (requires Node 20+)
	cd $(FRONTEND_DIR) && npm ci --no-audit --no-fund && npm test

test-frontend-docker: ## Run frontend tests in a Node container (no local Node needed)
	$(DOCKER_RUN) -v "$(PWD)/$(FRONTEND_DIR):/app" -w /app $(NODE_IMAGE) sh -c "npm ci --no-audit --no-fund && npm test"

lint: ## Vet the backend code
	cd $(BACKEND_DIR) && go vet ./...

fmt: ## Format the backend code with gofmt
	cd $(BACKEND_DIR) && gofmt -l -w .

dev: ## Start the full stack with Docker Compose
	docker compose up --build

down: ## Stop the Docker Compose stack
	docker compose down

docker-build: ## Build the backend and frontend Docker images
	docker build -t $(BACKEND_IMAGE) ./$(BACKEND_DIR)
	docker build -t $(FRONTEND_IMAGE) ./$(FRONTEND_DIR)

helm-deps: ## Fetch Helm chart dependencies
	helm dependency build ./$(HELM_DIR)

clean: ## Remove build artifacts (backend binary, frontend dist)
	rm -rf $(BACKEND_DIR)/bin $(FRONTEND_DIR)/dist
