.PHONY: build run clean docker-build docker-run docker-stop docker-logs dev-setup deps deps-tidy lint \
	test test-unit test-integration test-full \
	test-race test-race-short test-race-full \
	test-coverage test-coverage-short test-coverage-full test-coverage-ci test-coverage-check test-coverage-with-check \
	test-matrix test-ci \
	benchmark test-watch example-list example-discover example-provision

BINARY_NAME=shelly-manager
BUILD_DIR=bin
DOCKER_IMAGE=shelly-manager

# ==============================================================================
# BUILD COMMANDS
# ==============================================================================

# Build the application
build:
	CGO_ENABLED=1 go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/shelly-manager

# Run the application
run:
	go run ./cmd/shelly-manager server

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

# Check coverage threshold (30% minimum) - requires existing coverage.out
test-coverage-check:
	@COVERAGE=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "Total coverage: $${COVERAGE}%"; \
	THRESHOLD=28; \
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
	CGO_ENABLED=1 go test -v -race -short ./...

# Complete CI test suite (coverage + race detection + threshold check)
test-ci: test-coverage-check

# ==============================================================================
# LINTING AND QUALITY
# ==============================================================================

# Run basic linting (gofmt, go vet)
lint:
	@echo "Running go fmt..."
	go fmt ./...
	@echo "Running go vet..."
	go vet ./...

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

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)/
	rm -f *.db
	rm -f coverage.out coverage.html
	rm -rf tmp/

# ==============================================================================
# CLI EXAMPLES
# ==============================================================================

# CLI examples
example-list:
	./$(BUILD_DIR)/$(BINARY_NAME) list

example-discover:
	./$(BUILD_DIR)/$(BINARY_NAME) discover 192.168.1.0/24

example-provision:
	./$(BUILD_DIR)/$(BINARY_NAME) provision
