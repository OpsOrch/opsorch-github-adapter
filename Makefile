GO ?= go
GOCACHE ?= $(PWD)/.gocache
GOMODCACHE ?= $(PWD)/.gocache/mod
CACHE_ENV = GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE)

.PHONY: all clean test build plugins ticket-plugin deployment-plugin integ integ-ticket integ-deployment fmt deps lint

# Default target
all: build plugins

# Build plugins
plugins: ticket-plugin deployment-plugin

# Build ticket plugin
ticket-plugin:
	@echo "Building GitHub ticket plugin..."
	@mkdir -p bin
	$(CACHE_ENV) $(GO) build -o bin/ticketplugin ./cmd/ticketplugin

# Build deployment plugin  
deployment-plugin:
	@echo "Building GitHub deployment plugin..."
	@mkdir -p bin
	$(CACHE_ENV) $(GO) build -o bin/deploymentplugin ./cmd/deploymentplugin

# Build library (for in-process use)
build:
	@echo "Building GitHub adapter library..."
	$(CACHE_ENV) $(GO) build ./...

# Run unit tests
test:
	@echo "Running unit tests..."
	$(CACHE_ENV) $(GO) test ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

# Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GO) mod tidy
	$(GO) mod download

# Lint code
lint:
	@echo "Running linter..."
	golangci-lint run

# Run ticket integration tests
integ-ticket:
	@echo "Running GitHub ticket integration tests..."
	@if [ -z "$(GITHUB_TOKEN)" ]; then \
		echo "GITHUB_TOKEN environment variable is required for integration tests"; \
		echo "Set GITHUB_OWNER and GITHUB_REPO to override defaults (opsorch/opsorch-github-adapter)"; \
		exit 1; \
	fi
	$(CACHE_ENV) $(GO) run ./integ/ticket.go

# Run deployment integration tests
integ-deployment:
	@echo "Running GitHub deployment integration tests..."
	@if [ -z "$(GITHUB_TOKEN)" ]; then \
		echo "GITHUB_TOKEN environment variable is required for integration tests"; \
		echo "Set GITHUB_OWNER and GITHUB_REPO to override defaults (opsorch/opsorch-github-adapter)"; \
		exit 1; \
	fi
	$(CACHE_ENV) $(GO) run ./integ/deployment.go

# Run all integration tests
integ: integ-ticket integ-deployment

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(GOCACHE) bin/