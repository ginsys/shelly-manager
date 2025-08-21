.PHONY: build build-manager build-provisioner run run-provisioner clean docker-build docker-run docker-stop docker-logs dev-setup deps deps-tidy \
	lint lint-fix format format-check hooks-install hooks-uninstall \
	test test-unit test-integration test-full test-full-short \
	test-race test-race-short test-race-full \
	test-coverage test-coverage-short test-coverage-full test-coverage-ci test-coverage-check test-coverage-with-check \
	test-matrix test-ci \
	benchmark test-watch example-list example-discover example-provision example-provisioner-status example-provisioner-scan example-provisioner-provision

BINARY_NAME=shelly-manager
PROVISIONER_BINARY=shelly-provisioner
BUILD_DIR=bin
DOCKER_IMAGE=shelly-manager

# ==============================================================================
# BUILD COMMANDS
# ==============================================================================

# Build both applications
build: build-manager build-provisioner

# Build the main manager application
build-manager:
	CGO_ENABLED=1 go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/shelly-manager

# Build the provisioner application
build-provisioner:
	CGO_ENABLED=1 go build -o $(BUILD_DIR)/$(PROVISIONER_BINARY) ./cmd/shelly-provisioner

# Run the main manager application
run:
	go run ./cmd/shelly-manager server

# Run the provisioner application
run-provisioner:
	go run ./cmd/shelly-provisioner status

# ==============================================================================
# BASIC TEST COMMANDS
# ==============================================================================

# Run basic tests (fast mode, skips network tests)
test:
	CGO_ENABLED=1 go test -v -short ./...

# Run unit tests only (internal packages)
test-unit:
	CGO_ENABLED=1 go test -v -short ./internal/...

# Run integration tests (cmd packages)
test-integration:
	CGO_ENABLED=1 go test -v -short ./cmd/...

# Run full test suite including network tests (slower, with timeout)
test-full:
	CGO_ENABLED=1 go test -v -timeout=5m ./...

# Run full test suite with short flag (faster, skips network tests)
test-full-short:
	CGO_ENABLED=1 go test -v -short -timeout=5m ./...

# ==============================================================================
# RACE DETECTION TESTS
# ==============================================================================

# Run tests with race detection (short mode, default)
test-race:
	CGO_ENABLED=1 go test -v -short -race ./...

# Run tests with race detection (short mode, explicit)
test-race-short:
	CGO_ENABLED=1 go test -v -short -race ./...

# Run full tests with race detection (including network tests, with timeout)
test-race-full:
	CGO_ENABLED=1 go test -v -race -timeout=10m ./...

# ==============================================================================
# COVERAGE TESTS
# ==============================================================================

# Run tests with coverage report (short mode)
test-coverage:
	CGO_ENABLED=1 go test -v -short -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with coverage report (short mode, explicit)
test-coverage-short:
	CGO_ENABLED=1 go test -v -short -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Short coverage report generated: coverage.html"

# Run full coverage including network tests (with timeout)
test-coverage-full:
	CGO_ENABLED=1 go test -v -timeout=5m -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Full coverage report generated: coverage.html"

# Run tests with coverage for CI (race detection + atomic mode)
test-coverage-ci:
	CGO_ENABLED=1 go test -v -race -short -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"
	@echo "Coverage summary:"
	go tool cover -func=coverage.out | tail -1

# Check coverage threshold (27.5% minimum) - requires existing coverage.out
test-coverage-check:
	@COVERAGE=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "Total coverage: $${COVERAGE}%"; \
	THRESHOLD=27.5; \
	if [ "$$(echo "$$COVERAGE < $$THRESHOLD" | bc -l)" = "1" ]; then \
		echo "Coverage $${COVERAGE}% is below threshold $${THRESHOLD}%"; \
		exit 1; \
	fi; \
	echo "Coverage $${COVERAGE}% meets threshold $${THRESHOLD}%"

# Generate coverage and check threshold in one step
test-coverage-with-check: test-coverage-ci test-coverage-check

# ==============================================================================
# CI/MATRIX TESTS
# ==============================================================================

# Run matrix tests (race detection, short mode)
test-matrix:
	@echo "Running matrix tests with race detection and short mode..."
	CGO_ENABLED=1 go test -v -race -short ./...

# Complete CI test suite - matches GitHub Actions test.yml workflow exactly
# This is the most important test to run locally before committing
test-ci:
	@echo "Running complete CI test suite (matches GitHub Actions)..."
	@echo "Step 1/4: Installing dependencies..."
	$(MAKE) deps
	@echo "Step 2/4: Running tests with coverage and race detection..."
	$(MAKE) test-coverage-ci
	@echo "Step 3/4: Checking coverage threshold..."
	$(MAKE) test-coverage-check
	@echo "Step 4/4: Running linting..."
	$(MAKE) lint-ci
	@echo "✅ All CI tests passed! Ready to commit."

# ==============================================================================
# LINTING AND QUALITY
# ==============================================================================

# Run comprehensive linting (gofmt, go vet, golangci-lint)
lint:
	@echo "Running go fmt..."
	go fmt ./...
	@echo "Running go vet..."
	go vet ./...
	@echo "Running golangci-lint..."
	golangci-lint run --timeout=5m

# Run golangci-lint exactly as CI does (requires golangci-lint to be installed)
lint-ci:
	@echo "Running golangci-lint (same as CI)..."
	golangci-lint run --timeout=5m

# Run golangci-lint (requires golangci-lint to be installed)
lint-fix:
	@echo "Running golangci-lint with auto-fix..."
	golangci-lint run --fix

# Format code (gofmt + goimports)
format:
	@echo "Running go fmt..."
	go fmt ./...
	@echo "Running goimports..."
	goimports -w .

# Check if code is properly formatted
format-check:
	@echo "Checking if code is properly formatted..."
	@UNFORMATTED=$$(gofmt -l .); \
	if [ -n "$$UNFORMATTED" ]; then \
		echo "The following files are not properly formatted:"; \
		echo "$$UNFORMATTED"; \
		echo "Please run 'make format' to fix them."; \
		exit 1; \
	fi
	@echo "All files are properly formatted."

# Install git pre-commit hook for automatic formatting
hooks-install:
	@echo "Installing git pre-commit hook..."
	@echo '#!/bin/bash' > .git/hooks/pre-commit
	@echo '#' >> .git/hooks/pre-commit
	@echo '# Pre-commit hook for Go code formatting and comprehensive linting' >> .git/hooks/pre-commit
	@echo '# This hook runs the same linting as CI to ensure local-CI parity' >> .git/hooks/pre-commit
	@echo '#' >> .git/hooks/pre-commit
	@echo '' >> .git/hooks/pre-commit
	@echo '# Colors for output' >> .git/hooks/pre-commit
	@echo "RED='\033[0;31m'" >> .git/hooks/pre-commit
	@echo "GREEN='\033[0;32m'" >> .git/hooks/pre-commit
	@echo "YELLOW='\033[1;33m'" >> .git/hooks/pre-commit
	@echo "NC='\033[0m' # No Color" >> .git/hooks/pre-commit
	@echo '' >> .git/hooks/pre-commit
	@echo 'echo -e "$${YELLOW}Running pre-commit checks...$${NC}"' >> .git/hooks/pre-commit
	@echo '' >> .git/hooks/pre-commit
	@echo '# Check if we have Go files in the commit' >> .git/hooks/pre-commit
	@echo 'go_files=$$(git diff --cached --name-only --diff-filter=ACM | grep "\.go$$")' >> .git/hooks/pre-commit
	@echo '' >> .git/hooks/pre-commit
	@echo 'if [ -z "$$go_files" ]; then' >> .git/hooks/pre-commit
	@echo '    echo -e "$${GREEN}No Go files to check.$${NC}"' >> .git/hooks/pre-commit
	@echo '    exit 0' >> .git/hooks/pre-commit
	@echo 'fi' >> .git/hooks/pre-commit
	@echo '' >> .git/hooks/pre-commit
	@echo 'echo -e "$${YELLOW}Checking Go files: $$go_files$${NC}"' >> .git/hooks/pre-commit
	@echo '' >> .git/hooks/pre-commit
	@echo '# Run go fmt and capture any files that need formatting' >> .git/hooks/pre-commit
	@echo 'unformatted=$$(echo "$$go_files" | xargs gofmt -l)' >> .git/hooks/pre-commit
	@echo '' >> .git/hooks/pre-commit
	@echo 'if [ -n "$$unformatted" ]; then' >> .git/hooks/pre-commit
	@echo '    echo -e "$${YELLOW}Auto-formatting Go files...$${NC}"' >> .git/hooks/pre-commit
	@echo '    echo "$$unformatted" | xargs gofmt -w' >> .git/hooks/pre-commit
	@echo '    ' >> .git/hooks/pre-commit
	@echo '    # Run goimports if available' >> .git/hooks/pre-commit
	@echo '    if command -v goimports >/dev/null 2>&1; then' >> .git/hooks/pre-commit
	@echo '        echo -e "$${YELLOW}Auto-formatting imports...$${NC}"' >> .git/hooks/pre-commit
	@echo '        echo "$$go_files" | xargs goimports -w' >> .git/hooks/pre-commit
	@echo '    fi' >> .git/hooks/pre-commit
	@echo '    ' >> .git/hooks/pre-commit
	@echo '    # Add the formatted files back to the commit' >> .git/hooks/pre-commit
	@echo '    echo "$$unformatted" | xargs git add' >> .git/hooks/pre-commit
	@echo '    echo "$$go_files" | xargs git add' >> .git/hooks/pre-commit
	@echo '    ' >> .git/hooks/pre-commit
	@echo '    echo -e "$${GREEN}Code formatted and added to commit.$${NC}"' >> .git/hooks/pre-commit
	@echo 'fi' >> .git/hooks/pre-commit
	@echo '' >> .git/hooks/pre-commit
	@echo '# Run complete lint suite (same as CI)' >> .git/hooks/pre-commit
	@echo 'echo -e "$${YELLOW}Running complete lint suite (same as CI)...$${NC}"' >> .git/hooks/pre-commit
	@echo 'if ! make lint-ci; then' >> .git/hooks/pre-commit
	@echo '    echo -e "$${RED}Linting failed. Please fix issues before committing.$${NC}"' >> .git/hooks/pre-commit
	@echo '    exit 1' >> .git/hooks/pre-commit
	@echo 'fi' >> .git/hooks/pre-commit
	@echo '' >> .git/hooks/pre-commit
	@echo 'echo -e "$${GREEN}Pre-commit checks passed!$${NC}"' >> .git/hooks/pre-commit
	@echo 'exit 0' >> .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "Pre-commit hook installed successfully!"
	@echo "The hook will automatically format Go code and run comprehensive linting (same as CI) before each commit."

# Uninstall git pre-commit hook
hooks-uninstall:
	@echo "Removing git pre-commit hook..."
	@rm -f .git/hooks/pre-commit
	@echo "Pre-commit hook removed."

# ==============================================================================
# BENCHMARK AND PERFORMANCE TESTS
# ==============================================================================

# Run benchmarks (fast mode)
benchmark:
	CGO_ENABLED=1 go test -v -short -bench=. ./...

# Watch mode for tests (requires entr: brew install entr)
test-watch:
	find . -name "*.go" | entr -c make test-unit

# ==============================================================================
# DEPENDENCY MANAGEMENT
# ==============================================================================

# Install dependencies
deps:
	go mod download
	go mod verify

# Install dependencies and tidy
deps-tidy:
	go mod download
	go mod tidy

# ==============================================================================
# DOCKER COMMANDS
# ==============================================================================

# Build Docker image
docker-build:
	docker build -f docker/Dockerfile -t $(DOCKER_IMAGE) .

# Run with Docker Compose
docker-run:
	docker-compose up -d

# Stop Docker Compose
docker-stop:
	docker-compose down

# View logs
docker-logs:
	docker-compose logs -f

# ==============================================================================
# DEVELOPMENT AND SETUP
# ==============================================================================

# Development setup
dev-setup:
	go mod tidy
	mkdir -p $(BUILD_DIR) data
	@echo "Installing git pre-commit hooks..."
	$(MAKE) hooks-install

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)/
	rm -f *.db
	rm -f coverage.out coverage.html
	rm -rf tmp/

# ==============================================================================
# CLI EXAMPLES
# ==============================================================================

# CLI examples - Manager
example-list:
	./$(BUILD_DIR)/$(BINARY_NAME) list

example-discover:
	./$(BUILD_DIR)/$(BINARY_NAME) discover 192.168.1.0/24

example-provision:
	./$(BUILD_DIR)/$(BINARY_NAME) provision

# CLI examples - Provisioner
example-provisioner-status:
	./$(BUILD_DIR)/$(PROVISIONER_BINARY) status

example-provisioner-scan:
	./$(BUILD_DIR)/$(PROVISIONER_BINARY) scan-ap

example-provisioner-provision:
	./$(BUILD_DIR)/$(PROVISIONER_BINARY) provision "MyWiFi" "password123"
