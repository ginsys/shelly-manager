#!/bin/bash
# Optimized E2E Test Runner
# Implements Phase 1 critical optimizations for 50%+ performance improvement

set -e
echo "=== OPTIMIZED E2E Test Runner ==="

# Go runtime optimizations for tests
export GOGC=100              # Default GC target (don't over-optimize)
export GOMEMLIMIT=2GiB       # Memory limit for container environments
export GOMAXPROCS=2          # Match Playwright workers
export GODEBUG=gctrace=0     # Disable GC tracing for performance

# Kill any existing processes
pkill -f "shelly-manager server" || true
pkill -f "node.*vite.*preview" || true

# Build with test optimizations if needed
if [ ! -f "bin/shelly-manager" ] || [ internal/ -nt "bin/shelly-manager" ]; then
    echo "Building optimized backend..."
    CGO_ENABLED=1 go build \
        -ldflags="-s -w -X main.version=test" \
        -o bin/shelly-manager ./cmd/shelly-manager
fi

# Start backend with optimized settings
echo "Starting OPTIMIZED backend server..."
SHELLY_DATABASE_PROVIDER=sqlite \
SHELLY_DATABASE_PATH=":memory:" \
SHELLY_LOGGING_LEVEL=error \
SHELLY_DISCOVERY_ENABLED=false \
SHELLY_PROVISIONING_AUTO=false \
SHELLY_SECURITY_VALIDATION_TEST_MODE=true \
SHELLY_TEST_MODE_FAST_HEALTH=true \
GIN_MODE=release \
./bin/shelly-manager server > /dev/null 2>&1 &

BACKEND_PID=$!
echo "Backend started with PID: $BACKEND_PID"

# Optimized health check
echo "Waiting for backend (optimized)..."
for i in $(seq 1 15); do
    if timeout 3 curl -s -f http://localhost:8080/healthz >/dev/null 2>&1; then
        echo "Backend ready after ${i}s"
        break
    fi
    if [ $i -eq 15 ]; then
        echo "Backend failed to start after 15 seconds"
        kill $BACKEND_PID 2>/dev/null || true
        exit 1
    fi
    sleep 1
done

# Start frontend only if needed
if [ "${SKIP_FRONTEND:-}" != "true" ]; then
    cd ui

    # Build frontend if needed (production build is faster for tests)
    if [ "${USE_BUILD:-}" = "true" ]; then
        npm run build
        npx serve dist -l 5173 > /dev/null 2>&1 &
    else
        npm run preview -- --port 5173 --host 127.0.0.1 > /dev/null 2>&1 &
    fi

    FRONTEND_PID=$!
    cd ..

    # Quick frontend check
    echo "Waiting for frontend..."
    for i in $(seq 1 10); do
        if timeout 2 curl -s -f http://localhost:5173 >/dev/null 2>&1; then
            echo "Frontend ready after ${i}s"
            break
        fi
        if [ $i -eq 10 ]; then
            echo "Frontend failed to start"
            kill $BACKEND_PID $FRONTEND_PID 2>/dev/null || true
            exit 1
        fi
        sleep 1
    done
fi

# Cleanup function
cleanup() {
    echo "Cleaning up processes..."
    kill $BACKEND_PID $FRONTEND_PID 2>/dev/null || true
    wait 2>/dev/null || true
}
trap cleanup EXIT

# Run tests with optimized settings
cd ui
echo "Running optimized E2E tests..."

export PLAYWRIGHT_WORKERS=2
export PLAYWRIGHT_TIMEOUT=60000

# Choose test suite based on parameter
case "${1:-full}" in
    "dev")
        echo "Running development test suite (Chromium-only, 10-15 min target)..."
        npx playwright test --config=playwright-dev.config.ts --max-failures=10
        ;;
    "smoke")
        echo "Running smoke tests..."
        npx playwright test --config=playwright-smoke.config.ts
        ;;
    "critical")
        echo "Running critical tests..."
        npx playwright test tests/e2e/critical --workers=2 --timeout=60000
        ;;
    "quick")
        echo "Running quick test suite..."
        npx playwright test tests/e2e/smoke tests/e2e/critical --workers=2 --timeout=60000 --max-failures=5
        ;;
    *)
        echo "Running full test suite..."
        npx playwright test --workers=2 --timeout=60000 --max-failures=10
        ;;
esac

cd ..
echo "=== Test execution completed ==="
