# go-audio-converter Makefile
# ============================

.PHONY: all build test lint clean install docker help

# Variables
BINARY_NAME := audioconv
VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

LDFLAGS := -ldflags="-s -w \
	-X main.version=$(VERSION) \
	-X main.buildTime=$(BUILD_TIME) \
	-X main.gitCommit=$(GIT_COMMIT)"

GO := go
GOFLAGS := -v

# Directories
BIN_DIR := bin
DIST_DIR := dist

# Default target
all: build

# ============================
# Build
# ============================

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/audioconv

build-all: build-linux build-darwin build-windows ## Build for all platforms

build-linux: ## Build for Linux
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/audioconv
	GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/audioconv

build-darwin: ## Build for macOS
	@mkdir -p $(DIST_DIR)
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/audioconv
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/audioconv

build-windows: ## Build for Windows
	@mkdir -p $(DIST_DIR)
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/audioconv

build-wasm: ## Build for WebAssembly
	@mkdir -p $(DIST_DIR)/wasm
	GOOS=js GOARCH=wasm $(GO) build $(LDFLAGS) -o $(DIST_DIR)/wasm/audioconv.wasm ./cmd/audioconv
	cp "$$(go env GOROOT)/misc/wasm/wasm_exec.js" $(DIST_DIR)/wasm/

# ============================
# Development
# ============================

run: build ## Build and run
	./$(BIN_DIR)/$(BINARY_NAME) $(ARGS)

serve: build ## Run HTTP server
	./$(BIN_DIR)/$(BINARY_NAME) serve

dev: ## Run with hot reload (requires air)
	air

install: build ## Install to GOPATH/bin
	cp $(BIN_DIR)/$(BINARY_NAME) $(GOPATH)/bin/

# ============================
# Testing
# ============================

test: ## Run tests
	$(GO) test -v ./...

test-cover: ## Run tests with coverage
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-race: ## Run tests with race detector
	$(GO) test -v -race ./...

bench: ## Run benchmarks
	$(GO) test -bench=. -benchmem ./pkg/converter/

# ============================
# Code Quality
# ============================

lint: ## Run linter
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

fmt: ## Format code
	$(GO) fmt ./...
	gofmt -s -w .

vet: ## Run go vet
	$(GO) vet ./...

check: fmt vet lint test ## Run all checks

# ============================
# Dependencies
# ============================

deps: ## Download dependencies
	$(GO) mod download

deps-update: ## Update dependencies
	$(GO) get -u ./...
	$(GO) mod tidy

deps-tidy: ## Tidy dependencies
	$(GO) mod tidy

# ============================
# Docker
# ============================

docker-build: ## Build Docker image
	docker build -t $(BINARY_NAME):$(VERSION) -t $(BINARY_NAME):latest .

docker-run: ## Run Docker container
	docker run --rm -it -v $(PWD):/data $(BINARY_NAME):latest $(ARGS)

docker-serve: ## Run HTTP server in Docker
	docker run --rm -it -p 8080:8080 $(BINARY_NAME):latest serve --host 0.0.0.0

docker-push: ## Push to Docker Hub
	docker tag $(BINARY_NAME):latest formeo/$(BINARY_NAME):$(VERSION)
	docker tag $(BINARY_NAME):latest formeo/$(BINARY_NAME):latest
	docker push formeo/$(BINARY_NAME):$(VERSION)
	docker push formeo/$(BINARY_NAME):latest

# ============================
# Release
# ============================

release: clean build-all ## Create release archives
	@mkdir -p $(DIST_DIR)/release
	@cd $(DIST_DIR) && for f in $(BINARY_NAME)-*; do \
		if [ -f "$$f" ]; then \
			tar -czf release/$$f.tar.gz $$f; \
			rm $$f; \
		fi \
	done
	@echo "Release archives created in $(DIST_DIR)/release/"

# ============================
# Cleanup
# ============================

clean: ## Clean build artifacts
	rm -rf $(BIN_DIR) $(DIST_DIR)
	rm -f coverage.out coverage.html

# ============================
# Help
# ============================

help: ## Show this help
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
