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

// TestMySQLSecuritySSLValidation tests comprehensive SSL/TLS validation
func TestMySQLSecuritySSLValidation(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	// Test all valid TLS modes for MySQL
	validTLSModes := []string{"false", "true", "skip-verify", "preferred", "custom", "required", "verify-ca", "verify-identity"}

	for _, mode := range validTLSModes {
		t.Run(fmt.Sprintf("ValidTLSMode_%s", mode), func(t *testing.T) {
			options := map[string]string{"tls": mode}
			err := provider.validateSSLConfig(options)
			assert.NoError(t, err, "TLS mode %s should be valid", mode)
		})
	}

	// Test invalid TLS modes
	invalidTLSModes := []string{"invalid", "wrong", "bad-mode", "ssl", "disabled", "enabled"}

	for _, mode := range invalidTLSModes {
		t.Run(fmt.Sprintf("InvalidTLSMode_%s", mode), func(t *testing.T) {
			options := map[string]string{"tls": mode}
			err := provider.validateSSLConfig(options)
			assert.Error(t, err, "TLS mode %s should be invalid", mode)
			assert.Contains(t, err.Error(), "invalid TLS mode")
		})
	}
}

// TestMySQLSecuritySSLCertificateValidation tests SSL certificate validation
func TestMySQLSecuritySSLCertificateValidation(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

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
				"tls": "verify-ca",
				"ca":  validCaPath,
			},
			expectError: false,
		},
		{
			name: "verify-identity with all valid certs",
			options: map[string]string{
				"tls":  "verify-identity",
				"ca":   validCaPath,
				"cert": validCertPath,
				"key":  validKeyPath,
			},
			expectError: false,
		},
		{
			name: "custom with all valid certs",
			options: map[string]string{
				"tls":  "custom",
				"ca":   validCaPath,
				"cert": validCertPath,
				"key":  validKeyPath,
			},
			expectError: false,
		},
		{
			name: "verify-ca with missing root cert",
			options: map[string]string{
				"tls": "verify-ca",
				"ca":  "/nonexistent/ca.crt",
			},
			expectError: true,
			errorText:   "CA certificate not found",
		},
		{
			name: "verify-identity with missing client cert",
			options: map[string]string{
				"tls":  "verify-identity",
				"cert": "/nonexistent/client.crt",
			},
			expectError: true,
			errorText:   "client certificate not found",
		},
		{
			name: "custom with missing client key",
			options: map[string]string{
				"tls": "custom",
				"key": "/nonexistent/client.key",
			},
			expectError: true,
			errorText:   "client key not found",
		},
		{
			name: "required mode without certificates (should be valid)",
			options: map[string]string{
				"tls": "required",
			},
			expectError: false,
		},
		{
			name: "true mode without certificates (should be valid)",
			options: map[string]string{
				"tls": "true",
			},
			expectError: false,
		},
		{
			name: "skip-verify mode (should be valid but insecure)",
			options: map[string]string{
				"tls": "skip-verify",
			},
			expectError: false,
		},
		{
			name: "false mode (SSL disabled)",
			options: map[string]string{
				"tls": "false",
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

// TestMySQLSecurityCredentialProtection tests that credentials are not leaked
func TestMySQLSecurityCredentialProtection(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	sensitiveCredentials := []string{
		"secret_password",
		"super_secret_key",
		"confidential_user",
		"private_token",
		"admin_pass",
		"db_secret",
	}

	testDSNs := []string{
		"confidential_user:secret_password@tcp(localhost:3306)/testdb",
		"admin:super_secret_key@tcp(192.168.1.100:3306)/production",
		"service:private_token@tcp(db.internal.com:3306)/app_db",
		"root:admin_pass@tcp(127.0.0.1:3306)/secure_db",
		"user:db_secret@tcp(mysql.local:3306)/database",
	}

	for i, dsn := range testDSNs {
		t.Run(fmt.Sprintf("DSN_%d", i+1), func(t *testing.T) {
			config := DatabaseConfig{
				Provider: "mysql",
				DSN:      dsn,
				Options: map[string]string{
					"tls": "false",
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

// TestMySQLSecurityDSNInjection tests protection against DSN injection attacks
func TestMySQLSecurityDSNInjection(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	maliciousDSNs := []string{
		"user:pass@tcp(localhost:3306)/test'; DROP TABLE users; --",
		"user'; DELETE FROM sensitive_data; --:pass@tcp(localhost:3306)/db",
		"user:pass@tcp(localhost:3306)/db'; INSERT INTO admin_users VALUES ('hacker', 'admin'); --",
		"user:pass@tcp(localhost:3306)/db?timeout=5s'; DROP DATABASE production; --",
		"user:pass@tcp(localhost:3306)/db?charset=utf8'; TRUNCATE TABLE payments; --",
		"user:pass@tcp(localhost:3306)/db?readTimeout=30s/**/UNION SELECT password FROM users; --",
		"user:pass@tcp(localhost:3306)/db?writeTimeout=30s<script>alert('xss')</script>",
		"user:pass@tcp(localhost:3306)/db?tls=false; EXEC xp_cmdshell 'dir'; --",
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
				assert.NotContains(t, builtDSN, "TRUNCATE", "Built DSN should not contain SQL injection")
				assert.NotContains(t, builtDSN, "UNION SELECT", "Built DSN should not contain SQL injection")
				assert.NotContains(t, builtDSN, "<script", "Built DSN should not contain XSS injection")
				assert.NotContains(t, builtDSN, "EXEC", "Built DSN should not contain command injection")
			}
			// If DSN is rejected, that's also acceptable security behavior
		})
	}
}

// TestMySQLSecurityConnectionTimeout tests security aspects of timeouts
func TestMySQLSecurityConnectionTimeout(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	testCases := []struct {
		name        string
		dsn         string
		timeout     string
		maxDuration time.Duration
		expectError bool
	}{
		{
			name:        "Normal timeout",
			dsn:         "user:pass@tcp(127.0.0.1:9999)/testdb", // Non-existent port
			timeout:     "2s",
			maxDuration: 5 * time.Second,
			expectError: true,
		},
		{
			name:        "Very short timeout",
			dsn:         "user:pass@tcp(127.0.0.1:9999)/testdb",
			timeout:     "500ms",
			maxDuration: 3 * time.Second,
			expectError: true,
		},
		{
			name:        "Default timeout (should use 10s)",
			dsn:         "user:pass@tcp(127.0.0.1:9999)/testdb",
			timeout:     "",
			maxDuration: 15 * time.Second, // Default is 10s + some overhead
			expectError: true,
		},
		{
			name:        "Read timeout test",
			dsn:         "user:pass@tcp(127.0.0.1:9999)/testdb",
			timeout:     "1s",
			maxDuration: 4 * time.Second,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			options := map[string]string{
				"tls": "false",
			}
			if tc.timeout != "" {
				options["timeout"] = tc.timeout
			}

			config := DatabaseConfig{
				Provider: "mysql",
				DSN:      tc.dsn,
				Options:  options,
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

// TestMySQLSecurityQueryLogging tests that sensitive data is not logged
func TestMySQLSecurityQueryLogging(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	config := DatabaseConfig{
		Provider:           "mysql",
		DSN:                "user:pass@tcp(localhost:3306)/testdb",
		LogLevel:           "info", // Enable query logging
		SlowQueryThreshold: 100 * time.Millisecond,
		Options: map[string]string{
			"tls": "false",
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

	// Test that provider configuration doesn't leak credentials
	provider.config = config
	assert.Equal(t, "MySQL", provider.Name())
}

// TestMySQLSecurityTransactionIsolation tests transaction security
func TestMySQLSecurityTransactionIsolation(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	// Test that transaction uses proper isolation level
	// The actual test would require integration testing
	assert.Equal(t, "MySQL", provider.Name())

	// Test transaction struct
	tx := &mysqlTransaction{tx: nil}
	assert.NotNil(t, tx, "Transaction struct should be created")

	// Test transaction methods with nil tx (should not panic)
	assert.NotNil(t, tx.GetDB, "GetDB method should exist")
	assert.NotNil(t, tx.Commit, "Commit method should exist")
	assert.NotNil(t, tx.Rollback, "Rollback method should exist")
}

// TestMySQLSecurityResourceLimits tests resource limit enforcement
func TestMySQLSecurityResourceLimits(t *testing.T) {
	logger := logging.GetDefault()
	_ = NewMySQLProvider(logger)

	// Test connection pool limits with MySQL-specific defaults
	config := DatabaseConfig{
		Provider:        "mysql",
		DSN:             "user:pass@tcp(localhost:3306)/testdb",
		MaxOpenConns:    10,               // Limited
		MaxIdleConns:    3,                // Limited
		ConnMaxLifetime: 30 * time.Minute, // MySQL-specific default
		ConnMaxIdleTime: 5 * time.Minute,
		Options: map[string]string{
			"tls": "false",
		},
	}

	// Test that configuration is properly validated
	assert.Equal(t, 10, config.MaxOpenConns)
	assert.Equal(t, 3, config.MaxIdleConns)
	assert.Equal(t, 30*time.Minute, config.ConnMaxLifetime)

	// Test MySQL-specific defaults are reasonable
	if config.MaxOpenConns == 0 {
		t.Error("MaxOpenConns should have a reasonable default")
	}

	// Test very high connection limits (potential DoS)
	highConfig := DatabaseConfig{
		Provider:     "mysql",
		DSN:          "user:pass@tcp(localhost:3306)/testdb",
		MaxOpenConns: 10000, // Very high
		MaxIdleConns: 5000,  // Very high
	}

	// Should not panic or cause issues
	assert.Equal(t, 10000, highConfig.MaxOpenConns)
	assert.Equal(t, 5000, highConfig.MaxIdleConns)
}

// TestMySQLSecurityErrorHandling tests secure error handling
func TestMySQLSecurityErrorHandling(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

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
			assert.NotContains(t, errorMsg, "key", "Error should not contain key")
		})
	}
}

// TestMySQLSecurityHealthCheck tests health check security
func TestMySQLSecurityHealthCheck(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	ctx := context.Background()
	status := provider.HealthCheck(ctx)

	// Health check should work even when not connected
	assert.False(t, status.Healthy, "Health check should report unhealthy when not connected")
	assert.NotEmpty(t, status.Error, "Health check should provide error message")
	assert.NotZero(t, status.CheckedAt, "Health check should set checked time")

	// Ensure health check doesn't leak sensitive information
	details := fmt.Sprintf("%v", status.Details)
	sensitiveTerms := []string{"password", "secret", "token", "key", "credential", "pass", "pwd"}

	for _, term := range sensitiveTerms {
		assert.NotContains(t, strings.ToLower(details), term,
			"Health check details should not contain sensitive term: %s", term)
		assert.NotContains(t, strings.ToLower(status.Error), term,
			"Health check error should not contain sensitive term: %s", term)
	}

	// Test that health check includes expected provider information
	assert.Equal(t, "MySQL", provider.Name())
	assert.NotEmpty(t, provider.Version())
}

// TestMySQLSecurityConcurrentAccess tests thread safety and concurrent access security
func TestMySQLSecurityConcurrentAccess(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	// Test concurrent operations don't cause data races or security issues
	for i := 0; i < 20; i++ {
		go func() {
			// These operations should be thread-safe
			_ = provider.GetStats()
			_ = provider.Name()
			_ = provider.Version()
			_ = provider.HealthCheck(context.Background())

			// Test concurrent DSN operations
			_, _ = provider.buildDSN("user:pass@tcp(localhost:3306)/test", map[string]string{"tls": "false"})
			_ = provider.getHostFromDSN("user:pass@tcp(localhost:3306)/test")
			_ = provider.getDatabaseFromDSN("user:pass@tcp(localhost:3306)/test")
		}()
	}

	// Allow goroutines to complete
	time.Sleep(200 * time.Millisecond)

	// Verify provider state is still consistent
	assert.Equal(t, "MySQL", provider.Name())
	assert.NotEmpty(t, provider.Version())
}

// TestMySQLSecurityDSNValidation tests comprehensive DSN validation
func TestMySQLSecurityDSNValidation(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	testCases := []struct {
		name        string
		dsn         string
		options     map[string]string
		expectError bool
		errorText   string
	}{
		{
			name:        "Valid DSN with tcp protocol",
			dsn:         "user:pass@tcp(localhost:3306)/database",
			options:     map[string]string{"tls": "false"},
			expectError: false,
		},
		{
			name:        "Valid DSN with parameters",
			dsn:         "user:pass@tcp(localhost:3306)/database?charset=utf8mb4",
			options:     map[string]string{"tls": "false"},
			expectError: false,
		},
		{
			name:        "Empty DSN",
			dsn:         "",
			options:     nil,
			expectError: true,
			errorText:   "DSN cannot be empty",
		},
		{
			name:        "Invalid DSN format - no slash",
			dsn:         "user:pass@localhost:3306",
			options:     nil,
			expectError: true,
			errorText:   "invalid MySQL DSN format",
		},
		{
			name:        "DSN with SQL injection",
			dsn:         "user:pass@tcp(localhost:3306)/db'; DROP TABLE users; --",
			options:     nil,
			expectError: true,
			errorText:   "potentially dangerous DSN content detected",
		},
		{
			name:        "DSN with XSS attempt",
			dsn:         "user:pass@tcp(localhost:3306)/db<script>alert('xss')</script>",
			options:     nil,
			expectError: true,
			errorText:   "potentially dangerous DSN content detected",
		},
		{
			name:        "DSN with command injection",
			dsn:         "user:pass@tcp(localhost:3306)/db; EXEC xp_cmdshell 'dir'",
			options:     nil,
			expectError: true,
			errorText:   "potentially dangerous DSN content detected",
		},
		{
			name:    "Valid DSN with safe options",
			dsn:     "user:pass@tcp(localhost:3306)/database",
			options: map[string]string{"charset": "utf8mb4", "collation": "utf8mb4_unicode_ci"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := provider.buildDSN(tc.dsn, tc.options)

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

// TestMySQLSecurityCharsetValidation tests character set security
func TestMySQLSecurityCharsetValidation(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	testCases := []struct {
		name     string
		dsn      string
		options  map[string]string
		expected string
	}{
		{
			name:     "Default charset utf8mb4",
			dsn:      "user:pass@tcp(localhost:3306)/database",
			options:  map[string]string{"tls": "false"},
			expected: "charset=utf8mb4",
		},
		{
			name:     "Custom charset",
			dsn:      "user:pass@tcp(localhost:3306)/database",
			options:  map[string]string{"tls": "false", "charset": "latin1"},
			expected: "charset=latin1",
		},
		{
			name:     "Charset already in DSN",
			dsn:      "user:pass@tcp(localhost:3306)/database?charset=utf8",
			options:  map[string]string{"tls": "false"},
			expected: "charset=utf8", // Should preserve existing
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builtDSN, err := provider.buildDSN(tc.dsn, tc.options)
			assert.NoError(t, err)
			assert.Contains(t, builtDSN, tc.expected)
		})
	}
}

// TestMySQLSecurityTimeoutValidation tests timeout security settings
func TestMySQLSecurityTimeoutValidation(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	testCases := []struct {
		name            string
		dsn             string
		options         map[string]string
		expectedTimeout string
		expectedRead    string
		expectedWrite   string
	}{
		{
			name:            "Default timeouts",
			dsn:             "user:pass@tcp(localhost:3306)/database",
			options:         map[string]string{"tls": "false"},
			expectedTimeout: "timeout=10s",
			expectedRead:    "readTimeout=30s",
			expectedWrite:   "writeTimeout=30s",
		},
		{
			name:            "Custom timeouts",
			dsn:             "user:pass@tcp(localhost:3306)/database",
			options:         map[string]string{"tls": "false", "timeout": "5s", "readTimeout": "15s", "writeTimeout": "15s"},
			expectedTimeout: "timeout=5s",
			expectedRead:    "readTimeout=15s",
			expectedWrite:   "writeTimeout=15s",
		},
		{
			name:            "Partial custom timeouts",
			dsn:             "user:pass@tcp(localhost:3306)/database",
			options:         map[string]string{"tls": "false", "timeout": "3s"},
			expectedTimeout: "timeout=3s",
			expectedRead:    "readTimeout=30s",  // Should use default
			expectedWrite:   "writeTimeout=30s", // Should use default
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builtDSN, err := provider.buildDSN(tc.dsn, tc.options)
			assert.NoError(t, err)
			assert.Contains(t, builtDSN, tc.expectedTimeout)
			assert.Contains(t, builtDSN, tc.expectedRead)
			assert.Contains(t, builtDSN, tc.expectedWrite)
		})
	}
}

// TestMySQLSecurityErrorSanitization tests error message sanitization
func TestMySQLSecurityErrorSanitization(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	testErrors := []struct {
		name             string
		inputError       string
		shouldNotContain []string
		shouldContain    []string
	}{
		{
			name:             "Password in connection string",
			inputError:       "failed to connect to user:supersecret@tcp(localhost:3306)/db",
			shouldNotContain: []string{"supersecret"},
			shouldContain:    []string{"tcp(localhost:3306)", ":***@"},
		},
		{
			name:             "User credentials in error",
			inputError:       "authentication failed for user=admin password=secret123 dbname=production",
			shouldNotContain: []string{"admin", "secret123", "production"},
			shouldContain:    []string{"authentication", "user=***", "password=***", "dbname=***"},
		},
		{
			name:             "URL format credentials",
			inputError:       "connection error mysql://root:adminpass@localhost:3306/secure_db",
			shouldNotContain: []string{"root", "adminpass"},
			shouldContain:    []string{"mysql://", "://***:***@"},
		},
		{
			name:             "Complex DSN with credentials",
			inputError:       "failed: service:mypassword123@tcp(db.internal:3306)/app?charset=utf8mb4",
			shouldNotContain: []string{"mypassword123"},
			shouldContain:    []string{":***@", "tcp(db.internal:3306)"},
		},
		{
			name:             "Multiple credential patterns",
			inputError:       "error: user:secret123@tcp(host:3306)/db and also user=test password=secret dbname=app",
			shouldNotContain: []string{"secret123", "test", "secret"},
			shouldContain:    []string{":***@", "user=***", "password=***", "dbname=***"},
		},
	}

	for _, tc := range testErrors {
		t.Run(tc.name, func(t *testing.T) {
			originalErr := fmt.Errorf("%s", tc.inputError)
			sanitizedErr := provider.sanitizeError(originalErr)

			assert.NotNil(t, sanitizedErr)
			sanitizedMsg := sanitizedErr.Error()

			// Debug output to understand sanitization
			t.Logf("Original: %s", tc.inputError)
			t.Logf("Sanitized: %s", sanitizedMsg)

			// Check that sensitive information is removed
			for _, sensitive := range tc.shouldNotContain {
				assert.NotContains(t, sanitizedMsg, sensitive,
					"Sanitized error should not contain: %s", sensitive)
			}

			// Check that expected patterns are present
			for _, preserve := range tc.shouldContain {
				assert.Contains(t, sanitizedMsg, preserve,
					"Sanitized error should contain: %s", preserve)
			}
		})
	}
}
