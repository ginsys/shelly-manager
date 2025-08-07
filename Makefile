.PHONY: build run test clean docker-build docker-run dev-setup deps

BINARY_NAME=shelly-manager
BUILD_DIR=bin
DOCKER_IMAGE=shelly-manager

# Build the application
build:
	CGO_ENABLED=1 go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/shelly-manager

# Run the application
run:
	go run ./cmd/shelly-manager server

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)/
	rm -f *.db

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
