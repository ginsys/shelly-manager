.PHONY: build build-manager build-provisioner run run-provisioner clean docker-build docker-build-manager docker-build-provisioner docker-run docker-run-prod docker-stop docker-logs docker-pull docker-dev docker-clean dev-setup deps deps-tidy \
	lint lint-fix format format-check hooks-install hooks-uninstall \
	test test-unit test-integration test-full test-full-short \
	test-race test-race-short test-race-full \
	test-coverage test-coverage-short test-coverage-full test-coverage-ci test-coverage-check test-coverage-with-check \
	test-matrix test-ci \
	ui-dev ui-build ui-preview test-e2e test-e2e-dev test-e2e-dev-ui test-e2e-dev-headed test-smoke validate-integration \
	test-quick test-critical test-unit-fast test-full-fast test-ci-fast test-e2e-parallel test-env-smoke test-env-integration test-comprehensive \
	benchmark test-watch example-list example-discover example-provision example-provisioner-status example-provisioner-scan example-provisioner-provision

BINARY_NAME=shelly-manager
PROVISIONER_BINARY=shelly-provisioner
BUILD_DIR=bin
# Docker configuration
REGISTRY=ghcr.io/ginsys
MANAGER_IMAGE=shelly-manager
PROVISIONER_IMAGE=shelly-provisioner
DOCKER_TAG=latest

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
	SHELLY_DEV_EXPOSE_ADMIN_KEY=1 go run ./cmd/shelly-manager server

# One command to refresh UI if needed, then run server
.PHONY: start
start:
	@echo "Checking if UI build is up to date..."
	@if [ ! -f ui/dist/index.html ]; then \
			echo "UI dist missing; building UI..."; \
			$(MAKE) ui-build; \
		else \
			DIST_TS=$$(stat -c %Y ui/dist/index.html 2>/dev/null || stat -f %m ui/dist/index.html); \
			CHANGED=$$(find ui/src ui/index.html ui/vite.config.ts ui/package.json ui/package-lock.json -type f -newermt @$${DIST_TS} | head -n 1); \
			if [ -n "$$CHANGED" ]; then \
				echo "UI sources changed since last build; rebuilding UI..."; \
				$(MAKE) ui-build; \
			else \
				echo "UI is up to date."; \
			fi; \
		fi
	$(MAKE) run

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
	@if [ "$(shell uname)" = "Darwin" ] && [ "$(shell go version | grep -o 'go1\.22')" = "go1.22" ]; then \
		echo "macOS Go 1.22 detected - suppressing linker warnings"; \
		CGO_ENABLED=1 CGO_LDFLAGS="-Wl,-w" go test -v -race -short ./...; \
	else \
		CGO_ENABLED=1 go test -v -race -short ./...; \
	fi

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

# Build Docker images locally
docker-build: docker-build-manager docker-build-provisioner

# Build Manager Docker image locally
docker-build-manager:
	docker build -f deploy/docker/Dockerfile.manager -t $(REGISTRY)/$(MANAGER_IMAGE):$(DOCKER_TAG) .

# Build Provisioner Docker image locally
docker-build-provisioner:
	docker build -f deploy/docker/Dockerfile.provisioner -t $(REGISTRY)/$(PROVISIONER_IMAGE):$(DOCKER_TAG) .

# Run with Docker Compose (development)
docker-run:
	cd deploy/docker-compose && docker-compose -f docker-compose.yml -f docker-compose.dev.yml up -d

# Run with Docker Compose (production)
docker-run-prod:
	cd deploy/docker-compose && docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d

# Stop Docker Compose
docker-stop:
	cd deploy/docker-compose && docker-compose down

# View logs
docker-logs:
	cd deploy/docker-compose && docker-compose logs -f

# Pull latest images from registry
docker-pull:
	cd deploy/docker-compose && docker-compose pull

# Build and run locally (development)
docker-dev: docker-build docker-run

# Clean up Docker containers and images
docker-clean:
	cd deploy/docker-compose && docker-compose down -v --remove-orphans
	docker system prune -f

# ==============================================================================
# DEVELOPMENT AND SETUP
# ==============================================================================

# Development setup
dev-setup:
	go mod tidy
	mkdir -p $(BUILD_DIR) data
	@echo "Installing git pre-commit hooks..."
	$(MAKE) hooks-install

# Clean build artifacts and test outputs
clean:
	# Build artifacts
	rm -rf $(BUILD_DIR)/
	# Stray root binaries (from ad-hoc builds)
	rm -f shelly-manager shelly-provisioner shelly-manager.exe
	# Coverage artifacts
	rm -f coverage.out coverage.html
	rm -f coverage_*.out *_coverage.out
	# CI/JUnit and logs
	rm -f junit-*.xml ci-test.log matrix-test-*.log windows-test-*.log
	# Temporary folders used by local tooling and CI downloads
	rm -rf .tmp/ tmp/

# More aggressive cleanup including Go build caches and (optionally) local data
.PHONY: clean-all
clean-all: clean
	go clean -cache -testcache >/dev/null 2>&1 || true
	# Uncomment the next line if you also want to clear module cache
	# go clean -modcache || true
	# Local data (kept by default). Uncomment to wipe local DBs
	# rm -rf data/

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
# ==============================================================================
# UI (Vite) COMMANDS
# ==============================================================================

# Install UI dependencies (npm install / ci)
ui-deps:
	@echo "Installing UI dependencies..."
	@cd ui && if [ -f package-lock.json ]; then npm ci; else npm install; fi

# Run the SPA dev server (Vite)
ui-dev: ui-deps
	@echo "Starting Vite dev server for UI..."
	@cd ui && npm run dev

# Build the SPA to ui/dist
ui-build: ui-deps
	@echo "Building UI with Vite..."
	@cd ui && npm run build

# Preview the built UI from ui/dist (Vite preview server)
ui-preview:
	@echo "Previewing built UI..."
	@cd ui && npm run preview

# Run E2E tests with fresh database (all browsers)
test-e2e:
	@./scripts/test-e2e.sh

# Run E2E tests with development configuration (Chromium-only, 10-15 min target)
test-e2e-dev:
	@echo "Running E2E tests with development configuration (Chromium-only)..."
	@echo "Expected runtime: 10-15 minutes (vs 4+ hours for full browser suite)"
	@./scripts/test-e2e.sh dev

# Run E2E tests with development configuration in UI mode (interactive debugging)
test-e2e-dev-ui:
	@echo "Starting E2E tests in UI mode with development configuration..."
	@cd ui && npm run test:e2e:dev:ui

# Run E2E tests with development configuration in headed mode (visible browser)
test-e2e-dev-headed:
	@echo "Running E2E tests in headed mode with development configuration..."
	@cd ui && npm run test:e2e:dev:headed

# Run smoke tests for quick feedback (essential tests only)
test-smoke:
	@./scripts/test-smoke.sh

# Validate E2E integration pipeline after configuration changes
validate-integration:
	@./scripts/validate-integration.sh

# ==============================================================================
# OPTIMIZED TEST TARGETS (E2E Performance Improvements)
# ==============================================================================

# Quick test suite (smoke + critical tests, optimized for speed)
test-quick:
	@echo "Running quick test suite (smoke + critical tests)..."
	@cd ui && npm run test:e2e:quick

# Critical path tests only (most important functionality)
test-critical:
	@echo "Running critical path tests..."
	@cd ui && npm run test:e2e:critical

# Database-optimized unit tests (uses in-memory SQLite with GORM optimizations)
test-unit-fast:
	@echo "Running unit tests with database optimizations..."
	SHELLY_SECURITY_VALIDATION_TEST_MODE=true CGO_ENABLED=1 go test -v -short ./internal/...

# Full optimized test suite (all tests with performance optimizations)
test-full-fast:
	@echo "Running full optimized test suite..."
	SHELLY_SECURITY_VALIDATION_TEST_MODE=true CGO_ENABLED=1 go test -v -short -timeout=3m ./...

# Fast CI test suite (optimized for CI environments)
test-ci-fast:
	@echo "Running fast CI test suite with all optimizations..."
	@echo "Step 1/4: Installing dependencies..."
	$(MAKE) deps
	@echo "Step 2/4: Running optimized tests with coverage..."
	SHELLY_SECURITY_VALIDATION_TEST_MODE=true CGO_ENABLED=1 go test -v -race -short -coverprofile=coverage.out -covermode=atomic ./...
	@echo "Step 3/4: Checking coverage threshold..."
	$(MAKE) test-coverage-check
	@echo "Step 4/4: Running linting..."
	$(MAKE) lint-ci
	@echo "✅ All fast CI tests passed! Ready to commit."

# E2E tests with parallel execution and browser optimizations
test-e2e-parallel:
	@echo "Running E2E tests with parallel execution..."
	@cd ui && PLAYWRIGHT_WORKERS=2 npx playwright test --workers=2

# Test runner for different environments
test-env-smoke:
	@echo "Running smoke tests for environment validation..."
	@./scripts/test-smoke.sh

test-env-integration:
	@echo "Running integration validation..."
	@./scripts/validate-integration.sh

# Comprehensive test with all optimizations enabled
test-comprehensive:
	@echo "Running comprehensive test suite with all optimizations..."
	@echo "=== Backend Tests (Optimized) ==="
	$(MAKE) test-unit-fast
	@echo "=== E2E Tests (Parallel) ==="
	$(MAKE) test-e2e-parallel
	@echo "=== Smoke Tests ==="
	$(MAKE) test-smoke
	@echo "✅ All comprehensive tests completed!"
