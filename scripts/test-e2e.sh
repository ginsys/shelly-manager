#!/bin/bash

# E2E Test Runner Script
# This script provides a reliable way to run E2E tests with fresh database

set -e

echo "=== Starting E2E Tests with Fresh Database ==="

# Kill any existing backend servers
echo "Stopping any existing backend servers..."
pkill -f "shelly-manager server" || true
sleep 1

# Delete existing test database
echo "Cleaning test database..."
rm -f /tmp/shelly_test.db

# Build the backend if needed
if [ ! -f "bin/shelly-manager" ]; then
    echo "Building backend..."
    CGO_ENABLED=1 go build -o bin/shelly-manager ./cmd/shelly-manager
fi

# Start backend with test configuration
echo "Starting backend server..."
SHELLY_DATABASE_PROVIDER=sqlite \
SHELLY_DATABASE_PATH=/tmp/shelly_test.db \
SHELLY_LOGGING_LEVEL=warn \
SHELLY_DISCOVERY_ENABLED=false \
SHELLY_PROVISIONING_AUTO=false \
SHELLY_SECURITY_VALIDATION_TEST_MODE=true \
GIN_MODE=release \
./bin/shelly-manager server > backend.log 2>&1 &

BACKEND_PID=$!
echo "Backend started with PID: $BACKEND_PID"

# Function to cleanup on exit
cleanup() {
    echo "Cleaning up..."
    if kill -0 $BACKEND_PID 2>/dev/null; then
        echo "Stopping backend server (PID: $BACKEND_PID)..."
        kill $BACKEND_PID
        sleep 2
        # Force kill if still running
        if kill -0 $BACKEND_PID 2>/dev/null; then
            echo "Force killing backend..."
            kill -9 $BACKEND_PID
        fi
    fi
    echo "Cleanup complete."
}

# Set trap to cleanup on script exit
trap cleanup EXIT

# Wait for backend to be ready
echo "Waiting for backend to start..."
for i in $(seq 1 30); do
    if curl -f http://localhost:8080/healthz >/dev/null 2>&1; then
        echo "Backend ready after $i seconds"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "Backend failed to start after 30 seconds"
        echo "Backend logs:"
        cat backend.log || true
        exit 1
    fi
    sleep 1
done

# Run E2E tests
echo "Running E2E tests..."
cd ui
if npm run test:e2e; then
    echo "E2E tests completed successfully"
    exit_code=0
else
    echo "E2E tests failed"
    exit_code=1
fi

cd ..
exit $exit_code