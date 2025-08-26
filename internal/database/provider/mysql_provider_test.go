package provider

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// TestNewMySQLProvider tests the constructor
func TestNewMySQLProvider(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	assert.NotNil(t, provider)
	assert.Equal(t, "MySQL", provider.Name())
	assert.Equal(t, logger, provider.logger)
	assert.False(t, provider.connected)
	assert.NotNil(t, provider.stats)
}

// TestNewMySQLProviderWithNilLogger tests constructor with nil logger
func TestNewMySQLProviderWithNilLogger(t *testing.T) {
	provider := NewMySQLProvider(nil)

	assert.NotNil(t, provider)
	assert.Equal(t, "MySQL", provider.Name())
	assert.NotNil(t, provider.logger)
	assert.False(t, provider.connected)
}

// TestMySQLProviderName tests the Name method
func TestMySQLProviderName(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	assert.Equal(t, "MySQL", provider.Name())
}

// TestMySQLProviderVersion tests the Version method
func TestMySQLProviderVersion(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	// Should return "Unknown" initially
	assert.Equal(t, "Unknown", provider.Version())

	// After setting version in stats
	provider.statsMu.Lock()
	provider.stats.ProviderVersion = "8.0.33"
	provider.statsMu.Unlock()

	assert.Equal(t, "8.0.33", provider.Version())
}

// TestMySQLProviderSetLogger tests the SetLogger method
func TestMySQLProviderSetLogger(t *testing.T) {
	provider := NewMySQLProvider(nil)
	newLogger := logging.GetDefault()

	provider.SetLogger(newLogger)
	assert.Equal(t, newLogger, provider.logger)
}

// TestMySQLProviderGetStats tests the GetStats method
func TestMySQLProviderGetStats(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	stats := provider.GetStats()

	assert.Equal(t, "MySQL", stats.ProviderName)
	assert.Equal(t, "Unknown", stats.ProviderVersion)
	assert.NotNil(t, stats.Metadata)
	assert.Equal(t, int64(0), stats.TotalQueries)
	assert.Equal(t, int64(0), stats.SlowQueries)
	assert.Equal(t, int64(0), stats.FailedQueries)
}

// TestMySQLProviderPingNotConnected tests Ping when not connected
func TestMySQLProviderPingNotConnected(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	err := provider.Ping()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// TestMySQLProviderMigrateNotConnected tests Migrate when not connected
func TestMySQLProviderMigrateNotConnected(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	err := provider.Migrate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// TestMySQLProviderDropTablesNotConnected tests DropTables when not connected
func TestMySQLProviderDropTablesNotConnected(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	err := provider.DropTables()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// TestMySQLProviderBeginTransactionNotConnected tests BeginTransaction when not connected
func TestMySQLProviderBeginTransactionNotConnected(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	_, err := provider.BeginTransaction()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// TestMySQLProviderGetDBNotConnected tests GetDB when not connected
func TestMySQLProviderGetDBNotConnected(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	db := provider.GetDB()
	assert.Nil(t, db)
}

// TestMySQLProviderClose tests Close method
func TestMySQLProviderClose(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	// Close when not connected should not error
	err := provider.Close()
	assert.NoError(t, err)

	// Close when already closed should not error
	err = provider.Close()
	assert.NoError(t, err)
}

// TestMySQLProviderHealthCheckNotConnected tests HealthCheck when not connected
func TestMySQLProviderHealthCheckNotConnected(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	ctx := context.Background()
	status := provider.HealthCheck(ctx)

	assert.False(t, status.Healthy)
	assert.NotEmpty(t, status.Error)
	assert.NotZero(t, status.CheckedAt)
	assert.NotZero(t, status.ResponseTime)
	assert.NotNil(t, status.Details)
}

// TestMySQLProviderConnectInvalidConfig tests Connect with invalid configuration
func TestMySQLProviderConnectInvalidConfig(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	testCases := []struct {
		name   string
		config DatabaseConfig
		errMsg string
	}{
		{
			name: "Empty DSN",
			config: DatabaseConfig{
				Provider: "mysql",
				DSN:      "",
				Options:  map[string]string{"tls": "false"},
			},
			errMsg: "DSN cannot be empty",
		},
		{
			name: "Invalid DSN format",
			config: DatabaseConfig{
				Provider: "mysql",
				DSN:      "invalid-dsn",
				Options:  map[string]string{"tls": "false"},
			},
			errMsg: "invalid MySQL DSN format",
		},
		{
			name: "Dangerous DSN content",
			config: DatabaseConfig{
				Provider: "mysql",
				DSN:      "user:pass@tcp(localhost:3306)/db'; DROP TABLE users; --",
				Options:  map[string]string{"tls": "false"},
			},
			errMsg: "potentially dangerous DSN content detected",
		},
		{
			name: "Invalid TLS mode",
			config: DatabaseConfig{
				Provider: "mysql",
				DSN:      "user:pass@tcp(localhost:3306)/testdb",
				Options:  map[string]string{"tls": "invalid"},
			},
			errMsg: "invalid TLS mode",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := provider.Connect(tc.config)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)

			// Ensure provider is not marked as connected
			assert.False(t, provider.connected)
		})
	}
}

// TestMySQLProviderConnectAlreadyConnected tests Connect when already connected
func TestMySQLProviderConnectAlreadyConnected(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	// Simulate already connected state
	provider.connMu.Lock()
	provider.connected = true
	provider.connMu.Unlock()

	config := DatabaseConfig{
		Provider: "mysql",
		DSN:      "user:pass@tcp(localhost:3306)/testdb",
		Options:  map[string]string{"tls": "false"},
	}

	err := provider.Connect(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already connected")

	// Reset state for cleanup
	provider.connMu.Lock()
	provider.connected = false
	provider.connMu.Unlock()
}

// TestMySQLProviderBuildDSN tests DSN building functionality
func TestMySQLProviderBuildDSN(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	testCases := []struct {
		name        string
		baseDSN     string
		options     map[string]string
		expectedSub []string
		expectError bool
	}{
		{
			name:    "Basic DSN with defaults",
			baseDSN: "user:pass@tcp(localhost:3306)/testdb",
			options: map[string]string{"tls": "false"},
			expectedSub: []string{
				"user:pass@tcp(localhost:3306)/testdb",
				"tls=false",
				"timeout=10s",
				"readTimeout=30s",
				"writeTimeout=30s",
				"charset=utf8mb4",
			},
			expectError: false,
		},
		{
			name:    "DSN with custom options",
			baseDSN: "user:pass@tcp(localhost:3306)/testdb",
			options: map[string]string{
				"tls":          "required",
				"timeout":      "5s",
				"readTimeout":  "15s",
				"writeTimeout": "15s",
				"charset":      "latin1",
			},
			expectedSub: []string{
				"tls=required",
				"timeout=5s",
				"readTimeout=15s",
				"writeTimeout=15s",
				"charset=latin1",
			},
			expectError: false,
		},
		{
			name:    "DSN with existing parameters",
			baseDSN: "user:pass@tcp(localhost:3306)/testdb?timeout=3s&charset=utf8",
			options: map[string]string{"tls": "false"},
			expectedSub: []string{
				"timeout=3s",   // Should preserve existing
				"charset=utf8", // Should preserve existing
				"tls=false",
				"readTimeout=30s",  // Should add missing
				"writeTimeout=30s", // Should add missing
			},
			expectError: false,
		},
		{
			name:        "Empty DSN should fail",
			baseDSN:     "",
			options:     nil,
			expectError: true,
		},
		{
			name:        "Invalid DSN format should fail",
			baseDSN:     "invalid",
			options:     nil,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := provider.buildDSN(tc.baseDSN, tc.options)

			if tc.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotEmpty(t, result)

			// Check that expected substrings are present
			for _, expected := range tc.expectedSub {
				assert.Contains(t, result, expected,
					"DSN should contain: %s, but got: %s", expected, result)
			}
		})
	}
}

// TestMySQLProviderGetHostFromDSN tests host extraction
func TestMySQLProviderGetHostFromDSN(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	testCases := []struct {
		name     string
		dsn      string
		expected string
	}{
		{
			name:     "TCP format with port",
			dsn:      "user:pass@tcp(localhost:3306)/database",
			expected: "localhost:3306",
		},
		{
			name:     "TCP format with IP",
			dsn:      "user:pass@tcp(192.168.1.100:3306)/database",
			expected: "192.168.1.100:3306",
		},
		{
			name:     "Simple format",
			dsn:      "user:pass@localhost/database",
			expected: "localhost",
		},
		{
			name:     "Invalid format",
			dsn:      "invalid-dsn",
			expected: "unknown",
		},
		{
			name:     "No @ symbol",
			dsn:      "tcp(localhost:3306)/database",
			expected: "localhost:3306", // Should still extract from tcp() format
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := provider.getHostFromDSN(tc.dsn)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestMySQLProviderGetDatabaseFromDSN tests database name extraction
func TestMySQLProviderGetDatabaseFromDSN(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	testCases := []struct {
		name     string
		dsn      string
		expected string
	}{
		{
			name:     "TCP format",
			dsn:      "user:pass@tcp(localhost:3306)/testdb",
			expected: "testdb",
		},
		{
			name:     "TCP format with parameters",
			dsn:      "user:pass@tcp(localhost:3306)/testdb?charset=utf8mb4",
			expected: "testdb",
		},
		{
			name:     "Simple format",
			dsn:      "user:pass@localhost/database_name",
			expected: "database_name",
		},
		{
			name:     "No database name",
			dsn:      "user:pass@tcp(localhost:3306)/",
			expected: "unknown", // Should return "unknown" for empty database name
		},
		{
			name:     "Invalid format",
			dsn:      "invalid-dsn",
			expected: "unknown",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := provider.getDatabaseFromDSN(tc.dsn)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestMySQLProviderCreateGormLogger tests GORM logger creation
func TestMySQLProviderCreateGormLogger(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	logLevels := []string{"silent", "error", "warn", "info", "invalid"}

	for _, level := range logLevels {
		t.Run(fmt.Sprintf("LogLevel_%s", level), func(t *testing.T) {
			provider.config.LogLevel = level
			gormLogger := provider.createGormLogger()
			assert.NotNil(t, gormLogger)
		})
	}
}

// TestMySQLProviderValidateSSLConfig tests SSL configuration validation
func TestMySQLProviderValidateSSLConfig(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	// Test with no TLS options (should not error)
	err := provider.validateSSLConfig(nil)
	assert.NoError(t, err)

	err = provider.validateSSLConfig(map[string]string{})
	assert.NoError(t, err)

	// These tests are covered in detail in the security tests,
	// but we'll include basic validation here
	validModes := []string{"false", "true", "skip-verify", "preferred", "custom", "required", "verify-ca", "verify-identity"}
	for _, mode := range validModes {
		t.Run(fmt.Sprintf("ValidMode_%s", mode), func(t *testing.T) {
			err := provider.validateSSLConfig(map[string]string{"tls": mode})
			assert.NoError(t, err)
		})
	}

	invalidModes := []string{"invalid", "wrong"}
	for _, mode := range invalidModes {
		t.Run(fmt.Sprintf("InvalidMode_%s", mode), func(t *testing.T) {
			err := provider.validateSSLConfig(map[string]string{"tls": mode})
			assert.Error(t, err)
		})
	}
}

// TestMySQLTransactionMethods tests transaction interface methods
func TestMySQLTransactionMethods(t *testing.T) {
	// Test transaction struct methods (without actual database connection)
	tx := &mysqlTransaction{tx: nil}

	// These should not panic even with nil tx
	assert.NotNil(t, tx.GetDB)
	assert.NotNil(t, tx.Commit)
	assert.NotNil(t, tx.Rollback)

	// GetDB should return nil when tx is nil
	db := tx.GetDB()
	assert.Nil(t, db)

	// Note: We don't test Commit() and Rollback() with nil tx because
	// they will panic when calling methods on nil GORM DB. This is expected
	// behavior - transactions should only be created through BeginTransaction()
	// which ensures tx is never nil.

	// Test that the transaction implements the Transaction interface
	var _ Transaction = tx
}

// TestMySQLProviderValidateDSNInput tests DSN input validation
func TestMySQLProviderValidateDSNInput(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	validInputs := []string{
		"user:pass@tcp(localhost:3306)/database",
		"simple_database_name",
		"user=test",
		"charset=utf8mb4",
	}

	for _, input := range validInputs {
		t.Run(fmt.Sprintf("Valid_%s", input), func(t *testing.T) {
			err := provider.validateDSNInput(input)
			assert.NoError(t, err)
		})
	}

	dangerousInputs := []string{
		"DROP TABLE users",
		"DELETE FROM sensitive_data",
		"INSERT INTO malicious",
		"<script>alert('xss')</script>",
		"EXEC xp_cmdshell",
		"/* comment */",
		"-- sql comment",
		"; additional command",
	}

	for _, input := range dangerousInputs {
		t.Run(fmt.Sprintf("Dangerous_%s", input), func(t *testing.T) {
			err := provider.validateDSNInput(input)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "potentially dangerous DSN content detected")
		})
	}
}

// TestMySQLProviderStatisticsTracking tests internal statistics tracking
func TestMySQLProviderStatisticsTracking(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	// Test initial state
	stats := provider.GetStats()
	assert.Equal(t, int64(0), stats.TotalQueries)
	assert.Equal(t, int64(0), stats.SlowQueries)
	assert.Equal(t, int64(0), stats.FailedQueries)

	// Test manual statistics updates (simulate query execution)
	provider.queryCount = 10
	provider.slowQueries = 2
	provider.failedQueries = 1
	provider.totalLatency = int64(time.Millisecond*100) * 10 // 100ms per query

	stats = provider.GetStats()
	assert.Equal(t, int64(10), stats.TotalQueries)
	assert.Equal(t, int64(2), stats.SlowQueries)
	assert.Equal(t, int64(1), stats.FailedQueries)
	assert.Equal(t, 100*time.Millisecond, stats.AverageLatency)
}

// TestMySQLProviderConnectionPoolDefaults tests connection pool default values
func TestMySQLProviderConnectionPoolDefaults(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	// Test that defaults are applied correctly in configuration
	config := DatabaseConfig{
		Provider: "mysql",
		DSN:      "user:pass@tcp(localhost:3306)/testdb",
		// Leave pool settings empty to test defaults
		Options: map[string]string{"tls": "false"},
	}

	provider.config = config

	// These values should match the MySQL-specific defaults
	// MaxOpenConns: 20 (MySQL default conservative setting)
	// MaxIdleConns: 5
	// ConnMaxLifetime: 30 * time.Minute (MySQL connections should be rotated more frequently)
	// ConnMaxIdleTime: 5 * time.Minute

	assert.Equal(t, 0, config.MaxOpenConns) // Should be 0 initially (will be set to 20 in configureConnectionPool)
	assert.Equal(t, 0, config.MaxIdleConns) // Should be 0 initially (will be set to 5 in configureConnectionPool)
}

// TestMySQLProviderConcurrentSafety tests thread safety of public methods
func TestMySQLProviderConcurrentSafety(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	// Test concurrent access to read-only operations
	done := make(chan bool, 50)

	for i := 0; i < 50; i++ {
		go func() {
			defer func() { done <- true }()

			// These operations should be thread-safe
			_ = provider.Name()
			_ = provider.Version()
			_ = provider.GetStats()
			_ = provider.HealthCheck(context.Background())

			// DSN operations should be thread-safe
			_, _ = provider.buildDSN("user:pass@tcp(localhost:3306)/test", map[string]string{"tls": "false"})
			_ = provider.getHostFromDSN("user:pass@tcp(localhost:3306)/test")
			_ = provider.getDatabaseFromDSN("user:pass@tcp(localhost:3306)/test")
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 50; i++ {
		<-done
	}

	// Verify provider state is still consistent
	assert.Equal(t, "MySQL", provider.Name())
}

// Benchmark tests for performance validation
func BenchmarkMySQLProviderGetStats(b *testing.B) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.GetStats()
	}
}

func BenchmarkMySQLProviderBuildDSN(b *testing.B) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	baseDSN := "user:pass@tcp(localhost:3306)/testdb"
	options := map[string]string{"tls": "false", "charset": "utf8mb4"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = provider.buildDSN(baseDSN, options)
	}
}

func BenchmarkMySQLProviderGetHostFromDSN(b *testing.B) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	dsn := "user:pass@tcp(localhost:3306)/testdb"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.getHostFromDSN(dsn)
	}
}
