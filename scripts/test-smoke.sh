#!/bin/bash

# Smoke Test Runner Script
# This script provides a fast way to run essential tests for quick feedback

set -euo pipefail

echo "=== Starting Smoke Tests with Fresh Database ==="

# Kill any existing backend servers (use specific path to avoid killing unrelated processes)
echo "Stopping any existing backend servers..."
pkill -f "bin/shelly-manager server" || true
sleep 1

# Check if required ports are available (after cleanup)
# Portable: works on Linux (ss), macOS (lsof), and BSD (netstat)
check_port() {
    local port=$1
    local service=$2
    local in_use=false

    if command -v ss &>/dev/null; then
        ss -tuln 2>/dev/null | grep -q ":${port} " && in_use=true
    elif command -v lsof &>/dev/null; then
        lsof -nP -iTCP:"$port" -sTCP:LISTEN &>/dev/null && in_use=true
    else
        netstat -an 2>/dev/null | grep -qE "[:.]${port} .*LISTEN" && in_use=true
    fi

    if $in_use; then
        echo "ERROR: Port $port ($service) is already in use"
        echo "Please stop the existing process or use a different port"
        # Show what's using the port if possible
        if command -v lsof &>/dev/null; then
            lsof -nP -iTCP:"$port" -sTCP:LISTEN 2>/dev/null || true
        elif command -v ss &>/dev/null; then
            ss -tulnp 2>/dev/null | grep ":${port} " || true
        fi
        exit 1
    fi
}

echo "Checking port availability..."
check_port 8081 "Backend"
check_port 5173 "Frontend"

# Delete existing test database
echo "Cleaning test database..."
rm -f /tmp/shelly_test_smoke.db

# Build the backend if needed
if [ ! -f "bin/shelly-manager" ]; then
    echo "Building backend..."
    CGO_ENABLED=1 go build -o bin/shelly-manager ./cmd/shelly-manager
fi

# Start backend with test configuration (full router, not test mode)
# Note: We do NOT use SHELLY_SECURITY_VALIDATION_TEST_MODE because smoke tests
# need the full router with all routes (export, import, status, etc.)
echo "Starting backend server for smoke tests..."
SHELLY_DATABASE_PROVIDER=sqlite \
SHELLY_DATABASE_PATH="/tmp/shelly_test_smoke.db" \
SHELLY_LOGGING_LEVEL=error \
SHELLY_DISCOVERY_ENABLED=false \
SHELLY_PROVISIONING_AUTO=false \
SHELLY_HTTP_PORT=8081 \
GIN_MODE=release \
./bin/shelly-manager server > backend.log 2>&1 &

BACKEND_PID=$!
echo "Backend started with PID: $BACKEND_PID"

# Function to cleanup on exit
cleanup() {
    echo "Cleaning up..."
    if [ -n "${FRONTEND_PID:-}" ] && kill -0 "$FRONTEND_PID" 2>/dev/null; then
        echo "Stopping frontend server (PID: $FRONTEND_PID)..."
        kill "$FRONTEND_PID" || true
    fi
    if [ -n "${BACKEND_PID:-}" ] && kill -0 "$BACKEND_PID" 2>/dev/null; then
        echo "Stopping backend server (PID: $BACKEND_PID)..."
        kill "$BACKEND_PID" || true
    fi
    # Wait for processes to exit before removing DB
    sleep 2
    rm -f /tmp/shelly_test_smoke.db
    echo "Cleanup complete."
}

# Set trap to cleanup on script exit
trap cleanup EXIT

# Wait for backend to be ready
echo "Waiting for backend to start..."
for i in $(seq 1 30); do
    if curl -sf http://localhost:8081/healthz >/dev/null 2>&1; then
        echo "Backend ready after $i seconds"
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "Backend failed to start after 30 seconds"
        echo "Backend logs (last 100 lines):"
        tail -n 100 backend.log || true
        exit 1
    fi
    sleep 1
done

# Build frontend
echo "Building frontend..."
cd ui
npm ci
npm run build
cd ..

# Start frontend server
echo "Starting frontend server..."
cd ui
npm run preview -- --port 5173 --strictPort > ../frontend.log 2>&1 &
FRONTEND_PID=$!
cd ..
echo "Frontend started with PID: $FRONTEND_PID"

# Wait for frontend to be ready
echo "Waiting for frontend to start..."
for i in $(seq 1 30); do
    if curl -sf http://localhost:5173 >/dev/null 2>&1; then
        echo "Frontend ready after $i seconds"
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "Frontend failed to start after 30 seconds"
        echo "Frontend logs (last 100 lines):"
        tail -n 100 frontend.log || true
        exit 1
    fi
    sleep 1
done

# Run Smoke Tests
echo "Running smoke tests..."
cd ui
if npm run test:e2e:smoke; then
    echo "Smoke tests passed successfully"
    exit_code=0
else
    echo "Smoke tests failed"
    exit_code=1
fi

cd ..
exit $exit_code
