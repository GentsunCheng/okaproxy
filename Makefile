# OkaProxy Makefile
.PHONY: help build run test clean docker docker-run docker-stop install fmt vet security release

# Variables
BINARY_NAME := okaproxy
DOCKER_IMAGE := okaproxy:latest
GO_VERSION := 1.23
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v1.0.0")
BUILD_TIME := $(shell date +%Y-%m-%dT%H:%M:%S%z)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT) -w -s"

# Default target
help: ## Show this help message
	@echo "OkaProxy - High-performance HTTP Proxy Server"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Development targets
build: ## Build the application
	@echo "Building $(BINARY_NAME)..."
	@go build $(LDFLAGS) -o $(BINARY_NAME) .
	@echo "Built $(BINARY_NAME) successfully"

run: ## Run the application
	@echo "Starting $(BINARY_NAME)..."
	@go run . --config config.toml

dev: ## Run in development mode with auto-reload (requires air)
	@echo "Starting development server..."
	@air -c .air.toml

test: ## Run tests
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Test coverage report generated: coverage.html"

benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@goimports -w .

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

lint: ## Run golangci-lint
	@echo "Running golangci-lint..."
	@golangci-lint run

security: ## Run security checks with gosec
	@echo "Running security checks..."
	@gosec ./...

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@rm -rf dist/
	@go clean -cache
	@echo "Cleaned successfully"

# Dependency management
deps: ## Download and verify dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod verify

tidy: ## Tidy up go.mod
	@echo "Tidying go.mod..."
	@go mod tidy

update: ## Update dependencies
	@echo "Updating dependencies..."
	@go get -u ./...
	@go mod tidy

# Installation targets
install: ## Install the application
	@echo "Installing $(BINARY_NAME)..."
	@go install $(LDFLAGS) .

uninstall: ## Uninstall the application
	@echo "Uninstalling $(BINARY_NAME)..."
	@rm -f $(GOPATH)/bin/$(BINARY_NAME)

# Docker targets
docker: ## Build Docker image
	@echo "Building Docker image..."
	@docker build \
		--build-arg BUILD_DATE=$(BUILD_TIME) \
		--build-arg VCS_REF=$(GIT_COMMIT) \
		--build-arg VERSION=$(VERSION) \
		-t $(DOCKER_IMAGE) .
	@echo "Docker image built: $(DOCKER_IMAGE)"

docker-run: ## Run with Docker Compose
	@echo "Starting services with Docker Compose..."
	@docker-compose up -d
	@echo "Services started. Check logs with: make docker-logs"

docker-run-prod: ## Run in production mode with nginx
	@echo "Starting production services..."
	@docker-compose --profile production up -d
	@echo "Production services started"

docker-stop: ## Stop Docker Compose services
	@echo "Stopping services..."
	@docker-compose down
	@echo "Services stopped"

docker-logs: ## View Docker Compose logs
	@docker-compose logs -f

docker-clean: ## Clean Docker images and containers
	@echo "Cleaning Docker resources..."
	@docker-compose down --rmi all --volumes --remove-orphans
	@docker system prune -f
	@echo "Docker cleanup completed"

# Release targets
release: ## Create a release build for multiple platforms
	@echo "Creating release builds..."
	@mkdir -p dist
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 .
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 .
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 .
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 .
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe .
	@echo "Release builds created in dist/"

package: release ## Create release packages
	@echo "Creating release packages..."
	@cd dist && for binary in *; do \
		if [ "$$binary" != "*.tar.gz" ]; then \
			tar czf "$$binary.tar.gz" "$$binary" ../README.md ../config.toml.example; \
		fi; \
	done
	@echo "Release packages created"

# Setup targets
setup: ## Setup development environment
	@echo "Setting up development environment..."
	@go install github.com/cosmtrek/air@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@echo "Development tools installed"

init-config: ## Initialize configuration file
	@echo "Initializing configuration..."
	@if [ ! -f config.toml ]; then \
		cp config.toml.example config.toml; \
		echo "Configuration file created: config.toml"; \
		echo "Please edit config.toml before running the application"; \
	else \
		echo "Configuration file already exists: config.toml"; \
	fi

# Utility targets
logs: ## View application logs
	@tail -f logs/combined.log

status: ## Check service status
	@curl -s http://localhost:3000/health | jq . || echo "Service not running"

version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Go Version: $(shell go version)"

# CI/CD targets
ci: fmt vet lint security test ## Run all CI checks

pre-commit: fmt vet test ## Run pre-commit checks

# Example usage:
# make build          # Build the application
# make run            # Run the application
# make docker         # Build Docker image
# make docker-run     # Run with Docker Compose
# make test           # Run tests
# make release        # Create release builds