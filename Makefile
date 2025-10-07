.PHONY: build build-local test test-short test-coverage run clean docker-build docker-push deploy all help

# Variables
BINARY_NAME := autocache
BUILD_DIR := .
IMAGE_NAME := registry.digitalilusion.com/autocache
IMAGE_TAG := latest
IMAGE := $(IMAGE_NAME):$(IMAGE_TAG)
K8S_MANIFEST := k8s-manifest.yaml
K8S_SECRET := k8s-secret.yaml
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Default target for local development
all: test build-local

# Local development targets
build-local:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) ./cmd/autocache
	@echo "Build complete: $(BINARY_NAME)"

run: build-local
	@echo "Starting $(BINARY_NAME)..."
	./$(BINARY_NAME)

test:
	@echo "Running tests..."
	go test -v ./...

test-short:
	@echo "Running tests (short mode)..."
	go test -short -v ./...

test-coverage:
	@echo "Running tests with coverage..."
	go test -cover ./...
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Install from: https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run ./...

# Docker targets
docker-build:
	@echo "Building Docker image: $(IMAGE)"
	docker build \
		--platform linux/amd64 \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(IMAGE) \
		-f Dockerfile \
		.
	@echo "Build complete: $(IMAGE)"

docker-push:
	@echo "Pushing Docker image: $(IMAGE)"
	docker push $(IMAGE)
	@echo "Push complete: $(IMAGE)"

# Deploy secret to Kubernetes
deploy-secret:
	@echo "Deploying secret to Kubernetes namespace: montevive"
	kubectl apply -f $(K8S_SECRET)
	@echo "Secret deployment complete"

# Deploy to Kubernetes
deploy:
	@echo "Deploying to Kubernetes namespace: montevive"
	kubectl apply -f $(K8S_SECRET)
	kubectl apply -f $(K8S_MANIFEST)
	@echo "Deployment complete"

# Combined Docker targets
docker-build-push: docker-build docker-push

# Show deployment status
status:
	@echo "Checking deployment status..."
	kubectl -n montevive get deployment autocache
	kubectl -n montevive get pods -l app=autocache
	kubectl -n montevive get svc autocache

# Show logs
logs:
	@echo "Fetching logs..."
	kubectl -n montevive logs -l app=autocache --tail=100 -f

# Restart deployment
restart:
	@echo "Restarting deployment..."
	kubectl -n montevive rollout restart deployment/autocache
	kubectl -n montevive rollout status deployment/autocache

# Delete deployment
delete:
	@echo "Deleting deployment..."
	kubectl delete -f $(K8S_MANIFEST)
	kubectl delete -f $(K8S_SECRET) || true

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html
	docker rmi $(IMAGE) || true
	@echo "Clean complete"

# Show help
help:
	@echo "Autocache Makefile"
	@echo ""
	@echo "Local Development:"
	@echo "  make build-local        - Build binary locally"
	@echo "  make run                - Build and run locally"
	@echo "  make test               - Run all tests"
	@echo "  make test-short         - Run tests in short mode"
	@echo "  make test-coverage      - Run tests with coverage report"
	@echo "  make lint               - Run linter (requires golangci-lint)"
	@echo "  make clean              - Clean build artifacts"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build       - Build Docker image"
	@echo "  make docker-push        - Push Docker image to registry"
	@echo "  make docker-build-push  - Build and push Docker image"
	@echo ""
	@echo "Kubernetes (Private):"
	@echo "  make deploy-secret      - Deploy secret only"
	@echo "  make deploy             - Deploy secret and application"
	@echo "  make status             - Show deployment status"
	@echo "  make logs               - Show pod logs"
	@echo "  make restart            - Restart deployment"
	@echo "  make delete             - Delete deployment"
	@echo ""
	@echo "Variables:"
	@echo "  BINARY_NAME = $(BINARY_NAME)"
	@echo "  IMAGE_NAME  = $(IMAGE_NAME)"
	@echo "  IMAGE_TAG   = $(IMAGE_TAG)"
	@echo "  VERSION     = $(VERSION)"
	@echo "  BUILD_TIME  = $(BUILD_TIME)"
	@echo "  GIT_COMMIT  = $(GIT_COMMIT)"
