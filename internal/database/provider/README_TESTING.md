# PostgreSQL Provider Testing Guide

This document describes the comprehensive test suite for the PostgreSQL database provider.

## Test Categories

### 1. Unit Tests (`postgresql_provider_test.go`)
- **Basic Interface Testing**: Provider instantiation, method signatures, basic functionality
- **DSN Building and Validation**: Connection string construction, parameter handling
- **SSL Configuration Testing**: SSL mode validation, certificate handling
- **Error Handling**: Edge cases, invalid inputs, disconnected operations
- **Concurrent Access**: Thread safety, race condition prevention
- **Statistics and Monitoring**: Performance metrics, health checks

### 2. Integration Tests (`postgresql_integration_test.go`)
- **Full Connection Lifecycle**: Connect, operations, disconnect sequences
- **Database Schema Management**: Migrations, table operations, schema evolution
- **Transaction Management**: ACID compliance, isolation levels, concurrent transactions
- **Connection Pool Management**: Pool exhaustion, recovery, concurrent access
- **Performance Under Load**: Sustained operations, resource management
- **Data Consistency**: Foreign keys, constraints, referential integrity

### 3. Security Tests (`postgresql_security_test.go`)
- **SSL/TLS Validation**: Certificate validation, secure connections
- **Credential Protection**: No credential leakage in logs or errors
- **DSN Injection Protection**: SQL injection prevention
- **Connection Security**: Timeout enforcement, secure error handling
- **Access Control**: Proper authorization, resource limits
- **Concurrent Access Security**: Thread safety, race condition prevention

### 4. Performance Tests (`postgresql_performance_test.go`)
- **Connection Performance**: Connection establishment benchmarks
- **Operation Throughput**: CRUD operation performance
- **Concurrent Load Testing**: Multi-threaded performance
- **Transaction Performance**: Transaction throughput, batch operations
- **Memory Usage**: Large result sets, memory efficiency
- **Connection Pool Efficiency**: Pool configuration optimization

## Test Setup Requirements

### Environment Variables
Set these environment variables for integration testing:

```bash
# Enable integration tests
export INTEGRATION_TESTS=true

# PostgreSQL connection settings
export POSTGRES_TEST_HOST=localhost
export POSTGRES_TEST_PORT=5432
export POSTGRES_TEST_USER=postgres
export POSTGRES_TEST_PASSWORD=postgres
export POSTGRES_TEST_DB=test_shelly_manager

# Enable performance tests
export PERFORMANCE_TESTS=true
export BENCHMARK_TESTS=true
```

### PostgreSQL Setup
You can use Docker to run PostgreSQL for testing:

```bash
docker run --name postgres-test -e POSTGRES_PASSWORD=postgres -p 5432:5432 -d postgres:15

# Create test database
docker exec postgres-test createdb -U postgres test_shelly_manager
```

Or use an existing PostgreSQL instance by setting the environment variables accordingly.

## Running Tests

### All Tests
```bash
go test ./internal/database/provider -v
```

### Unit Tests Only
```bash
go test ./internal/database/provider -v -short
```

### Integration Tests
```bash
INTEGRATION_TESTS=true go test ./internal/database/provider -v -run TestPostgreSQLIntegration
```

### Security Tests
```bash
go test ./internal/database/provider -v -run Security
```

### Performance Tests
```bash
PERFORMANCE_TESTS=true go test ./internal/database/provider -v -run Performance
```

### Benchmarks
```bash
BENCHMARK_TESTS=true go test ./internal/database/provider -bench=. -benchmem
```

### Build Tags
Some tests use build tags for conditional compilation:

```bash
# Integration tests
go test -tags=integration ./internal/database/provider

# Performance tests  
go test -tags=performance ./internal/database/provider
```

## Test Coverage

### Target Coverage Metrics
- **Unit Test Coverage**: >95%
- **Integration Test Coverage**: >90%
- **Security Test Coverage**: >85%
- **Performance Test Coverage**: Critical paths covered

### Coverage Analysis
```bash
go test ./internal/database/provider -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## Test Structure

### Test Suites
The tests are organized into test suites using testify/suite:

- `PostgreSQLTestSuite`: Basic integration testing with real database
- `PostgreSQLIntegrationTestSuite`: Comprehensive integration scenarios
- Individual test functions for unit and security testing

### Test Helpers
- `postgresql_test_helpers.go`: Common test utilities and setup functions
- `SetupPostgreSQLContainer()`: Database container management
- `CreateTestProvider()`: Provider factory for tests
- `CreateTestSchema()`: Test table creation
- `SeedTestData()`: Test data generation

### Test Models
Test models are defined for comprehensive testing:
- `TestModelUser`: User entity with constraints
- `TestModelProduct`: Product entity with foreign keys

## Security Testing Focus Areas

### 1. SSL/TLS Security
- Certificate validation
- SSL mode enforcement
- Secure connection establishment
- Certificate file existence checks

### 2. Credential Protection
- No credentials in error messages
- No credentials in logs
- Secure DSN handling
- Connection string sanitization

### 3. Injection Prevention
- DSN injection attacks
- SQL injection through configuration
- Parameter sanitization
- Input validation

### 4. Resource Security
- Connection limits enforcement
- Timeout enforcement
- Resource exhaustion protection
- Concurrent access control

## Performance Testing Focus Areas

### 1. Connection Performance
- Connection establishment time
- Connection pool efficiency
- Connection reuse patterns
- Pool configuration optimization

### 2. Operation Performance
- CRUD operation throughput
- Query execution time
- Transaction performance
- Batch operation efficiency

### 3. Concurrent Performance
- Multi-threaded access
- Connection pool under load
- Transaction isolation performance
- Lock contention analysis

### 4. Resource Management
- Memory usage patterns
- Connection pool utilization
- Query caching effectiveness
- Resource cleanup efficiency

## Test Maintenance

### Adding New Tests
1. Follow existing naming conventions
2. Use appropriate test categories (unit/integration/security/performance)
3. Include proper setup and teardown
4. Document test purpose and expected behavior
5. Ensure tests are deterministic and reproducible

### Test Data Management
- Use temporary databases for integration tests
- Clean up test data after each test
- Use transactions for test isolation
- Avoid test interdependencies

### Continuous Integration
Tests are designed to run in CI/CD environments:
- Graceful skipping when PostgreSQL unavailable
- Environment-based configuration
- Reasonable timeouts and resource limits
- Clear error messages for debugging

## Troubleshooting

### Common Issues
1. **PostgreSQL Not Available**: Tests will skip gracefully
2. **Permission Issues**: Ensure database user has proper permissions
3. **Connection Timeouts**: Check network connectivity and firewall settings
4. **Test Database Creation**: Ensure user can create/drop databases

### Debug Mode
Set verbose logging for debugging:
```bash
export LOG_LEVEL=debug
go test ./internal/database/provider -v -run TestSpecificTest
```

This comprehensive test suite ensures the PostgreSQL provider meets enterprise-grade quality, security, and performance requirements.