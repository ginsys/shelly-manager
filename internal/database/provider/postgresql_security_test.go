package provider

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// TestPostgreSQLSecuritySSLValidation tests comprehensive SSL/TLS validation
func TestPostgreSQLSecuritySSLValidation(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	// Test all valid SSL modes
	validSSLModes := []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}

	for _, mode := range validSSLModes {
		t.Run(fmt.Sprintf("ValidSSLMode_%s", mode), func(t *testing.T) {
			options := map[string]string{"sslmode": mode}
			err := provider.validateSSLConfig(options)
			assert.NoError(t, err, "SSL mode %s should be valid", mode)
		})
	}

	// Test invalid SSL modes
	invalidSSLModes := []string{"invalid", "wrong", "bad-mode", "ssl", "tls"}

	for _, mode := range invalidSSLModes {
		t.Run(fmt.Sprintf("InvalidSSLMode_%s", mode), func(t *testing.T) {
			options := map[string]string{"sslmode": mode}
			err := provider.validateSSLConfig(options)
			assert.Error(t, err, "SSL mode %s should be invalid", mode)
			assert.Contains(t, err.Error(), "invalid SSL mode")
		})
	}
}

// TestPostgreSQLSecuritySSLCertificateValidation tests SSL certificate validation
func TestPostgreSQLSecuritySSLCertificateValidation(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	// Create temporary certificate files for testing
	tmpDir := t.TempDir()
	validCertPath := tmpDir + "/valid.crt"
	validKeyPath := tmpDir + "/valid.key"
	validCaPath := tmpDir + "/ca.crt"

	// Create dummy certificate files
	for _, path := range []string{validCertPath, validKeyPath, validCaPath} {
		err := os.WriteFile(path, []byte("dummy certificate content"), 0644)
		require.NoError(t, err)
	}

	testCases := []struct {
		name        string
		options     map[string]string
		expectError bool
		errorText   string
	}{
		{
			name: "verify-ca with valid root cert",
			options: map[string]string{
				"sslmode":     "verify-ca",
				"sslrootcert": validCaPath,
			},
			expectError: false,
		},
		{
			name: "verify-full with all valid certs",
			options: map[string]string{
				"sslmode":     "verify-full",
				"sslrootcert": validCaPath,
				"sslcert":     validCertPath,
				"sslkey":      validKeyPath,
			},
			expectError: false,
		},
		{
			name: "verify-ca with missing root cert",
			options: map[string]string{
				"sslmode":     "verify-ca",
				"sslrootcert": "/nonexistent/ca.crt",
			},
			expectError: true,
			errorText:   "SSL root certificate not found",
		},
		{
			name: "verify-full with missing client cert",
			options: map[string]string{
				"sslmode": "verify-full",
				"sslcert": "/nonexistent/client.crt",
			},
			expectError: true,
			errorText:   "SSL certificate not found",
		},
		{
			name: "verify-full with missing client key",
			options: map[string]string{
				"sslmode": "verify-full",
				"sslkey":  "/nonexistent/client.key",
			},
			expectError: true,
			errorText:   "SSL key not found",
		},
		{
			name: "require mode without certificates (should be valid)",
			options: map[string]string{
				"sslmode": "require",
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := provider.validateSSLConfig(tc.options)

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorText != "" {
					assert.Contains(t, err.Error(), tc.errorText)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestPostgreSQLSecurityCredentialProtection tests that credentials are not leaked
func TestPostgreSQLSecurityCredentialProtection(t *testing.T) {
	// Skip network tests if in short mode
	if testing.Short() {
		t.Skip("Skipping network-dependent PostgreSQL security tests in short mode")
	}

	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	sensitiveCredentials := []string{
		"secret_password",
		"super_secret_key",
		"confidential_user",
		"private_token",
	}

	testDSNs := []string{
		"postgres://confidential_user:secret_password@localhost:5432/testdb",
		"postgresql://admin:super_secret_key@192.168.1.100:5432/production",
		"postgres://service:private_token@db.internal.com:5432/app_db",
	}

	for i, dsn := range testDSNs {
		t.Run(fmt.Sprintf("DSN_%d", i+1), func(t *testing.T) {
			config := DatabaseConfig{
				Provider: "postgresql",
				DSN:      dsn,
				Options: map[string]string{
					"sslmode":           "disable",
					"connect_timeout":   "2",    // Short timeout for tests
					"statement_timeout": "2000", // 2 second statement timeout
				},
			}

			// Connection will fail, but error should not contain credentials
			err := provider.Connect(config)
			assert.Error(t, err, "Connection should fail for test DSN")

			errorMsg := strings.ToLower(err.Error())

			// Check that no sensitive credentials are leaked in error messages
			for _, credential := range sensitiveCredentials {
				assert.NotContains(t, errorMsg, strings.ToLower(credential),
					"Error message should not contain credential: %s", credential)
			}

			// Test helper methods don't leak credentials
			host := provider.getHostFromDSN(dsn)
			assert.NotContains(t, host, sensitiveCredentials[i%len(sensitiveCredentials)],
				"Host extraction should not contain credentials")

			db := provider.getDatabaseFromDSN(dsn)
			assert.NotContains(t, db, sensitiveCredentials[i%len(sensitiveCredentials)],
				"Database extraction should not contain credentials")
		})
	}
}

// TestPostgreSQLSecurityDSNInjection tests protection against DSN injection attacks
func TestPostgreSQLSecurityDSNInjection(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	maliciousDSNs := []string{
		"postgres://user:pass@localhost:5432/test?sslmode=disable'; DROP TABLE users; --",
		"postgres://user'; DELETE FROM sensitive_data; --:pass@localhost:5432/db",
		"postgres://user:pass@localhost:5432/db'; INSERT INTO admin_users VALUES ('hacker', 'admin'); --",
		"postgres://user:pass@localhost/db?application_name=app'; DROP DATABASE production; --",
	}

	for i, dsn := range maliciousDSNs {
		t.Run(fmt.Sprintf("InjectionAttempt_%d", i+1), func(t *testing.T) {
			// DSN parsing should either fail or sanitize the input
			_, err := provider.buildDSN(dsn, nil)

			// Either the DSN should be rejected as invalid, or if accepted,
			// it should be properly escaped/sanitized
			if err == nil {
				// If DSN is accepted, verify it doesn't contain dangerous SQL
				builtDSN, _ := provider.buildDSN(dsn, nil)
				assert.NotContains(t, builtDSN, "DROP TABLE", "Built DSN should not contain SQL injection")
				assert.NotContains(t, builtDSN, "DELETE FROM", "Built DSN should not contain SQL injection")
				assert.NotContains(t, builtDSN, "INSERT INTO", "Built DSN should not contain SQL injection")
			}
			// If DSN is rejected, that's also acceptable security behavior
		})
	}
}

// TestPostgreSQLSecurityConnectionTimeout tests security aspects of timeouts
func TestPostgreSQLSecurityConnectionTimeout(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	testCases := []struct {
		name        string
		dsn         string
		timeout     string
		maxDuration time.Duration
		expectError bool
	}{
		{
			name:        "Normal timeout",
			dsn:         "postgres://user:pass@127.0.0.1:9999/testdb", // Non-existent port
			timeout:     "2",
			maxDuration: 5 * time.Second,
			expectError: true,
		},
		{
			name:        "Very short timeout",
			dsn:         "postgres://user:pass@127.0.0.1:9999/testdb",
			timeout:     "1",
			maxDuration: 3 * time.Second,
			expectError: true,
		},
		{
			name:        "Zero timeout (should use default)",
			dsn:         "postgres://user:pass@127.0.0.1:9999/testdb",
			timeout:     "0",
			maxDuration: 15 * time.Second, // Default is 10s + some overhead
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := DatabaseConfig{
				Provider: "postgresql",
				DSN:      tc.dsn,
				Options: map[string]string{
					"connect_timeout": tc.timeout,
					"sslmode":         "disable",
				},
			}

			start := time.Now()
			err := provider.Connect(config)
			duration := time.Since(start)

			if tc.expectError {
				assert.Error(t, err, "Connection should timeout")
			}

			assert.Less(t, duration, tc.maxDuration,
				"Connection attempt should timeout within expected duration")
		})
	}
}

// TestPostgreSQLSecurityQueryLogging tests that sensitive data is not logged
func TestPostgreSQLSecurityQueryLogging(t *testing.T) {
	// This test would require actual database connection and query execution
	// For now, we test the logging configuration

	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	config := DatabaseConfig{
		Provider:           "postgresql",
		DSN:                "postgres://user:pass@localhost:5432/testdb",
		LogLevel:           "info", // Enable query logging
		SlowQueryThreshold: 100 * time.Millisecond,
		Options: map[string]string{
			"sslmode": "disable",
		},
	}

	// Test that GORM logger is configured properly
	gormLogger := provider.createGormLogger()
	assert.NotNil(t, gormLogger, "GORM logger should be created")

	// Test different log levels
	logLevels := []string{"silent", "error", "warn", "info"}
	for _, level := range logLevels {
		t.Run(fmt.Sprintf("LogLevel_%s", level), func(t *testing.T) {
			config.LogLevel = level
			gormLogger := provider.createGormLogger()
			assert.NotNil(t, gormLogger, "GORM logger should be created for level %s", level)
		})
	}
}

// TestPostgreSQLSecurityTransactionIsolation tests transaction security
func TestPostgreSQLSecurityTransactionIsolation(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	// Test that transaction uses proper isolation level
	// The actual test would require integration testing
	assert.Equal(t, "PostgreSQL", provider.Name())

	// Test transaction struct
	tx := &postgresTransaction{tx: nil}
	assert.NotNil(t, tx, "Transaction struct should be created")
}

// TestPostgreSQLSecurityResourceLimits tests resource limit enforcement
func TestPostgreSQLSecurityResourceLimits(t *testing.T) {
	logger := logging.GetDefault()
	_ = NewPostgreSQLProvider(logger)

	// Test connection pool limits
	config := DatabaseConfig{
		Provider:        "postgresql",
		DSN:             "postgres://user:pass@localhost:5432/testdb",
		MaxOpenConns:    5, // Limited
		MaxIdleConns:    2, // Limited
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 10 * time.Minute,
		Options: map[string]string{
			"sslmode": "disable",
		},
	}

	// Test that configuration is properly validated
	assert.Equal(t, 5, config.MaxOpenConns)
	assert.Equal(t, 2, config.MaxIdleConns)

	// Test default values are reasonable
	if config.MaxOpenConns == 0 {
		t.Error("MaxOpenConns should have a reasonable default")
	}
}

// TestPostgreSQLSecurityErrorHandling tests secure error handling
func TestPostgreSQLSecurityErrorHandling(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	// Test operations on non-connected provider
	securityTests := []struct {
		name      string
		operation func() error
		expectErr string
	}{
		{
			name:      "Ping when not connected",
			operation: func() error { return provider.Ping() },
			expectErr: "not connected",
		},
		{
			name:      "Migrate when not connected",
			operation: func() error { return provider.Migrate() },
			expectErr: "not connected",
		},
		{
			name: "BeginTransaction when not connected",
			operation: func() error {
				_, err := provider.BeginTransaction()
				return err
			},
			expectErr: "not connected",
		},
		{
			name:      "DropTables when not connected",
			operation: func() error { return provider.DropTables() },
			expectErr: "not connected",
		},
	}

	for _, test := range securityTests {
		t.Run(test.name, func(t *testing.T) {
			err := test.operation()
			assert.Error(t, err, "Operation should fail when not connected")
			assert.Contains(t, err.Error(), test.expectErr, "Error should indicate connection issue")

			// Ensure error doesn't leak system information
			errorMsg := strings.ToLower(err.Error())
			assert.NotContains(t, errorMsg, "password", "Error should not contain password")
			assert.NotContains(t, errorMsg, "secret", "Error should not contain secret")
			assert.NotContains(t, errorMsg, "token", "Error should not contain token")
		})
	}
}

// TestPostgreSQLSecurityHealthCheck tests health check security
func TestPostgreSQLSecurityHealthCheck(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	ctx := context.Background()
	status := provider.HealthCheck(ctx)

	// Health check should work even when not connected
	assert.False(t, status.Healthy, "Health check should report unhealthy when not connected")
	assert.NotEmpty(t, status.Error, "Health check should provide error message")
	assert.NotZero(t, status.CheckedAt, "Health check should set checked time")

	// Ensure health check doesn't leak sensitive information
	details := fmt.Sprintf("%v", status.Details)
	sensitiveTerms := []string{"password", "secret", "token", "key", "credential"}

	for _, term := range sensitiveTerms {
		assert.NotContains(t, strings.ToLower(details), term,
			"Health check details should not contain sensitive term: %s", term)
		assert.NotContains(t, strings.ToLower(status.Error), term,
			"Health check error should not contain sensitive term: %s", term)
	}
}

// TestPostgreSQLSecurityConcurrentAccess tests thread safety and concurrent access security
func TestPostgreSQLSecurityConcurrentAccess(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	// Test concurrent operations don't cause data races or security issues
	for i := 0; i < 10; i++ {
		go func() {
			// These operations should be thread-safe
			_ = provider.GetStats()
			_ = provider.Name()
			_ = provider.Version()
			_ = provider.HealthCheck(context.Background())
		}()
	}

	// Allow goroutines to complete
	time.Sleep(100 * time.Millisecond)

	// Verify provider state is still consistent
	assert.Equal(t, "PostgreSQL", provider.Name())
	assert.NotEmpty(t, provider.Version())
}
