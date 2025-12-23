.PHONY: help dev dev-ui dev-bff build build-ui build-bff clean docker-build docker-build-multi docker-push docker-run

# Image configuration
IMAGE_REGISTRY ?= ghcr.io/k8s-lynq
IMAGE_NAME ?= lynq-dashboard
IMAGE_TAG ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
IMAGE_FULL ?= $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
PLATFORMS ?= linux/amd64,linux/arm64

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Development
dev: ## Run both UI and BFF in development mode (requires two terminals)
	@echo "Run 'make dev-ui' in one terminal and 'make dev-bff' in another"

dev-ui: ## Run UI development server
	cd ui && npm run dev

dev-bff: ## Run BFF development server
	cd bff && go run ./cmd/server -mode local

# Docker Development
docker-up: ## Start development environment with Docker Compose
	docker-compose up --build

docker-down: ## Stop development environment
	docker-compose down

# Docker Production Build
docker-build: ## Build production Docker image (local arch)
	@echo "Building image: $(IMAGE_FULL)"
	DOCKER_BUILDKIT=1 docker build \
		--build-arg VERSION=$(IMAGE_TAG) \
		-t $(IMAGE_FULL) \
		-t $(IMAGE_REGISTRY)/$(IMAGE_NAME):latest \
		.

docker-build-multi: ## Build multi-arch Docker image (amd64/arm64)
	@echo "Building multi-arch image: $(IMAGE_FULL)"
	docker buildx build \
		--platform $(PLATFORMS) \
		--build-arg VERSION=$(IMAGE_TAG) \
		-t $(IMAGE_FULL) \
		-t $(IMAGE_REGISTRY)/$(IMAGE_NAME):latest \
		--push \
		.

docker-push: ## Push Docker image to registry
	@echo "Pushing image: $(IMAGE_FULL)"
	docker push $(IMAGE_FULL)
	docker push $(IMAGE_REGISTRY)/$(IMAGE_NAME):latest

docker-run: docker-build ## Build and run Docker image locally
	docker run --rm -p 8080:8080 $(IMAGE_FULL)

# Build
build: build-ui build-bff ## Build both UI and BFF

build-ui: ## Build UI for production
	cd ui && npm run build

build-bff: ## Build BFF binary
	cd bff && go build -o ../bin/bff ./cmd/server

# Clean
clean: ## Clean build artifacts
	rm -rf ui/dist bin/
	cd bff && go clean

# Install
install-ui: ## Install UI dependencies
	cd ui && npm install

install-bff: ## Download BFF dependencies
	cd bff && go mod download

install: install-ui install-bff ## Install all dependencies

# Lint
lint-ui: ## Lint UI code
	cd ui && npm run lint

lint-bff: ## Lint BFF code
	cd bff && go vet ./...

lint: lint-ui lint-bff ## Lint all code

# Test
test-ui: ## Run UI tests
	cd ui && npm test || true

test-bff: ## Run BFF tests
	cd bff && go test ./...

test: test-ui test-bff ## Run all tests
