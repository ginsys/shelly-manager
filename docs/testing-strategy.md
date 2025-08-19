# Testing Strategy

## Network Test Strategy

The project uses a comprehensive testing strategy that **does NOT connect to real Shelly devices** and balances test coverage with performance and reliability.

### Test Categories

1. **Unit Tests**: Fast, isolated tests that don't require network access
2. **Integration Tests**: Tests with mock servers and databases, no external network access
3. **Network Tests**: Tests using non-routable TEST-NET addresses only

### Network Test Controls

#### Environment Variables

- `SHELLY_FORCE_NETWORK_TESTS=1` - Forces network tests to run even in short mode
- `SHELLY_CI_NETWORK_TESTS=1` - Enables network tests in CI environments
- `SHELLY_NETWORK_TEST_TIMEOUT=5s` - Override default network test timeout

#### Test Modes

- `go test -short` - Skips network tests for fast feedback
- `go test` - Runs all tests including network tests
- `make test` - Runs tests in short mode (fast)
- `make test-full` - Runs all tests including network tests

### Best Practices

#### For Test Authors

1. **Use Helper Functions**: Use `testutil.SkipNetworkTestIfNeeded()` instead of manual skipping
2. **Safe Test Addresses**: Use `testutil.TestNetworkAddress()` for consistent test addresses
3. **Appropriate Timeouts**: Use `testutil.CreateNetworkTestContext()` for context with proper timeouts
4. **Non-routable Addresses**: Always use TEST-NET addresses (192.0.2.x) for negative tests

#### Example Usage

```go
func TestNetworkOperation(t *testing.T) {
    // Use improved network test strategy
    config := testutil.DefaultNetworkConfig()
    testutil.SkipNetworkTestIfNeeded(t, config)

    // Create context with appropriate timeout
    ctx, cancel := testutil.CreateNetworkTestContext(config)
    defer cancel()

    // Use safe test address
    result, err := scanner.ScanHost(ctx, testutil.TestNetworkAddress())
    // ... test logic
}
```

### Benefits

1. **Performance**: Fast feedback in development with `-short` flag
2. **Reliability**: Consistent timeout handling and safe test addresses
3. **Flexibility**: Environment variable controls for different scenarios
4. **CI-Friendly**: Explicit controls for CI environments
5. **Documentation**: Clear indication of test intent and requirements

## Safety Guarantees

### No Real Device Connections
- **Mock Servers**: All Shelly device interactions use `httptest.Server` mocks
- **TEST-NET Addresses**: Network tests use non-routable RFC 5737 addresses:
  - `192.0.2.x` (TEST-NET-1) 
  - `203.0.113.x` (TEST-NET-3)
- **Localhost Only**: Real HTTP connections only to localhost mock servers
- **Database Isolation**: Tests use in-memory SQLite or temporary files

### Performance Optimizations (2024)
- **Timeout Reductions**: Discovery tests reduced from 5s to 100ms
- **Sleep Elimination**: Removed unnecessary `time.Sleep()` calls:
  - Logging tests: 200ms → 0ms (synchronous writes)
  - Metrics tests: 1.1s → 0ms (manual counter testing)  
  - Server tests: 2s → 50-500ms (HTTP polling)
- **Parallel Execution**: Tests run concurrently where safe
- **Overall Improvement**: ~70% faster test execution

### Implementation Status

- ✅ Helper functions created in `internal/testutil/network_test_helper.go`
- ✅ Discovery tests updated to use new strategy with optimized timeouts
- ✅ Service tests use TEST-NET addresses and short timeouts
- ✅ Comprehensive mock servers for all Shelly device interactions
- ✅ Test timing optimizations reduce execution time by ~70%

### Migration Guide

To migrate existing network tests:

1. Replace manual `testing.Short()` checks with `testutil.SkipNetworkTestIfNeeded()`
2. Use `testutil.CreateNetworkTestContext()` for timeouts
3. Replace hardcoded test addresses with `testutil.TestNetworkAddress()`
4. Update test documentation to reflect new capabilities

## CI/CD Configuration

### Recommended Settings

**GitHub Actions / CI Environments:**
```yaml
# Fast default tests (recommended for CI)
- name: Run Tests
  run: make test  # Uses -short flag

# Full tests including network tests (optional)  
- name: Run Full Test Suite
  run: make test-full
  env:
    SHELLY_CI_NETWORK_TESTS: "1"
```

**Local Development:**
```bash
# Fast feedback loop
make test                    # Short mode, ~30s

# Coverage with fast tests  
make test-coverage          # Short mode with coverage

# Full test suite (when needed)
SHELLY_FORCE_NETWORK_TESTS=1 make test-full  # ~45s
```

### Environment Variables

| Variable | Effect | Default |
|----------|---------|---------|
| `CI=true` | Enables CI mode, skips network tests unless forced | Auto-detected |
| `SHELLY_FORCE_NETWORK_TESTS=1` | Forces network tests to run even in short mode | Disabled |
| `SHELLY_CI_NETWORK_TESTS=1` | Enables network tests in CI environments | Disabled |
| `SHELLY_NETWORK_TEST_TIMEOUT=5s` | Override default network test timeout | 5s |

### Test Execution Times

| Command | Mode | Typical Duration | Coverage |
|---------|------|------------------|----------|
| `make test` | Short | ~30s | Core functionality |
| `make test-full` | Full | ~45s | All including network |
| `make test-race` | Race detection | ~40s | Concurrency issues |
| `make test-coverage` | Coverage | ~35s | With coverage report |

### Performance Monitoring

Tests include built-in performance monitoring:
- Discovery timeouts reduced from 5s → 100ms
- Sleep-based waits eliminated (1.3s → 0s total)
- Server startup polling (2s → 50-500ms average)
- Overall test suite ~70% faster than previous versions