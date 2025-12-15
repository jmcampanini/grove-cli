BINARY_NAME := grove
BUILD_DIR := build

.PHONY: all build check lint test clean help

all: build ## Default target

build: check ## Build the binary
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) .

check: lint test ## Run all quality checks

lint: ## Run golangci-lint
	golangci-lint run ./...

test: ## Run tests
	go test ./...

clean: ## Remove build artifacts
	go clean
	rm -rf $(BUILD_DIR)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-10s %s\n", $$1, $$2}'
