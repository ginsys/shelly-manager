package provider

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// TestPostgreSQLProviderBasicInterface tests that PostgreSQL provider implements the interface correctly
func TestPostgreSQLProviderBasicInterface(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	// Test provider info
	assert.Equal(t, "PostgreSQL", provider.Name())
	assert.NotEmpty(t, provider.Version()) // Should return "Unknown" initially

	// Test health check when not connected
	status := provider.HealthCheck(context.Background())
	assert.False(t, status.Healthy)
	assert.NotEmpty(t, status.Error)
	assert.NotZero(t, status.CheckedAt)

	// Test stats when not connected
	stats := provider.GetStats()
	assert.Equal(t, "PostgreSQL", stats.ProviderName)
	assert.NotNil(t, stats.Metadata)
}

// TestPostgreSQLProviderConnectionFailure tests connection failure scenarios
func TestPostgreSQLProviderConnectionFailure(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	testCases := []struct {
		name   string
		config DatabaseConfig
	}{
		{
			name: "Invalid DSN",
			config: DatabaseConfig{
				Provider: "postgresql",
				DSN:      "invalid://dsn",
			},
		},
		{
			name: "Connection timeout",
			config: DatabaseConfig{
				Provider: "postgresql",
				DSN:      "postgres://user:pass@nonexistent:5432/db",
				Options: map[string]string{
					"connect_timeout": "1", // 1 second timeout
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := provider.Connect(tc.config)
			assert.Error(t, err, "Expected connection to fail")

			// Test that operations fail when not connected
			err = provider.Ping()
			assert.Error(t, err)

			err = provider.Migrate()
			assert.Error(t, err)

			_, err = provider.BeginTransaction()
			assert.Error(t, err)
		})
	}
}

// TestPostgreSQLProviderDSNBuilding tests DSN construction with various options
func TestPostgreSQLProviderDSNBuilding(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	testCases := []struct {
		name     string
		baseDSN  string
		options  map[string]string
		expected map[string]bool // Expected parts in the DSN
	}{
		{
			name:    "Basic DSN with SSL default",
			baseDSN: "postgres://user:pass@localhost:5432/testdb",
			options: nil,
			expected: map[string]bool{
				"sslmode=require":    true,
				"connect_timeout=10": true,
			},
		},
		{
			name:    "DSN with custom SSL mode",
			baseDSN: "postgres://user:pass@localhost:5432/testdb",
			options: map[string]string{
				"sslmode": "disable",
			},
			expected: map[string]bool{
				"sslmode=disable":    true,
				"connect_timeout=10": true,
			},
		},
		{
			name:    "DSN with SSL certificates",
			baseDSN: "postgres://user:pass@localhost:5432/testdb",
			options: map[string]string{
				"sslmode":     "verify-full",
				"sslcert":     "/path/to/cert.pem",
				"sslkey":      "/path/to/key.pem",
				"sslrootcert": "/path/to/ca.pem",
			},
			expected: map[string]bool{
				"sslmode=verify-full":               true,
				"sslcert=%2Fpath%2Fto%2Fcert.pem":   true,
				"sslkey=%2Fpath%2Fto%2Fkey.pem":     true,
				"sslrootcert=%2Fpath%2Fto%2Fca.pem": true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dsn, err := provider.buildDSN(tc.baseDSN, tc.options)
			require.NoError(t, err)

			for expectedPart, shouldBePresent := range tc.expected {
				if shouldBePresent {
					assert.Contains(t, dsn, expectedPart, "DSN should contain %s", expectedPart)
				} else {
					assert.NotContains(t, dsn, expectedPart, "DSN should not contain %s", expectedPart)
				}
			}
		})
	}
}

// TestPostgreSQLProviderInvalidDSN tests invalid DSN formats
func TestPostgreSQLProviderInvalidDSN(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	invalidDSNs := []string{
		"", // Empty DSN
		"not-a-url",
		"://invalid",
	}

	for _, dsn := range invalidDSNs {
		t.Run("DSN: "+dsn, func(t *testing.T) {
			_, err := provider.buildDSN(dsn, nil)
			assert.Error(t, err, "Expected error for invalid DSN: %s", dsn)
		})
	}
}

// TestPostgreSQLProviderSSLValidation tests SSL configuration validation
func TestPostgreSQLProviderSSLValidation(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	testCases := []struct {
		name          string
		options       map[string]string
		expectedError bool
	}{
		{
			name:          "No SSL options",
			options:       nil,
			expectedError: false,
		},
		{
			name: "Valid SSL mode - require",
			options: map[string]string{
				"sslmode": "require",
			},
			expectedError: false,
		},
		{
			name: "Valid SSL mode - disable",
			options: map[string]string{
				"sslmode": "disable",
			},
			expectedError: false,
		},
		{
			name: "Invalid SSL mode",
			options: map[string]string{
				"sslmode": "invalid-mode",
			},
			expectedError: true,
		},
		{
			name: "SSL verify mode with non-existent certificate",
			options: map[string]string{
				"sslmode":     "verify-ca",
				"sslrootcert": "/nonexistent/ca.pem",
			},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := provider.validateSSLConfig(tc.options)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestPostgreSQLProviderStatsAndMetrics tests statistics collection
func TestPostgreSQLProviderStatsAndMetrics(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	// Get stats when not connected
	stats := provider.GetStats()
	assert.Equal(t, "PostgreSQL", stats.ProviderName)
	assert.Equal(t, int64(0), stats.TotalQueries)
	assert.Equal(t, int64(0), stats.SlowQueries)
	assert.Equal(t, int64(0), stats.FailedQueries)
	assert.Equal(t, time.Duration(0), stats.AverageLatency)
	assert.NotNil(t, stats.Metadata)

	// Test concurrent access to statistics
	var wg sync.WaitGroup
	errorChan := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// This should not panic or cause race conditions
			stats := provider.GetStats()
			if stats.ProviderName != "PostgreSQL" {
				errorChan <- fmt.Errorf("unexpected provider name: %s", stats.ProviderName)
			}
		}()
	}

	wg.Wait()
	close(errorChan)

	for err := range errorChan {
		assert.NoError(t, err)
	}
}

// TestPostgreSQLProviderDoubleConnect tests that double connection fails appropriately
func TestPostgreSQLProviderDoubleConnect(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	// Mock a successful connection by setting connected = true
	provider.connMu.Lock()
	provider.connected = true
	provider.connMu.Unlock()

	config := DatabaseConfig{
		Provider: "postgresql",
		DSN:      "postgres://user:pass@localhost:5432/testdb",
	}

	// Second connection should fail
	err := provider.Connect(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already connected")
}

// TestPostgreSQLProviderClose tests connection closing
func TestPostgreSQLProviderClose(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	// Close when not connected should not error
	err := provider.Close()
	assert.NoError(t, err)

	// Close multiple times should not error
	err = provider.Close()
	assert.NoError(t, err)
}

// TestPostgreSQLTransaction tests transaction interface implementation
func TestPostgreSQLTransaction(t *testing.T) {
	// Test transaction struct methods without actual database
	tx := &postgresTransaction{tx: nil}

	// These will return nil/errors since tx is nil, but we're testing the interface
	db := tx.GetDB()
	assert.Nil(t, db) // Returns nil since tx.tx is nil

	// Note: We can't test Commit() and Rollback() with nil tx as they panic
	// This would require a real database connection for proper testing
	// The interface implementation is verified by compilation
}

// PostgreSQLTestSuite provides comprehensive testing with actual PostgreSQL
type PostgreSQLTestSuite struct {
	suite.Suite
	provider *PostgreSQLProvider
	logger   *logging.Logger
	testDB   *sql.DB
	config   DatabaseConfig
}

// SetupSuite runs once before all tests in the suite
func (suite *PostgreSQLTestSuite) SetupSuite() {
	suite.logger = logging.GetDefault()

	// Check if PostgreSQL is available for integration testing
	pgHost := os.Getenv("POSTGRES_HOST")
	if pgHost == "" {
		pgHost = "localhost"
	}

	pgPort := os.Getenv("POSTGRES_PORT")
	if pgPort == "" {
		pgPort = "5432"
	}

	pgUser := os.Getenv("POSTGRES_USER")
	if pgUser == "" {
		pgUser = "postgres"
	}

	pgPassword := os.Getenv("POSTGRES_PASSWORD")
	if pgPassword == "" {
		pgPassword = "postgres"
	}

	pgDatabase := os.Getenv("POSTGRES_DB")
	if pgDatabase == "" {
		pgDatabase = "test_shelly_manager"
	}

	// Create test configuration
	suite.config = DatabaseConfig{
		Provider:           "postgresql",
		DSN:                fmt.Sprintf("postgres://%s:%s@%s:%s/%s", pgUser, pgPassword, pgHost, pgPort, pgDatabase),
		MaxOpenConns:       10,
		MaxIdleConns:       2,
		ConnMaxLifetime:    time.Hour,
		ConnMaxIdleTime:    10 * time.Minute,
		SlowQueryThreshold: 200 * time.Millisecond,
		LogLevel:           "error",
		Options: map[string]string{
			"sslmode": "disable", // Use disable for local testing
		},
	}

	// Test basic PostgreSQL connectivity
	var err error
	suite.testDB, err = sql.Open("pgx", suite.config.DSN+"?sslmode=disable")
	if err != nil {
		suite.T().Skip("PostgreSQL not available for integration testing: " + err.Error())
		return
	}

	if pingErr := suite.testDB.Ping(); pingErr != nil {
		suite.T().Skip("PostgreSQL not available for integration testing: " + pingErr.Error())
		return
	}

	// Create test database if it doesn't exist
	_, _ = suite.testDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", pgDatabase))
	_, err = suite.testDB.Exec(fmt.Sprintf("CREATE DATABASE %s", pgDatabase))
	if err != nil {
		suite.T().Skip("Failed to create test database: " + err.Error())
		return
	}
}

// SetupTest runs before each test
func (suite *PostgreSQLTestSuite) SetupTest() {
	suite.provider = NewPostgreSQLProvider(suite.logger)
}

// TearDownTest runs after each test
func (suite *PostgreSQLTestSuite) TearDownTest() {
	if suite.provider != nil {
		_ = suite.provider.Close()
	}
}

// TearDownSuite runs once after all tests in the suite
func (suite *PostgreSQLTestSuite) TearDownSuite() {
	if suite.testDB != nil {
		_ = suite.testDB.Close()
	}
}

// TestPostgreSQLIntegration runs the complete integration test suite
func TestPostgreSQLIntegration(t *testing.T) {
	// Skip integration tests if not explicitly enabled
	if os.Getenv("INTEGRATION_TESTS") != "true" {
		t.Skip("Integration tests disabled. Set INTEGRATION_TESTS=true to enable.")
	}

	suite.Run(t, new(PostgreSQLTestSuite))
}

// Test successful connection and basic operations
func (suite *PostgreSQLTestSuite) TestSuccessfulConnection() {
	err := suite.provider.Connect(suite.config)
	suite.Require().NoError(err)

	// Test provider info
	suite.Equal("PostgreSQL", suite.provider.Name())
	suite.NotEqual("Unknown", suite.provider.Version())

	// Test ping
	err = suite.provider.Ping()
	suite.NoError(err)

	// Test GetDB
	db := suite.provider.GetDB()
	suite.NotNil(db)
}

// Test connection with various SSL modes
func (suite *PostgreSQLTestSuite) TestSSLModes() {
	testCases := []struct {
		name        string
		sslMode     string
		expectError bool
	}{
		{"SSL Disable", "disable", false},
		{"SSL Prefer", "prefer", false},
		{"SSL Require", "require", false}, // May fail without SSL setup
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			provider := NewPostgreSQLProvider(suite.logger)
			config := suite.config
			config.Options = map[string]string{"sslmode": tc.sslMode}

			err := provider.Connect(config)
			defer func() { _ = provider.Close() }()

			if tc.expectError {
				suite.Error(err)
			} else {
				// SSL require might fail in test environment, so we're lenient
				if tc.sslMode != "require" {
					suite.NoError(err)
				}
			}
		})
	}
}

// Test connection pool behavior
func (suite *PostgreSQLTestSuite) TestConnectionPool() {
	err := suite.provider.Connect(suite.config)
	suite.Require().NoError(err)

	// Get stats and check pool configuration
	stats := suite.provider.GetStats()
	suite.Equal("PostgreSQL", stats.ProviderName)

	// Test concurrent connections
	var wg sync.WaitGroup
	errorChan := make(chan error, 20)

	// Launch multiple goroutines to test concurrent access
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Test ping from multiple goroutines
			if err := suite.provider.Ping(); err != nil {
				errorChan <- fmt.Errorf("goroutine %d ping failed: %w", id, err)
				return
			}

			// Test getting DB and executing query
			db := suite.provider.GetDB()
			if db == nil {
				errorChan <- fmt.Errorf("goroutine %d got nil DB", id)
				return
			}

			var result int
			if err := db.Raw("SELECT 1").Scan(&result).Error; err != nil {
				errorChan <- fmt.Errorf("goroutine %d query failed: %w", id, err)
				return
			}

			if result != 1 {
				errorChan <- fmt.Errorf("goroutine %d got unexpected result: %d", id, result)
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	// Check for any errors from concurrent operations
	for err := range errorChan {
		suite.NoError(err)
	}
}

// Test transaction operations
func (suite *PostgreSQLTestSuite) TestTransactions() {
	err := suite.provider.Connect(suite.config)
	suite.Require().NoError(err)

	// Create a test table
	type TestModel struct {
		ID   uint   `gorm:"primaryKey"`
		Name string `gorm:"not null"`
	}

	err = suite.provider.Migrate(&TestModel{})
	suite.Require().NoError(err)

	// Test successful transaction
	tx, err := suite.provider.BeginTransaction()
	suite.Require().NoError(err)
	suite.NotNil(tx)

	// Insert data in transaction
	testModel := TestModel{Name: "test"}
	err = tx.GetDB().Create(&testModel).Error
	suite.NoError(err)

	// Commit transaction
	err = tx.Commit()
	suite.NoError(err)

	// Verify data was committed
	var count int64
	err = suite.provider.GetDB().Model(&TestModel{}).Count(&count).Error
	suite.NoError(err)
	suite.Equal(int64(1), count)

	// Test rollback transaction
	tx2, err := suite.provider.BeginTransaction()
	suite.Require().NoError(err)

	testModel2 := TestModel{Name: "rollback_test"}
	err = tx2.GetDB().Create(&testModel2).Error
	suite.NoError(err)

	// Rollback transaction
	err = tx2.Rollback()
	suite.NoError(err)

	// Verify data was not committed
	err = suite.provider.GetDB().Model(&TestModel{}).Count(&count).Error
	suite.NoError(err)
	suite.Equal(int64(1), count) // Should still be 1, not 2

	// Clean up
	err = suite.provider.DropTables(&TestModel{})
	suite.NoError(err)
}

// Test migration operations
func (suite *PostgreSQLTestSuite) TestMigrations() {
	err := suite.provider.Connect(suite.config)
	suite.Require().NoError(err)

	type TestModel struct {
		ID        uint      `gorm:"primaryKey"`
		Name      string    `gorm:"not null"`
		Email     string    `gorm:"uniqueIndex"`
		CreatedAt time.Time `gorm:"autoCreateTime"`
	}

	// Test migration
	err = suite.provider.Migrate(&TestModel{})
	suite.NoError(err)

	// Test that table exists by inserting data
	testModel := TestModel{
		Name:  "Test User",
		Email: "test@example.com",
	}

	err = suite.provider.GetDB().Create(&testModel).Error
	suite.NoError(err)
	suite.NotZero(testModel.ID)

	// Test drop tables
	err = suite.provider.DropTables(&TestModel{})
	suite.NoError(err)

	// Verify table was dropped
	err = suite.provider.GetDB().Create(&testModel).Error
	suite.Error(err) // Should fail because table doesn't exist
}

// Test health check functionality
func (suite *PostgreSQLTestSuite) TestHealthCheck() {
	err := suite.provider.Connect(suite.config)
	suite.Require().NoError(err)

	ctx := context.Background()
	status := suite.provider.HealthCheck(ctx)

	suite.True(status.Healthy)
	suite.Empty(status.Error)
	suite.NotZero(status.ResponseTime)
	suite.NotZero(status.CheckedAt)
	suite.NotNil(status.Details)

	// Check that details contain expected information
	suite.Contains(status.Details, "database_size")
	suite.Contains(status.Details, "total_queries")
	suite.Contains(status.Details, "connection_count")
	suite.Contains(status.Details, "version")
}

// Test statistics collection
func (suite *PostgreSQLTestSuite) TestStatistics() {
	err := suite.provider.Connect(suite.config)
	suite.Require().NoError(err)

	stats := suite.provider.GetStats()
	suite.Equal("PostgreSQL", stats.ProviderName)
	suite.NotEqual("Unknown", stats.ProviderVersion)
	suite.NotNil(stats.Metadata)

	// Database size should be available for PostgreSQL
	suite.GreaterOrEqual(stats.DatabaseSize, int64(0))

	// Connection stats should be reasonable
	suite.GreaterOrEqual(stats.OpenConnections, 0)
	suite.GreaterOrEqual(stats.IdleConnections, 0)
	suite.GreaterOrEqual(stats.InUseConnections, 0)
}

// Test connection timeout and error scenarios
func (suite *PostgreSQLTestSuite) TestConnectionTimeoutAndErrors() {
	// Test connection timeout with invalid host
	provider := NewPostgreSQLProvider(suite.logger)
	config := DatabaseConfig{
		Provider: "postgresql",
		DSN:      "postgres://user:pass@nonexistent.host.invalid:5432/testdb",
		Options: map[string]string{
			"connect_timeout": "2",
			"sslmode":         "disable",
		},
	}

	start := time.Now()
	err := provider.Connect(config)
	duration := time.Since(start)

	suite.Error(err)
	// Should timeout within reasonable time (allowing some overhead)
	suite.Less(duration, 10*time.Second)
}

// Test concurrent connection stress
func (suite *PostgreSQLTestSuite) TestConcurrentConnectionStress() {
	err := suite.provider.Connect(suite.config)
	suite.Require().NoError(err)

	// Create a test table for stress testing
	type StressTestModel struct {
		ID    uint   `gorm:"primaryKey"`
		Value string `gorm:"not null"`
	}

	err = suite.provider.Migrate(&StressTestModel{})
	suite.Require().NoError(err)
	defer func() { _ = suite.provider.DropTables(&StressTestModel{}) }()

	var wg sync.WaitGroup
	errorChan := make(chan error, 50)

	// Launch many concurrent operations
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Mix of read and write operations
			db := suite.provider.GetDB()

			// Insert operation
			model := StressTestModel{Value: fmt.Sprintf("test_%d", id)}
			if createErr := db.Create(&model).Error; createErr != nil {
				errorChan <- fmt.Errorf("insert failed for id %d: %w", id, createErr)
				return
			}

			// Read operation
			var count int64
			if countErr := db.Model(&StressTestModel{}).Count(&count).Error; countErr != nil {
				errorChan <- fmt.Errorf("count failed for id %d: %w", id, countErr)
				return
			}

			// Transaction operation
			tx, txErr := suite.provider.BeginTransaction()
			if txErr != nil {
				errorChan <- fmt.Errorf("begin tx failed for id %d: %w", id, txErr)
				return
			}

			txModel := StressTestModel{Value: fmt.Sprintf("tx_test_%d", id)}
			if txCreateErr := tx.GetDB().Create(&txModel).Error; txCreateErr != nil {
				_ = tx.Rollback()
				errorChan <- fmt.Errorf("tx insert failed for id %d: %w", id, txCreateErr)
				return
			}

			if commitErr := tx.Commit(); commitErr != nil {
				errorChan <- fmt.Errorf("tx commit failed for id %d: %w", id, commitErr)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		suite.NoError(err)
	}

	// Verify all data was inserted
	var finalCount int64
	err = suite.provider.GetDB().Model(&StressTestModel{}).Count(&finalCount).Error
	suite.NoError(err)
	suite.Equal(int64(50), finalCount) // 25 regular inserts + 25 transaction inserts
}

// Comprehensive DSN building tests
func TestPostgreSQLDSNBuildingComprehensive(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	testCases := []struct {
		name        string
		baseDSN     string
		options     map[string]string
		expected    []string // Parts that must be present
		notExpected []string // Parts that must not be present
		expectError bool
	}{
		{
			name:    "Basic DSN with defaults",
			baseDSN: "postgres://user:pass@localhost:5432/testdb",
			options: nil,
			expected: []string{
				"postgres://user:pass@localhost:5432/testdb",
				"sslmode=require",
				"connect_timeout=10",
			},
		},
		{
			name:    "DSN with custom SSL mode",
			baseDSN: "postgres://user:pass@localhost:5432/testdb",
			options: map[string]string{
				"sslmode": "disable",
			},
			expected: []string{
				"sslmode=disable",
				"connect_timeout=10",
			},
			notExpected: []string{"sslmode=require"},
		},
		{
			name:    "DSN with existing query parameters",
			baseDSN: "postgres://user:pass@localhost:5432/testdb?application_name=test",
			options: map[string]string{
				"sslmode": "prefer",
			},
			expected: []string{
				"application_name=test",
				"sslmode=prefer",
				"connect_timeout=10",
			},
		},
		{
			name:    "DSN with SSL certificates",
			baseDSN: "postgres://user:pass@localhost:5432/testdb",
			options: map[string]string{
				"sslmode":     "verify-full",
				"sslcert":     "/path/to/client.crt",
				"sslkey":      "/path/to/client.key",
				"sslrootcert": "/path/to/ca.crt",
			},
			expected: []string{
				"sslmode=verify-full",
				"sslcert=",
				"sslkey=",
				"sslrootcert=",
			},
		},
		{
			name:    "DSN with custom timeout",
			baseDSN: "postgres://user:pass@localhost:5432/testdb",
			options: map[string]string{
				"connect_timeout": "30",
			},
			expected: []string{
				"connect_timeout=30",
				"sslmode=require",
			},
			notExpected: []string{"connect_timeout=10"},
		},
		{
			name:        "Empty DSN",
			baseDSN:     "",
			expectError: true,
		},
		{
			name:        "Invalid DSN scheme",
			baseDSN:     "mysql://user:pass@localhost:3306/testdb",
			expectError: true,
		},
		{
			name:        "Malformed URL",
			baseDSN:     "not-a-url",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dsn, err := provider.buildDSN(tc.baseDSN, tc.options)

			if tc.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			for _, expected := range tc.expected {
				assert.Contains(t, dsn, expected, "DSN should contain %s", expected)
			}

			for _, notExpected := range tc.notExpected {
				assert.NotContains(t, dsn, notExpected, "DSN should not contain %s", notExpected)
			}
		})
	}
}

// Test SSL configuration validation comprehensively
func TestPostgreSQLSSLValidationComprehensive(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	testCases := []struct {
		name        string
		options     map[string]string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "No SSL options (valid)",
			options:     nil,
			expectError: false,
		},
		{
			name: "Valid SSL mode - disable",
			options: map[string]string{
				"sslmode": "disable",
			},
			expectError: false,
		},
		{
			name: "Valid SSL mode - allow",
			options: map[string]string{
				"sslmode": "allow",
			},
			expectError: false,
		},
		{
			name: "Valid SSL mode - prefer",
			options: map[string]string{
				"sslmode": "prefer",
			},
			expectError: false,
		},
		{
			name: "Valid SSL mode - require",
			options: map[string]string{
				"sslmode": "require",
			},
			expectError: false,
		},
		{
			name: "Valid SSL mode - verify-ca",
			options: map[string]string{
				"sslmode": "verify-ca",
			},
			expectError: false, // No cert files specified, but mode is valid
		},
		{
			name: "Valid SSL mode - verify-full",
			options: map[string]string{
				"sslmode": "verify-full",
			},
			expectError: false, // No cert files specified, but mode is valid
		},
		{
			name: "Invalid SSL mode",
			options: map[string]string{
				"sslmode": "invalid-mode",
			},
			expectError: true,
			errorMsg:    "invalid SSL mode",
		},
		{
			name: "SSL verify-ca with non-existent root cert",
			options: map[string]string{
				"sslmode":     "verify-ca",
				"sslrootcert": "/nonexistent/ca.pem",
			},
			expectError: true,
			errorMsg:    "SSL root certificate not found",
		},
		{
			name: "SSL verify-full with non-existent client cert",
			options: map[string]string{
				"sslmode": "verify-full",
				"sslcert": "/nonexistent/client.pem",
			},
			expectError: true,
			errorMsg:    "SSL certificate not found",
		},
		{
			name: "SSL verify-full with non-existent client key",
			options: map[string]string{
				"sslmode": "verify-full",
				"sslkey":  "/nonexistent/client.key",
			},
			expectError: true,
			errorMsg:    "SSL key not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := provider.validateSSLConfig(tc.options)

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test credential security - ensure no credentials are leaked in logs or errors
func TestPostgreSQLCredentialSecurity(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	// Test with sensitive credentials
	sensitiveConfig := DatabaseConfig{
		Provider: "postgresql",
		DSN:      "postgres://secret_user:super_secret_password@nonexistent.host:5432/testdb",
		Options: map[string]string{
			"sslmode": "disable",
		},
	}

	// Connection should fail, but error should not contain credentials
	err := provider.Connect(sensitiveConfig)
	assert.Error(t, err)

	errorMsg := err.Error()
	assert.NotContains(t, errorMsg, "secret_user", "Error message should not contain username")
	assert.NotContains(t, errorMsg, "super_secret_password", "Error message should not contain password")

	// Test DSN helper methods don't leak credentials
	host := provider.getHostFromDSN(sensitiveConfig.DSN)
	assert.Contains(t, host, "nonexistent.host") // Host should be present
	assert.NotContains(t, host, "secret_user", "Host extraction should not contain username")
	assert.NotContains(t, host, "super_secret_password", "Host extraction should not contain password")

	db := provider.getDatabaseFromDSN(sensitiveConfig.DSN)
	assert.Equal(t, "testdb", db)
	assert.NotContains(t, db, "secret_user", "Database extraction should not contain username")
	assert.NotContains(t, db, "super_secret_password", "Database extraction should not contain password")
}

// Test error handling for operations on disconnected provider
func TestPostgreSQLOperationsWhenDisconnected(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	// Test operations when not connected
	err := provider.Ping()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")

	err = provider.Migrate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")

	_, err = provider.BeginTransaction()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")

	err = provider.DropTables()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")

	// GetDB should return nil when not connected
	db := provider.GetDB()
	assert.Nil(t, db)

	// Health check should indicate unhealthy
	status := provider.HealthCheck(context.Background())
	assert.False(t, status.Healthy)
	assert.NotEmpty(t, status.Error)

	// GetStats should work even when not connected
	stats := provider.GetStats()
	assert.Equal(t, "PostgreSQL", stats.ProviderName)
	assert.Equal(t, int64(0), stats.TotalQueries)
}

// Test provider info and version detection
func TestPostgreSQLProviderInfo(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	// Test basic provider info
	assert.Equal(t, "PostgreSQL", provider.Name())
	assert.Equal(t, "Unknown", provider.Version()) // Should be "Unknown" before connection

	// Test logger setting
	newLogger := logging.GetDefault()
	provider.SetLogger(newLogger)
	assert.Equal(t, newLogger, provider.logger)
}

// Performance benchmark test
func BenchmarkPostgreSQLOperations(b *testing.B) {
	if os.Getenv("BENCHMARK_TESTS") != "true" {
		b.Skip("Benchmark tests disabled. Set BENCHMARK_TESTS=true to enable.")
	}

	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	config := DatabaseConfig{
		Provider: "postgresql",
		DSN:      "postgres://postgres:postgres@localhost:5432/benchmark_test",
		Options:  map[string]string{"sslmode": "disable"},
	}

	err := provider.Connect(config)
	if err != nil {
		b.Skip("PostgreSQL not available for benchmarking: " + err.Error())
	}
	defer func() { _ = provider.Close() }()

	b.ResetTimer()

	b.Run("Ping", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = provider.Ping()
		}
	})

	b.Run("GetStats", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = provider.GetStats()
		}
	})

	b.Run("HealthCheck", func(b *testing.B) {
		ctx := context.Background()
		for i := 0; i < b.N; i++ {
			_ = provider.HealthCheck(ctx)
		}
	})
}
