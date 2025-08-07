.PHONY: build run test test-unit test-integration test-coverage test-race clean docker-build docker-run dev-setup deps

BINARY_NAME=shelly-manager
BUILD_DIR=bin
DOCKER_IMAGE=shelly-manager

# Build the application
build:
	CGO_ENABLED=1 go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/shelly-manager

# Run the application
run:
	go run ./cmd/shelly-manager server

# Run all tests
test:
	CGO_ENABLED=1 go test -v ./...

# Run unit tests only (exclude integration tests)
test-unit:
	CGO_ENABLED=1 go test -v -short ./internal/...

# Run integration tests
test-integration:
	CGO_ENABLED=1 go test -v ./cmd/...

# Run tests with coverage report
test-coverage:
	CGO_ENABLED=1 go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with race detection
test-race:
	CGO_ENABLED=1 go test -v -race ./...

# Run benchmarks
benchmark:
	CGO_ENABLED=1 go test -v -bench=. ./...

# Watch mode for tests (requires entr: brew install entr)
test-watch:
	find . -name "*.go" | entr -c make test-unit

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)/
	rm -f *.db
	rm -f coverage.out coverage.html
	rm -rf tmp/

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

# Development setup
dev-setup:
	go mod tidy
	mkdir -p $(BUILD_DIR) data

# Install dependencies
deps:
	go mod download
	go mod tidy

# CLI examples
example-list:
	./$(BUILD_DIR)/$(BINARY_NAME) list

example-discover:
	./$(BUILD_DIR)/$(BINARY_NAME) discover 192.168.1.0/24

example-provision:
	./$(BUILD_DIR)/$(BINARY_NAME) provision
