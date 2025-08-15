# Testing Strategy

## Network Test Strategy

The project uses an improved network testing strategy that balances test coverage with performance and reliability.

### Test Categories

1. **Unit Tests**: Fast, isolated tests that don't require network access
2. **Integration Tests**: Tests that may require network access for realistic scenarios
3. **Network Tests**: Tests that specifically test network functionality

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

### Implementation Status

- ✅ Helper functions created in `internal/testutil/network_test_helper.go`
- ✅ Discovery tests updated to use new strategy
- 🔄 Service tests can be updated to use the same pattern
- 📋 Documentation created

### Migration Guide

To migrate existing network tests:

1. Replace manual `testing.Short()` checks with `testutil.SkipNetworkTestIfNeeded()`
2. Use `testutil.CreateNetworkTestContext()` for timeouts
3. Replace hardcoded test addresses with `testutil.TestNetworkAddress()`
4. Update test documentation to reflect new capabilities