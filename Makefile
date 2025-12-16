.PHONY: help build build-manager build-provisioner run run-provisioner clean docker-build docker-build-manager docker-build-provisioner docker-run docker-run-prod docker-stop docker-logs docker-pull docker-dev docker-clean dev-setup deps deps-tidy \
	lint fix hooks-install hooks-uninstall \
	test test-unit test-integration test-race test-security test-all test-extra test-vitest \
	test-coverage test-coverage-ci test-coverage-check \
	test-ci check-go-version upgrade-go-version \
	ui-dev ui-build ui-preview test-e2e test-e2e-dev test-e2e-dev-ui test-smoke validate-integration \
	benchmark example-list example-discover example-provision example-provisioner-status example-provisioner-scan example-provisioner-provision

BINARY_NAME=shelly-manager
PROVISIONER_BINARY=shelly-provisioner
BUILD_DIR=bin
# Docker configuration
REGISTRY=ghcr.io/ginsys
MANAGER_IMAGE=shelly-manager
PROVISIONER_IMAGE=shelly-provisioner
DOCKER_TAG=latest

# Color definitions for help output
CYAN := \033[1;36m
WHITE := \033[1;37m
YELLOW := \033[0;33m
GREEN := \033[0;32m
NC := \033[0m

# ==============================================================================
# HELP (default target)
# ==============================================================================

help:
	@echo ""
	@echo "$(CYAN)Shelly Manager - Makefile Targets$(NC)"
	@echo "$(CYAN)==================================$(NC)"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "$(CYAN)BUILD$(NC)"
	@echo "  $(WHITE)build$(NC)               Build both binaries $(YELLOW)→ build-manager, build-provisioner$(NC)"
	@echo "  $(WHITE)build-manager$(NC)       Build the manager binary"
	@echo "  $(WHITE)build-provisioner$(NC)   Build the provisioner binary"
	@echo ""
	@echo "$(CYAN)RUN$(NC)"
	@echo "  $(WHITE)run$(NC)                 Run the manager server (dev mode)"
	@echo "  $(WHITE)start$(NC)               Build UI if needed, then run server $(YELLOW)→ ui-build?, run$(NC)"
	@echo "  $(WHITE)run-provisioner$(NC)     Run the provisioner status command"
	@echo ""
	@echo "$(CYAN)GO VERSION MANAGEMENT$(NC)"
	@echo "  $(WHITE)check-go-version$(NC)    Validate Go version consistency across project"
	@echo "  $(WHITE)upgrade-go-version$(NC)  Upgrade Go version (usage: make upgrade-go-version VERSION=X.Y.Z)"
	@echo ""
	@echo "$(CYAN)TESTING$(NC)"
	@echo "  $(WHITE)test$(NC)                Run basic tests (fast mode, skips network tests)"
	@echo "  $(WHITE)test-all$(NC)            Run all local tests (Go+UI+security+lint, ~2-3 min)"
	@echo "  $(WHITE)test-extra$(NC)          Run network tests, E2E, benchmarks (~10-15 min)"
	@echo "  $(WHITE)test-unit$(NC)           Run unit tests only (internal packages)"
	@echo "  $(WHITE)test-integration$(NC)    Run integration tests (cmd packages)"
	@echo "  $(WHITE)test-race$(NC)           Run tests with race detection"
	@echo "  $(WHITE)test-security$(NC)       Run security tests with production settings"
	@echo ""
	@echo "$(CYAN)COVERAGE$(NC)"
	@echo "  $(WHITE)test-coverage$(NC)       Run tests with coverage report (generates coverage.html)"
	@echo "  $(WHITE)test-coverage-ci$(NC)    Run tests with coverage for CI (race + atomic)"
	@echo "  $(WHITE)test-coverage-check$(NC) Check coverage threshold (27.5% minimum)"
	@echo ""
	@echo "$(CYAN)CI$(NC)"
	@echo "  $(WHITE)test-ci$(NC)             Complete CI test suite $(YELLOW)→ check-go-version, deps, test-coverage-ci, test-coverage-check, lint, test-vitest$(NC)"
	@echo "  $(WHITE)test-vitest$(NC)         Run frontend unit tests (vitest)"
	@echo ""
	@echo "$(CYAN)LINTING & QUALITY$(NC)"
	@echo "  $(WHITE)lint$(NC)                Run go vet + golangci-lint"
	@echo "  $(WHITE)fix$(NC)                 Auto-fix formatting and lint issues"
	@echo ""
	@echo "$(CYAN)GIT HOOKS$(NC)"
	@echo "  $(WHITE)hooks-install$(NC)       Install pre-commit hook for formatting/linting"
	@echo "  $(WHITE)hooks-uninstall$(NC)     Remove pre-commit hook"
	@echo ""
	@echo "$(CYAN)BENCHMARKS$(NC)"
	@echo "  $(WHITE)benchmark$(NC)           Run benchmark tests"
	@echo ""
	@echo "$(CYAN)DEPENDENCIES$(NC)"
	@echo "  $(WHITE)deps$(NC)                Download and verify Go modules"
	@echo "  $(WHITE)deps-tidy$(NC)           Download and tidy Go modules"
	@echo ""
	@echo "$(CYAN)DOCKER$(NC)"
	@echo "  $(WHITE)docker-build$(NC)             Build both Docker images $(YELLOW)→ docker-build-manager, docker-build-provisioner$(NC)"
	@echo "  $(WHITE)docker-build-manager$(NC)     Build manager Docker image"
	@echo "  $(WHITE)docker-build-provisioner$(NC) Build provisioner Docker image"
	@echo "  $(WHITE)docker-run$(NC)          Run with Docker Compose (development)"
	@echo "  $(WHITE)docker-run-prod$(NC)     Run with Docker Compose (production)"
	@echo "  $(WHITE)docker-stop$(NC)         Stop Docker Compose"
	@echo "  $(WHITE)docker-logs$(NC)         View Docker Compose logs"
	@echo "  $(WHITE)docker-pull$(NC)         Pull latest images from registry"
	@echo "  $(WHITE)docker-dev$(NC)          Build and run locally $(YELLOW)→ docker-build, docker-run$(NC)"
	@echo "  $(WHITE)docker-clean$(NC)        Clean up Docker containers and images"
	@echo ""
	@echo "$(CYAN)DEVELOPMENT & SETUP$(NC)"
	@echo "  $(WHITE)dev-setup$(NC)           Initial development setup $(YELLOW)→ hooks-install$(NC)"
	@echo "  $(WHITE)clean$(NC)               Clean build artifacts and test outputs"
	@echo "  $(WHITE)clean-all$(NC)           Aggressive cleanup including Go caches $(YELLOW)→ clean$(NC)"
	@echo ""
	@echo "$(CYAN)UI (VITE)$(NC)"
	@echo "  $(WHITE)ui-deps$(NC)             Install UI dependencies"
	@echo "  $(WHITE)ui-dev$(NC)              Run Vite dev server $(YELLOW)→ ui-deps$(NC)"
	@echo "  $(WHITE)ui-build$(NC)            Build UI to ui/dist $(YELLOW)→ ui-deps$(NC)"
	@echo "  $(WHITE)ui-preview$(NC)          Preview built UI"
	@echo ""
	@echo "$(CYAN)E2E TESTING$(NC)"
	@echo "  $(WHITE)test-e2e$(NC)            Run E2E tests (all browsers)"
	@echo "  $(WHITE)test-e2e-dev$(NC)        Run E2E tests (Chromium-only, faster)"
	@echo "  $(WHITE)test-e2e-dev-ui$(NC)     Run E2E tests in UI mode (interactive)"
	@echo "  $(WHITE)test-smoke$(NC)          Run smoke tests for quick feedback"
	@echo "  $(WHITE)validate-integration$(NC) Validate E2E integration pipeline"
	@echo ""
	@echo "$(CYAN)CLI EXAMPLES$(NC)"
	@echo "  $(WHITE)example-list$(NC)        List devices"
	@echo "  $(WHITE)example-discover$(NC)    Discover devices on network"
	@echo "  $(WHITE)example-provision$(NC)   Provision devices"
	@echo "  $(WHITE)example-provisioner-status$(NC)    Show provisioner status"
	@echo "  $(WHITE)example-provisioner-scan$(NC)      Scan for access points"
	@echo "  $(WHITE)example-provisioner-provision$(NC) Provision with WiFi (usage: make ... SSID=x PASS=y)"
	@echo ""
	@echo "$(CYAN)RECOMMENDED WORKFLOWS$(NC)"
	@echo "  test-all           $(YELLOW)→$(NC) Recommended before every commit"
	@echo "  test-extra         $(YELLOW)→$(NC) Network/E2E tests (combine with test-all for full coverage)"
	@echo ""
	@echo "$(CYAN)DEPENDENCY OVERVIEW$(NC)"
	@echo "  build              $(YELLOW)→$(NC) build-manager, build-provisioner"
	@echo "  start              $(YELLOW)→$(NC) ui-build (if needed) $(YELLOW)→$(NC) ui-deps $(YELLOW)→$(NC) run"
	@echo "  test-ci            $(YELLOW)→$(NC) check-go-version $(YELLOW)→$(NC) deps $(YELLOW)→$(NC) test-coverage-ci $(YELLOW)→$(NC) test-coverage-check $(YELLOW)→$(NC) lint $(YELLOW)→$(NC) test-vitest"
	@echo "  test-coverage      $(YELLOW)→$(NC) generates coverage.out, coverage.html"
	@echo "  test-coverage-ci   $(YELLOW)→$(NC) generates coverage.out, coverage.html"
	@echo "  test-coverage-check $(YELLOW)→$(NC) requires coverage.out"
	@echo "  docker-build       $(YELLOW)→$(NC) docker-build-manager, docker-build-provisioner"
	@echo "  docker-dev         $(YELLOW)→$(NC) docker-build $(YELLOW)→$(NC) docker-run"
	@echo "  dev-setup          $(YELLOW)→$(NC) hooks-install"
	@echo "  clean-all          $(YELLOW)→$(NC) clean"
	@echo "  ui-dev             $(YELLOW)→$(NC) ui-deps"
	@echo "  ui-build           $(YELLOW)→$(NC) ui-deps"
	@echo ""

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
# GO VERSION MANAGEMENT
# ==============================================================================

# Check Go version consistency across all project files
check-go-version:
	@./scripts/check-go-version.sh

# Upgrade Go version in all project files (usage: make upgrade-go-version VERSION=1.24.0)
upgrade-go-version:
	@if [ -z "$(VERSION)" ]; then \
		echo "Usage: make upgrade-go-version VERSION=1.24.0"; \
		exit 1; \
	fi
	@./scripts/upgrade-go-version.sh $(VERSION)

# ==============================================================================
# BASIC TEST COMMANDS
# ==============================================================================

# Run basic tests (fast mode, skips network tests, test mode enabled)
test:
	SHELLY_SECURITY_VALIDATION_TEST_MODE=true CGO_ENABLED=1 go test -v -short ./...

# Run unit tests only (internal packages)
test-unit:
	SHELLY_SECURITY_VALIDATION_TEST_MODE=true CGO_ENABLED=1 go test -v -short ./internal/...

# Run integration tests (cmd packages)
test-integration:
	SHELLY_SECURITY_VALIDATION_TEST_MODE=true CGO_ENABLED=1 go test -v -short ./cmd/...

# Run tests with race detection
test-race:
	SHELLY_SECURITY_VALIDATION_TEST_MODE=true CGO_ENABLED=1 go test -v -short -race ./...

# Run security-focused tests WITHOUT test mode (validates actual security behavior)
test-security:
	@echo "Running security tests with production-like security settings..."
	CGO_ENABLED=1 go test -v -short -run "Security|Auth|Validation" ./...

# ==============================================================================
# COVERAGE TESTS
# ==============================================================================

# Run tests with coverage report (local development)
test-coverage:
	SHELLY_SECURITY_VALIDATION_TEST_MODE=true CGO_ENABLED=1 go test -v -short -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with coverage for CI (race detection + atomic mode)
# NOTE: Does NOT use test mode - runs against full security router to test all routes
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

# ==============================================================================
# CI TESTS
# ==============================================================================

# Frontend unit tests (vitest)
test-vitest:
	@echo "Running frontend unit tests (vitest)..."
	cd ui && npx vitest run --coverage

# Complete CI test suite - matches GitHub Actions test.yml workflow exactly
# This is the most important test to run locally before committing
test-ci:
	@echo "Running complete CI test suite (matches GitHub Actions)..."
	@echo "Step 1/6: Validating Go version consistency..."
	$(MAKE) check-go-version
	@echo "Step 2/6: Installing dependencies..."
	$(MAKE) deps
	@echo "Step 3/6: Running tests with coverage and race detection..."
	$(MAKE) test-coverage-ci
	@echo "Step 4/6: Checking coverage threshold..."
	$(MAKE) test-coverage-check
	@echo "Step 5/6: Running linting..."
	$(MAKE) lint
	@echo "Step 6/6: Running frontend unit tests (vitest)..."
	$(MAKE) test-vitest
	@echo "✅ All CI tests passed! Ready to commit."

# ==============================================================================
# COMPREHENSIVE LOCAL TEST TARGETS
# ==============================================================================

# Run all important tests locally (fast, no network, ~2-3 min)
# Use this before committing - covers Go, UI, security, lint
test-all:
	@echo "=== Running comprehensive local tests (~2-3 min) ==="
	@echo ""
	@echo "Step 1/6: Validating Go version..."
	$(MAKE) check-go-version
	@echo ""
	@echo "Step 2/6: Running Go tests with race detection + coverage..."
	SHELLY_SECURITY_VALIDATION_TEST_MODE=true CGO_ENABLED=1 go test -race -short -coverprofile=coverage.out -covermode=atomic ./...
	@echo ""
	@echo "Step 3/6: Running security tests (production settings)..."
	$(MAKE) test-security
	@echo ""
	@echo "Step 4/6: Running frontend unit tests (vitest)..."
	cd ui && npx vitest run --coverage
	@echo ""
	@echo "Step 5/6: Running Go lint..."
	$(MAKE) lint
	@echo ""
	@echo "Step 6/6: Checking coverage threshold..."
	$(MAKE) test-coverage-check
	@echo ""
	@echo "✅ All local tests passed!"

# Run network tests, E2E, and benchmarks (~10-15 min)
# Use separately or combine with test-all: make test-all test-extra
test-extra:
	@echo "=== Running extra tests (network, E2E, benchmarks) ==="
	@echo ""
	@echo "Step 1/3: Running Go tests with network (no -short flag)..."
	CGO_ENABLED=1 go test -race -timeout=10m ./...
	@echo ""
	@echo "Step 2/3: Running E2E tests (Chromium only)..."
	$(MAKE) test-e2e-dev
	@echo ""
	@echo "Step 3/3: Running benchmarks..."
	$(MAKE) benchmark
	@echo ""
	@echo "✅ Extra tests passed!"

# ==============================================================================
# LINTING AND QUALITY
# ==============================================================================

# Run comprehensive linting (go vet + golangci-lint) - used by CI
lint:
	@echo "Running go vet..."
	go vet ./...
	@echo "Running golangci-lint..."
	golangci-lint run --timeout=5m

# Fix all auto-fixable issues (format + lint fixes)
fix:
	@echo "Running go fmt..."
	go fmt ./...
	@echo "Running goimports..."
	goimports -w . 2>/dev/null || echo "goimports not installed, skipping"
	@echo "Running golangci-lint with auto-fix..."
	golangci-lint run --fix --timeout=5m

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
	@echo 'if ! make lint; then' >> .git/hooks/pre-commit
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
	SHELLY_SECURITY_VALIDATION_TEST_MODE=true CGO_ENABLED=1 go test -v -short -bench=. ./...

# ==============================================================================
# DEPENDENCY MANAGEMENT
# ==============================================================================

# Install dependencies
deps:
	go mod download
	go mod verify

# Tidy dependencies (download + cleanup unused)
deps-tidy:
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
	# Root-level log files and server artifacts (including hidden)
	rm -f *.log .*.log server.pid
	# Stray database files in cmd directory (from test runs)
	rm -rf cmd/shelly-manager/data/
	# UI test artifacts (playwright reports, test results, coverage)
	rm -rf ui/playwright-report/ ui/test-results/ ui/coverage/
	rm -f ui/junit.xml ui/test-report.html ui/test-report.json ui/coverage.xml
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
	@if [ -z "$(SSID)" ] || [ -z "$(PASS)" ]; then \
		echo "Usage: make example-provisioner-provision SSID=YourNetwork PASS=YourPassword"; \
		exit 1; \
	fi
	./$(BUILD_DIR)/$(PROVISIONER_BINARY) provision "$(SSID)" "$(PASS)"
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

# Run smoke tests for quick feedback (essential tests only)
test-smoke:
	@./scripts/test-smoke.sh

# Validate E2E integration pipeline after configuration changes
validate-integration:
	@./scripts/validate-integration.sh
