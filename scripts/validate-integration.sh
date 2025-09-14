#!/bin/bash

# Integration Validation Script
# Comprehensive validation of E2E test pipeline after configuration changes

set -e

echo "=== E2E Integration Validation Suite ==="

# Function to log with timestamp
log() {
    echo "[$(date '+%H:%M:%S')] $1"
}

# Function to check process resource usage
check_resources() {
    local process_name="$1"
    local max_memory_mb="$2"
    
    pids=$(pgrep -f "$process_name" || echo "")
    if [ -n "$pids" ]; then
        for pid in $pids; do
            if ps -p $pid > /dev/null; then
                memory_kb=$(ps -o rss= -p $pid | tail -1 | tr -d ' ')
                memory_mb=$((memory_kb / 1024))
                log "Process $process_name (PID: $pid) using ${memory_mb}MB RAM"
                if [ $memory_mb -gt $max_memory_mb ]; then
                    log "WARNING: Memory usage exceeds threshold ($max_memory_mb MB)"
                fi
            fi
        done
    fi
}

# Function to validate database access
validate_database() {
    local db_path="$1"
    if [ -f "$db_path" ]; then
        log "Database found at $db_path"
        # Check if database is accessible (not locked)
        if timeout 5s sqlite3 "$db_path" "SELECT 1;" > /dev/null 2>&1; then
            log "Database is accessible"
        else
            log "ERROR: Database is locked or inaccessible"
            return 1
        fi
    else
        log "Database not found at $db_path (this may be expected)"
    fi
}

# Function to check port availability
check_port() {
    local port="$1"
    local service="$2"
    if netstat -tuln 2>/dev/null | grep ":$port " > /dev/null; then
        log "Port $port ($service) is in use"
    else
        log "Port $port ($service) is available"
    fi
}

# Phase 1: Pre-test System State Validation
log "Phase 1: Pre-test System State Validation"

log "Checking port availability..."
check_port 8080 "Backend"
check_port 5173 "Frontend"

log "Checking for existing processes..."
check_resources "shelly-manager" 500
check_resources "vite" 300

log "Validating database state..."
validate_database "/tmp/shelly_test.db"

# Phase 2: Configuration Validation
log "Phase 2: Configuration Validation"

# Verify Playwright configuration changes
if grep -q "fullyParallel: false" ui/playwright.config.ts; then
    log "✓ Playwright configuration: fullyParallel correctly set to false"
else
    log "✗ ERROR: fullyParallel setting not found or incorrect"
    exit 1
fi

if grep -q "workers: 2" ui/playwright.config.ts; then
    log "✓ Playwright configuration: workers correctly set to 2"
else
    log "✗ ERROR: workers setting not found or incorrect"
    exit 1
fi

# Phase 3: Integration Test Execution
log "Phase 3: Integration Test Execution"

# Start resource monitoring in background
(
    log "Starting resource monitoring..."
    while sleep 5; do
        check_resources "shelly-manager" 500
        check_resources "node" 200
        check_resources "chromium" 300
        check_resources "firefox" 350
    done
) > integration-monitoring.log 2>&1 &
MONITOR_PID=$!

# Cleanup function
cleanup() {
    log "Cleaning up monitoring process..."
    if [ -n "$MONITOR_PID" ] && kill -0 $MONITOR_PID 2>/dev/null; then
        kill $MONITOR_PID
    fi
}
trap cleanup EXIT

# Run smoke test first (quick validation)
log "Running smoke test for quick validation..."
start_time=$(date +%s)
if cd ui && npm run test:e2e:smoke; then
    end_time=$(date +%s)
    duration=$((end_time - start_time))
    log "✓ Smoke test completed in ${duration}s"
    cd ..
else
    log "✗ Smoke test failed"
    cd ..
    exit 1
fi

# Phase 4: Integration Metrics Collection
log "Phase 4: Integration Metrics Collection"

log "Collecting integration metrics..."

# Database metrics
if [ -f "/tmp/shelly_test.db" ]; then
    db_size=$(du -h /tmp/shelly_test.db | cut -f1)
    log "Database size: $db_size"
fi

# Memory usage summary
total_memory_mb=$(free -m | awk '/^Mem:/{print $3}')
available_memory_mb=$(free -m | awk '/^Mem:/{print $7}')
log "System memory: ${total_memory_mb}MB used, ${available_memory_mb}MB available"

# Process count
playwright_processes=$(pgrep -f "playwright" | wc -l)
browser_processes=$(pgrep -f "chromium\|firefox\|webkit" | wc -l)
log "Active Playwright processes: $playwright_processes"
log "Active browser processes: $browser_processes"

# Phase 5: Integration Health Check
log "Phase 5: Integration Health Check"

# Check service endpoints
if curl -f -m 5 http://localhost:8080/healthz > /dev/null 2>&1; then
    log "✓ Backend health check passed"
else
    log "? Backend not running (may be expected after test)"
fi

if curl -f -m 5 http://localhost:5173 > /dev/null 2>&1; then
    log "✓ Frontend health check passed"
else
    log "? Frontend not running (may be expected after test)"
fi

log "=== Integration Validation Complete ==="
log "Summary:"
log "- Configuration validation: PASSED"
log "- Smoke test execution: PASSED"
log "- Resource monitoring: COMPLETED"
log "- Integration metrics: COLLECTED"

echo ""
echo "Integration validation successful! E2E testing pipeline is ready."
echo "Monitoring logs: integration-monitoring.log"