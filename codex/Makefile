.PHONY: build test lint clean deps qdrant-up qdrant-down models install

# Build settings
BINARY_MCP=recall-mcp
BINARY_WEB=codex-web
BINARY_CLI=codex-cli
BUILD_DIR=bin
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"

# Go settings
GOFLAGS=-tags "fts5"

# Default target
all: build

# Build all binaries
build: build-mcp build-web build-cli

build-mcp:
	go build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_MCP) ./cmd/recall-mcp

build-web:
	go build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_WEB) ./cmd/codex-web

build-cli:
	go build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_CLI) ./cmd/codex-cli

# Install binaries to GOPATH/bin
install: build
	cp $(BUILD_DIR)/$(BINARY_MCP) $(GOPATH)/bin/
	cp $(BUILD_DIR)/$(BINARY_WEB) $(GOPATH)/bin/
	cp $(BUILD_DIR)/$(BINARY_CLI) $(GOPATH)/bin/

# Run tests
test:
	go test $(GOFLAGS) -v ./...

test-coverage:
	go test $(GOFLAGS) -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Lint
lint:
	golangci-lint run ./...

# Dependencies
deps:
	go mod download
	go mod tidy

# Qdrant management
qdrant-up:
	docker-compose up -d qdrant

qdrant-down:
	docker-compose down

qdrant-logs:
	docker-compose logs -f qdrant

# Download ONNX models for reranking
models:
	@echo "Downloading BGE reranker models..."
	@mkdir -p models/bge-reranker-base models/bge-reranker-v2-m3
	@echo "Note: You need to manually download ONNX models from HuggingFace"
	@echo "See: https://huggingface.co/BAAI/bge-reranker-base"
	@echo "See: https://huggingface.co/BAAI/bge-reranker-v2-m3"

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Development helpers
dev-mcp: build-mcp
	./$(BUILD_DIR)/$(BINARY_MCP)

dev-web: build-web
	./$(BUILD_DIR)/$(BINARY_WEB)

# Run migrations
migrate:
	./$(BUILD_DIR)/$(BINARY_CLI) migrate

# Format code
fmt:
	go fmt ./...
	goimports -w .

# Generate (if needed)
generate:
	go generate ./...
