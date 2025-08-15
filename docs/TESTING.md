# Testing Guide

This document describes the comprehensive testing framework for the Shelly Manager project, including all available test commands and when to use them.

## Test Structure

```
├── internal/
│   ├── api/handlers_test.go           # API endpoint tests
│   ├── database/database_test.go      # Database operations tests
│   ├── discovery/discovery_test.go    # Device discovery tests
│   └── testutil/testutil.go          # Test utilities and helpers
└── cmd/shelly-manager/main_test.go    # Integration/CLI tests
```

## Test Command Groups

All test commands are organized into logical groups and defined in the `Makefile`. The GitHub Actions workflows use these make commands for consistent testing across environments.

### 1. Basic Test Commands

#### `make test`
**Purpose**: Run basic tests in fast mode, skipping network-dependent tests  
**Command**: `CGO_ENABLED=1 go test -v -short ./...`  
**When to use**: 
- During development for quick feedback
- Default test command for local development
- When you want to run most tests but skip slow network tests

#### `make test-unit` 
**Purpose**: Run only unit tests (internal packages)  
**Command**: `CGO_ENABLED=1 go test -v -short ./internal/...`  
**When to use**:
- Testing business logic in isolation
- Fastest test execution (11 seconds)
- When working on internal package changes

#### `make test-integration`
**Purpose**: Run integration tests (cmd packages)  
**Command**: `CGO_ENABLED=1 go test -v -short ./cmd/...`  
**When to use**:
- Testing CLI commands and their integration
- After changes to command-line interfaces
- Verifying end-to-end functionality

#### `make test-full`
**Purpose**: Run complete test suite including network tests  
**Command**: `CGO_ENABLED=1 go test -v -timeout=5m ./...`  
**When to use**:
- Before committing major changes
- When network-dependent functionality changes
- Full validation before releases
- **Warning**: Slower execution due to network tests, includes 5-minute timeout

### 2. Race Detection Tests

#### `make test-race` (default)
**Purpose**: Run tests with race detection in fast mode  
**Command**: `CGO_ENABLED=1 go test -v -short -race ./...`  
**When to use**:
- Detecting concurrency issues during development
- Default race detection command
- Before working with goroutines or shared data

#### `make test-race-short`
**Purpose**: Explicit short mode race detection  
**Command**: `CGO_ENABLED=1 go test -v -short -race ./...`  
**When to use**:
- Same as `make test-race`
- More explicit about running in short mode

#### `make test-race-full`
**Purpose**: Full race detection including network tests  
**Command**: `CGO_ENABLED=1 go test -v -race -timeout=10m ./...`  
**When to use**:
- Comprehensive race detection
- Before releases
- When debugging race conditions in network code
- **Warning**: Slowest test execution, includes 10-minute timeout

### 3. Coverage Tests

#### `make test-coverage`
**Purpose**: Generate test coverage report in fast mode  
**Command**: `CGO_ENABLED=1 go test -v -short -coverprofile=coverage.out ./...`  
**When to use**:
- Regular coverage checking during development
- Quick coverage feedback
- Generates `coverage.html` report

#### `make test-coverage-short`
**Purpose**: Explicit short mode coverage testing  
**Command**: Same as `test-coverage`  
**When to use**:
- More explicit about running in short mode
- Same use cases as `test-coverage`

#### `make test-coverage-full`
**Purpose**: Complete coverage including network tests  
**Command**: `CGO_ENABLED=1 go test -v -timeout=5m -coverprofile=coverage.out ./...`  
**When to use**:
- Comprehensive coverage analysis
- Before releases
- When you need complete coverage metrics
- **Warning**: Includes 5-minute timeout for network tests

#### `make test-coverage-ci`
**Purpose**: Coverage testing optimized for CI/CD  
**Command**: `CGO_ENABLED=1 go test -v -race -short -coverprofile=coverage.out -covermode=atomic ./...`  
**When to use**:
- CI/CD pipelines (used by GitHub Actions)
- Combines race detection with coverage
- Atomic coverage mode for better accuracy

#### `make test-coverage-check`
**Purpose**: Check if coverage meets minimum threshold (30%)  
**When to use**:
- Quality gates in CI/CD
- Ensuring minimum test coverage
- **Requires**: Existing `coverage.out` file

#### `make test-coverage-with-check`
**Purpose**: Generate coverage and check threshold in one step  
**When to use**:
- Complete coverage validation
- CI/CD quality gates
- Local validation before commits

### 4. CI/Matrix Tests

#### `make test-matrix`
**Purpose**: Run tests suitable for matrix testing across different environments  
**Command**: `CGO_ENABLED=1 go test -v -race -short ./...`  
**When to use**:
- GitHub Actions matrix testing (used automatically)
- Testing across different Go versions and OS
- Standardized test execution for CI

#### `make test-ci`
**Purpose**: Complete CI test suite with all validations  
**Dependencies**: `test-coverage-with-check`  
**When to use**:
- Main CI/CD pipeline
- Comprehensive validation
- Quality gate enforcement

### 5. Performance and Development Tests

#### `make benchmark`
**Purpose**: Run benchmark tests  
**Command**: `CGO_ENABLED=1 go test -v -short -bench=. ./...`  
**When to use**:
- Performance regression testing
- Optimizing critical code paths
- Measuring performance improvements

#### `make test-watch`
**Purpose**: Watch for file changes and re-run unit tests  
**Command**: `find . -name "*.go" | entr -c make test-unit`  
**When to use**:
- Active development with continuous feedback
- TDD (Test-Driven Development)
- **Requires**: `entr` tool installed

### 6. Quality and Dependencies

#### `make lint`
**Purpose**: Run code linting with golangci-lint  
**Command**: `golangci-lint run --timeout=5m`  
**When to use**:
- Code quality checking
- Before commits
- Catching style and potential issues

#### `make deps`
**Purpose**: Download and verify dependencies  
**Command**: `go mod download && go mod verify`  
**When to use**:
- Initial project setup
- CI/CD dependency installation
- Verifying dependency integrity

#### `make deps-tidy`
**Purpose**: Download dependencies and clean up go.mod  
**Command**: `go mod download && go mod tidy`  
**When to use**:
- After adding/removing dependencies
- Cleaning up unused dependencies
- Preparing for commits

## GitHub Actions Integration

The GitHub Actions workflows (`test.yml`) use the following make commands:

### Main test job:
1. `make deps` - Install and verify dependencies
2. `make test-coverage-ci` - Run tests with coverage and race detection
3. `make test-coverage-check` - Verify coverage meets 30% threshold

### Matrix test job:
1. `make deps` - Install dependencies  
2. `make test-matrix` - Run tests across OS/Go version matrix

### Lint job:
- Uses golangci-lint action directly
- Alternative: `make lint` (requires golangci-lint installed)

### Build job:
- `make build` - Build the binary

## Command Selection Guide

**For Development**:
- Quick feedback: `make test`
- Unit testing: `make test-unit`
- Coverage checking: `make test-coverage`
- Race detection: `make test-race`
- Continuous testing: `make test-watch`

**Before Commits**:
- Full validation: `make test-coverage-with-check`
- Race detection: `make test-race`
- Code quality: `make lint`

**Before Releases**:
- Complete testing: `make test-full`
- Full race detection: `make test-race-full`
- Complete coverage: `make test-coverage-full`
- Performance check: `make benchmark`

**CI/CD Usage**:
- Main pipeline: `make test-ci`
- Matrix testing: `make test-matrix`
- Coverage CI: `make test-coverage-ci`

## Coverage Thresholds and Environment

### Coverage Requirements
- **Minimum coverage**: 30% (enforced in CI)
- **Coverage check**: Automatically enforced with `make test-coverage-check`
- **Files generated**: `coverage.out`, `coverage.html`

### Environment Requirements
- **CGO_ENABLED=1**: Required for most tests (SQLite database functionality)
- **Go version**: 1.21+ (as specified in workflows)
- **Optional tools**: 
  - `entr` for watch mode (`brew install entr`)
  - `golangci-lint` for linting
  - `bc` for coverage calculations

### Test Categories by Location
- **Unit tests**: `./internal/...` - Business logic, isolated testing
- **Integration tests**: `./cmd/...` - CLI commands, end-to-end functionality
- **All tests**: `./...` - Complete test suite

## Coverage Results

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