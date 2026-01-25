.PHONY: build test install clean build-all lint fmt

VERSION := 0.1.0
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
CGO_TAGS := -tags "fts5"
BINARY := edi

# Default target
all: build

# Build for current platform
build:
	go build $(CGO_TAGS) $(LDFLAGS) -o bin/$(BINARY) ./cmd/edi

# Run tests
test:
	go test $(CGO_TAGS) -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Install to ~/.local/bin
install: build
	mkdir -p ~/.local/bin
	cp bin/$(BINARY) ~/.local/bin/

# Install globally (requires sudo)
install-global: build
	sudo cp bin/$(BINARY) /usr/local/bin/

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Cross-compilation for all supported platforms
build-all:
	GOOS=darwin GOARCH=amd64 go build $(CGO_TAGS) $(LDFLAGS) -o bin/$(BINARY)-darwin-amd64 ./cmd/edi
	GOOS=darwin GOARCH=arm64 go build $(CGO_TAGS) $(LDFLAGS) -o bin/$(BINARY)-darwin-arm64 ./cmd/edi
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build $(CGO_TAGS) $(LDFLAGS) -o bin/$(BINARY)-linux-amd64 ./cmd/edi

# Run linter
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...

# Tidy dependencies
tidy:
	go mod tidy

# Verify dependencies
verify:
	go mod verify

# Generate (for future use with go generate)
generate:
	go generate ./...

# Development build (no optimizations)
dev:
	go build $(CGO_TAGS) -o bin/$(BINARY) ./cmd/edi

# Help
help:
	@echo "Available targets:"
	@echo "  build       - Build for current platform"
	@echo "  build-all   - Build for all supported platforms"
	@echo "  test        - Run tests"
	@echo "  install     - Install to ~/.local/bin"
	@echo "  clean       - Remove build artifacts"
	@echo "  lint        - Run linter"
	@echo "  fmt         - Format code"
	@echo "  tidy        - Tidy dependencies"
