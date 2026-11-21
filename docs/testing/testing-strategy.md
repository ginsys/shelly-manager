# Testing Strategy

## Network Test Strategy

The project uses a comprehensive testing strategy that **does NOT connect to real Shelly devices** and balances test coverage with performance and reliability.

### Test Categories

1. **Unit Tests**: Fast, isolated tests that don't require network access
2. **Integration Tests**: Tests with mock servers and databases, no external network access
3. **Network Tests**: Tests that dial a loopback address only (a closed port, a loopback range or a local test server)

### Network Test Controls

#### Environment Variables

- `SHELLY_FORCE_NETWORK_TESTS=1` - Forces network tests to run even in short mode
- `SHELLY_CI_NETWORK_TESTS=1` - Enables network tests in CI environments
- `SHELLY_NETWORK_TEST_TIMEOUT=5s` - Override default network test timeout

#### Test Modes

- `go test -short` - Skips network tests for fast feedback
- `go test` - Runs all tests including network tests
- `make test` - Runs tests in short mode (fast)
- `make test-extra` - Runs all tests including network tests

### Best Practices

#### For Test Authors

1. **Use Helper Functions**: Use `testutil.SkipNetworkTestIfNeeded()` instead of manual skipping
2. **Safe Test Addresses**: Use `testutil.TestNetworkAddress()` (a closed loopback port) for an address that no device answers
3. **Appropriate Timeouts**: Use `testutil.CreateNetworkTestContext()` for context with proper timeouts
4. **Stay on the Host**: A test never dials an address off this host. A TEST-NET address (192.0.2.x, RFC 5737) is not safe: the host still sends the packet to its default gateway. Use `127.0.0.1:1` (refused at once) or an `httptest` server that holds the request. `testutil.AssertLoopbackAddress()` checks an address

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
- **Loopback Addresses**: Negative tests dial a closed loopback port (`127.0.0.1:1`) or a loopback
  range (`127.0.0.0/30`), which never leave the host
- **Localhost Only**: Real HTTP connections only to localhost mock servers
- **No WiFi tool**: The `scan-ap` CLI test runs with a PATH that holds no `nmcli`, so it never
  touches a real interface
- **Database Isolation**: Tests use in-memory SQLite or temporary files

### The dial guard

Every test package has a `TestMain` that calls `dialguard.Main`
(`internal/testutil/dialguard`). The guard refuses any dial that is not a loopback address,
`localhost` or a unix socket, and it refuses every DNS query. The refusal names the address. A test
that dials off the host fails, and the package fails even if every test passed. A meta-test
(`TestEveryTestPackageCallsMain`) fails for a test package without the call, whether the `TestMain`
sits in the internal or the external test package.

- **What it covers:** `http.DefaultTransport` (so `http.DefaultClient` and every client that does
  not set its own transport), `net.DefaultResolver`, and the three transports the code builds
  itself (OPNsense, Shelly gen1 and gen2). Those three dial through `internal/netguard`, which
  production leaves at the plain dialer.
- **A test database host** must be a literal loopback address or `localhost`. A host name is a DNS
  query, and the guard refuses it.
- **What it does not cover:** a raw `net.Dial`, `smtp.SendMail`, multicast (mDNS), and a database
  driver that dials by itself with its own resolver. Use a loopback address in those tests.
- **Proof that nothing leaves the host:** run the suite in an empty network namespace with only
  `lo` up: `unshare -rn sh -c 'ip link set lo up && go test -short ./...'`. A dial to any other
  address then fails at once with `ENETUNREACH`.

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
- ✅ Service tests use loopback addresses and short timeouts
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
  run: CGO_ENABLED=1 go test -race -timeout=10m ./...
  env:
    SHELLY_CI_NETWORK_TESTS: "1"
```

**Local Development:**
```bash
# Fast feedback loop
make test                    # Short mode, ~30s

# Coverage with fast tests  
make test-coverage          # Short mode with coverage (see coverage.html)

# Full test suite (when needed)
SHELLY_FORCE_NETWORK_TESTS=1 make test-extra  # ~10-15 min
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
| `make test-extra` | Full | ~10-15 min | All including network |
| `make test-race` | Race detection | ~40s | Concurrency issues |
| `make test-coverage` | Coverage | ~35s | With coverage report |

### Performance Monitoring

Tests include built-in performance monitoring:
- Discovery timeouts reduced from 5s → 100ms
- Sleep-based waits eliminated (1.3s → 0s total)
- Server startup polling (2s → 50-500ms average)
- Overall test suite ~70% faster than previous versions
