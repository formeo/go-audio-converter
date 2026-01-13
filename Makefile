# go-audio-converter Makefile

BINARY_NAME := audioconv
GO := go

.PHONY: all build clean test

all: build

build: ## Build the binary
	$(GO) build -o $(BINARY_NAME) ./cmd/audioconv

build-windows: ## Build for Windows
	GOOS=windows GOARCH=amd64 $(GO) build -o $(BINARY_NAME).exe ./cmd/audioconv

build-linux: ## Build for Linux
	GOOS=linux GOARCH=amd64 $(GO) build -o $(BINARY_NAME)-linux ./cmd/audioconv

build-mac: ## Build for macOS
	GOOS=darwin GOARCH=amd64 $(GO) build -o $(BINARY_NAME)-mac ./cmd/audioconv

test: ## Run tests
	$(GO) test -v ./...

clean: ## Clean build artifacts
	rm -f $(BINARY_NAME) $(BINARY_NAME).exe $(BINARY_NAME)-linux $(BINARY_NAME)-mac

deps: ## Download dependencies
	$(GO) mod tidy

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'
