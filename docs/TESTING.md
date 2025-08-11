# Testing Framework Documentation

This document describes the comprehensive testing framework for the Shelly Manager project.

## Test Structure

```
├── internal/
│   ├── api/handlers_test.go           # API endpoint tests
│   ├── database/database_test.go      # Database operations tests
│   ├── discovery/discovery_test.go    # Device discovery tests
│   └── testutil/testutil.go          # Test utilities and helpers
└── cmd/shelly-manager/main_test.go    # Integration/CLI tests
```

## Running Tests

### Quick Commands

```bash
# Run all tests
make test

# Run only unit tests (fast)
make test-unit

# Run integration tests
make test-integration  

# Generate coverage report
make test-coverage

# Run with race detection
make test-race

# Run benchmarks
make benchmark
```

### Coverage Results

Current test coverage by package:
- **Database**: 94.4% - Excellent coverage of all CRUD operations
- **API**: 81.4% - Good coverage of HTTP handlers and routing
- **Discovery**: 53.0% - Core device discovery logic covered
- **Overall**: ~65% - Good foundation with room for improvement

## Test Categories

### Unit Tests (`internal/*/`)

**Database Tests** (`internal/database/database_test.go`):
- ✅ Database connection and migration
- ✅ Device CRUD operations 
- ✅ Constraint validation (unique IP addresses)
- ✅ Timestamp handling (CreatedAt, UpdatedAt)
- ✅ Error handling for invalid operations
- ✅ Multiple device management

**API Tests** (`internal/api/handlers_test.go`):
- ✅ All HTTP endpoints (GET, POST, PUT, DELETE)
- ✅ JSON serialization/deserialization
- ✅ Error responses (400, 404, 500)
- ✅ Request routing and parameter handling
- ✅ Integration with database layer

**Discovery Tests** (`internal/discovery/discovery_test.go`):
- ✅ HTTP scanner configuration and timeouts
- ✅ Gen1 and Gen2 device detection
- ✅ Device type classification (30+ device types)
- ✅ Network scanning with CIDR ranges
- ✅ Mock Shelly server responses
- ✅ Error handling and edge cases

### Integration Tests (`cmd/shelly-manager/main_test.go`)

**CLI Command Tests**:
- ✅ Help command output validation
- ✅ List command with empty/populated database
- ✅ Discovery command with network scanning
- ✅ Provision command (mock implementation)
- ✅ Server startup and configuration
- ✅ Config file handling (valid/invalid/missing)
- ✅ Error scenarios and timeouts

## Test Utilities

### Mock Servers (`internal/testutil/testutil.go`)

**MockShellyServer()** - Simulates Gen1 devices:
```json
{
  "type": "SHPLG-S",
  "mac": "A4CF12345678", 
  "auth": true,
  "fw": "20231219-134356"
}
```

**MockShellyGen2Server()** - Simulates Gen2+ devices:
```json
{
  "id": "shellyplusht-08b61fcb7f3c",
  "mac": "08B61FCB7F3C",
  "model": "SNSN-0013A",
  "gen": 2
}
```

### Helper Functions

- `TestDatabase(t)` - Creates in-memory SQLite for testing
- `TestDevice()` - Generates realistic test device data
- `TestConfig()` - Creates test configuration
- `TempDir(t)` - Creates temporary directories with cleanup
- `AssertNoError(t, err)` - Common assertion helpers

## Test Data

### Device Types Tested

The discovery tests validate correct classification of 30+ Shelly device types:

**Gen1 Devices**: SHPLG-S, SHSW-1, SHSW-PM, SHSW-25, SHIX3-1, SHDM-1, etc.
**Gen2 Devices**: SNSN-*, SPSW-*, SPSH-* pattern matching
**Pattern Matching**: plug, dimmer, sensor, controller, etc.

### Test Scenarios

- **Happy Path**: Valid devices, successful operations
- **Error Cases**: Invalid data, network timeouts, missing resources
- **Edge Cases**: Empty databases, malformed config files
- **Concurrency**: Race condition detection with `-race` flag
- **Performance**: Benchmarking with network operations

## Continuous Integration

### Pre-commit Checks
```bash
# Recommended before committing
make test-unit      # Fast feedback (11 seconds)
make test-coverage  # Full coverage report
```

### CI Pipeline Suggestions
```yaml
# Example GitHub Actions
- name: Run Tests
  run: |
    make test-unit
    make test-race
    make test-coverage
```

## Adding New Tests

### For New Features

1. **Unit Tests**: Add to relevant `internal/*/test.go` file
2. **API Tests**: Add endpoint tests to `api/handlers_test.go`
3. **Integration**: Add CLI tests to `cmd/*/main_test.go`

### Test Naming Conventions

```go
func TestFeatureName(t *testing.T)           // Basic functionality
func TestFeatureName_ErrorCase(t *testing.T) // Error scenarios  
func TestFeatureName_EdgeCase(t *testing.T)  // Edge cases
```

### Mock Usage

```go
// Use mock servers for external dependencies
server := testutil.MockShellyServer()
defer server.Close()

// Use test database for persistence
db := testutil.TestDatabase(t)
```

## Performance Testing

### Benchmarks

Current benchmarks test CLI command performance:

```bash
make benchmark
# BenchmarkListCommand-8    100    10.2ms/op
```

### Profiling

```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof

# Memory profiling  
go test -memprofile=mem.prof -bench=.
go tool pprof mem.prof
```

## Future Improvements

### Areas for Enhanced Coverage

1. **Service Layer**: Add tests for `internal/service/` (0% coverage)
2. **Config Package**: Add tests for `internal/config/` (0% coverage)
3. **mDNS Discovery**: Real mDNS integration tests
4. **Network Scenarios**: Test with actual Shelly devices
5. **Error Propagation**: More comprehensive error path testing

### Test Infrastructure

1. **Test Containers**: Docker containers for integration testing
2. **Mock Device Farm**: Simulate multiple device types
3. **Performance Regression**: Automated performance monitoring
4. **Chaos Testing**: Network failure simulation

## Coverage Goals

- **Target**: 80%+ overall coverage
- **Critical Paths**: 90%+ for database and API packages
- **Integration**: All CLI commands and major workflows
- **Performance**: Benchmark coverage for critical operations

The testing framework provides a solid foundation for reliable development and ensures the Shelly Manager works correctly across all supported device types and usage scenarios.