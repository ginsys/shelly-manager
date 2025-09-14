#!/bin/bash

# Smoke Test Runner Script
# This script provides a fast way to run essential tests for quick feedback

set -e

echo "=== Starting Smoke Tests with Fresh Database ==="

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

# Start backend with OPTIMIZED test configuration
echo "Starting OPTIMIZED backend server for smoke tests..."
SHELLY_DATABASE_PROVIDER=sqlite \
SHELLY_DATABASE_PATH=":memory:" \
SHELLY_LOGGING_LEVEL=error \
SHELLY_DISCOVERY_ENABLED=false \
SHELLY_PROVISIONING_AUTO=false \
SHELLY_SECURITY_VALIDATION_TEST_MODE=true \
GIN_MODE=release \
./bin/shelly-manager server > /dev/null 2>&1 &

BACKEND_PID=$!
echo "Backend started with PID: $BACKEND_PID"

# Function to cleanup on exit
cleanup() {
    echo "Cleaning up..."
    if [ ! -z "$FRONTEND_PID" ] && kill -0 $FRONTEND_PID 2>/dev/null; then
        echo "Stopping frontend server (PID: $FRONTEND_PID)..."
        kill $FRONTEND_PID
        sleep 1
    fi
    if [ ! -z "$BACKEND_PID" ] && kill -0 $BACKEND_PID 2>/dev/null; then
        echo "Stopping backend server (PID: $BACKEND_PID)..."
        kill $BACKEND_PID
        sleep 1
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

# Build frontend
echo "Building frontend..."
cd ui
npm run build
cd ..

# Start frontend server
echo "Starting frontend server..."
cd ui
npm run preview -- --port 5173 > ../frontend.log 2>&1 &
FRONTEND_PID=$!
cd ..
echo "Frontend started with PID: $FRONTEND_PID"

# Wait for frontend to be ready
echo "Waiting for frontend to start..."
for i in $(seq 1 30); do
    if curl -f http://localhost:5173 >/dev/null 2>&1; then
        echo "Frontend ready after $i seconds"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "Frontend failed to start after 30 seconds"
        echo "Frontend logs:"
        cat frontend.log || true
        exit 1
    fi
    sleep 1
done

# Run Smoke Tests
echo "Running smoke tests..."
cd ui
if npm run test:e2e:smoke; then
    echo "✅ Smoke tests passed successfully"
    exit_code=0
else
    echo "❌ Smoke tests failed"
    exit_code=1
fi

cd ..
exit $exit_code