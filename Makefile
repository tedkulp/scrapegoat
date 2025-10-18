.PHONY: help build clean install test fmt vet lint tidy all release

.DEFAULT_GOAL := help

## help: Show this help message
help:
	@echo "Scrapegoat - ROM Scraper Makefile"
	@echo ""
	@echo "Available targets:"
	@awk '/^##/ { desc = substr($$0, 4) } /^[a-zA-Z_-]+:/ && desc { split(desc, parts, ":"); printf "  \033[36m%-15s\033[0m %s\n", $$1, substr(desc, length(parts[1])+3); desc = "" }' $(MAKEFILE_LIST)

## build: Build the scrapegoat binary
build:
	@echo "Building scrapegoat..."
	go build -o scrapegoat cmd/scraper/main.go
	@echo "Build complete: ./scrapegoat"

## clean: Remove built binaries and cache
clean:
	@echo "Cleaning up..."
	rm -f scrapegoat
	rm -rf ~/.scrapegoat/cache
	@echo "Clean complete"

## install: Install scrapegoat to GOPATH/bin
install:
	@echo "Installing scrapegoat..."
	go install ./cmd/scraper
	@echo "Install complete"

## test: Run all tests
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "Test complete"

## fmt: Format Go code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "Format complete"

## vet: Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...
	@echo "Vet complete"

## lint: Run golangci-lint (requires golangci-lint installed)
lint:
	@echo "Running golangci-lint..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Install: https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run ./...
	@echo "Lint complete"

## tidy: Tidy and verify Go modules
tidy:
	@echo "Tidying modules..."
	go mod tidy
	go mod verify
	@echo "Tidy complete"

## user-info: Display ScreenScraper user account and API quota information
user-info: build
	@echo "Fetching user info..."
	./scrapegoat user-info

## all: Format, vet, test, and build
all: fmt vet test build
	@echo "All tasks complete"

## release: Create and push a git tag for GoReleaser (reads version from main.go)
release:
	@echo "Preparing release..."
	@VERSION=$$(grep '^[[:space:]]*version = ' cmd/scraper/main.go | sed 's/.*= "\(.*\)"/\1/'); \
	if [ -z "$$VERSION" ] || [ "$$VERSION" = "dev" ]; then \
		echo "Error: Invalid version '$$VERSION' in cmd/scraper/main.go"; \
		echo "Please update the version variable to a valid version (e.g., \"1.0.0\")"; \
		exit 1; \
	fi; \
	TAG="v$$VERSION"; \
	echo "Version: $$VERSION"; \
	echo "Tag: $$TAG"; \
	echo ""; \
	if [ -n "$$(git status --porcelain)" ]; then \
		echo "Error: Working directory is not clean. Please commit or stash changes."; \
		git status --short; \
		exit 1; \
	fi; \
	if git rev-parse "$$TAG" >/dev/null 2>&1; then \
		echo "Error: Tag $$TAG already exists"; \
		exit 1; \
	fi; \
	echo "Creating tag $$TAG..."; \
	git tag -a "$$TAG" -m "Release $$TAG"; \
	echo "Pushing tag $$TAG to origin..."; \
	git push origin "$$TAG"; \
	echo ""; \
	echo "Release $$TAG created and pushed successfully!"; \
	echo "GitHub Actions will now build and publish the release."
