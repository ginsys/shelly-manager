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

# Prefer backend-served UI for Firefox runs to avoid SPA deep-linking quirks
if [ "${PW_PROJECT:-}" = "firefox" ]; then
  export SKIP_FRONTEND=true
  export PW_BASEURL=http://localhost:8080
  echo "Firefox project detected: using backend-hosted UI at $PW_BASEURL (SKIP_FRONTEND=${SKIP_FRONTEND})"
fi

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

# Start frontend only if needed (optionally serve UI via backend)
if [ "${SKIP_FRONTEND:-}" != "true" ]; then
    cd ui

    # Ensure dependencies are installed (non-interactive)
    if [ ! -d node_modules ]; then
        npm ci
    fi

    # Always build once to ensure vite preview has dist/ available
    VITE_API_BASE=http://localhost:8080/api/v1 VITE_WS_URL=ws://localhost:8080 npm run build
    # Provide runtime app-config.js for UI to pick up base URLs
    echo "window.__API_BASE__='http://localhost:8080/api/v1'; window.__ADMIN_KEY__='dev-admin-key';" > dist/app-config.js
    # Serve the built assets using Vite preview (fast, no extra deps)
    npm run preview -- --port 5173 --host localhost > /dev/null 2>&1 &

    FRONTEND_PID=$!
    cd ..

    # Quick frontend check
    echo "Waiting for frontend..."
    for i in $(seq 1 20); do
        if timeout 2 curl -s -f http://localhost:5173 >/dev/null 2>&1; then
            echo "Frontend ready after ${i}s"
            break
        fi
        if [ $i -eq 20 ]; then
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

# Optional Playwright args (e.g., PW_PROJECT=firefox)
PW_ARGS=""
if [ -n "${PW_PROJECT:-}" ]; then
  PW_ARGS="--project=${PW_PROJECT}"
fi

# Ensure Playwright browsers are installed (non-interactive)
echo "Ensuring Playwright browsers are installed..."
npm run test:install --silent || npx playwright install --with-deps

# Choose test suite based on parameter
case "${1:-full}" in
    "dev")
        echo "Running development test suite (Chromium-only, 10-15 min target)..."
        npx playwright test --config=playwright-dev.config.ts --max-failures=10 ${PW_ARGS}
        ;;
    "smoke")
        echo "Running smoke tests..."
        npx playwright test --config=playwright-smoke.config.ts ${PW_ARGS}
        ;;
    "critical")
        echo "Running critical tests..."
        npx playwright test tests/e2e/critical --workers=2 --timeout=60000 ${PW_ARGS}
        ;;
    "quick")
        echo "Running quick test suite..."
        npx playwright test tests/e2e/smoke tests/e2e/critical --workers=2 --timeout=60000 --max-failures=5 ${PW_ARGS}
        ;;
    *)
        echo "Running full test suite..."
        npx playwright test --workers=2 --timeout=60000 --max-failures=10 ${PW_ARGS}
        ;;
esac

cd ..
echo "=== Test execution completed ==="
