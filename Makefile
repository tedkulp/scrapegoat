.PHONY: help build clean install test fmt vet lint run-dry run-verbose tidy

# Default target - show help
help:
	@echo "Scrapegoat - ROM Scraper Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  make build        - Build the scrapegoat binary"
	@echo "  make clean        - Remove built binaries and cache"
	@echo "  make install      - Install scrapegoat to GOPATH/bin"
	@echo "  make test         - Run all tests"
	@echo "  make fmt          - Format Go code"
	@echo "  make vet          - Run go vet"
	@echo "  make lint         - Run golangci-lint (requires golangci-lint installed)"
	@echo "  make tidy         - Tidy and verify Go modules"
	@echo "  make all          - Format, vet, test, and build"

# Build the binary
build:
	@echo "Building scrapegoat..."
	go build -o scrapegoat cmd/scraper/main.go
	@echo "Build complete: ./scrapegoat"

# Remove built binaries and cache
clean:
	@echo "Cleaning up..."
	rm -f scrapegoat
	rm -rf ~/.scrapegoat/cache
	@echo "Clean complete"

# Install to GOPATH/bin
install:
	@echo "Installing scrapegoat..."
	go install ./cmd/scraper
	@echo "Install complete"

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "Test complete"

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "Format complete"

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...
	@echo "Vet complete"

# Run golangci-lint (requires golangci-lint to be installed)
lint:
	@echo "Running golangci-lint..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Install: https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run ./...
	@echo "Lint complete"

# Tidy and verify modules
tidy:
	@echo "Tidying modules..."
	go mod tidy
	go mod verify
	@echo "Tidy complete"

# Build, format, vet, and test
all: fmt vet test build
	@echo "All tasks complete"
