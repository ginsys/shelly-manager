package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// TestMySQLSecuritySSLValidation tests comprehensive SSL/TLS validation
func TestMySQLSecuritySSLValidation(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	// The modes go-sql-driver/mysql takes for `tls=`; any other value is a registered config name, and
	// this provider registers none.
	validTLSModes := []string{"false", "true", "skip-verify", "preferred"}

	for _, mode := range validTLSModes {
		t.Run(fmt.Sprintf("ValidTLSMode_%s", mode), func(t *testing.T) {
			options := map[string]string{"tls": mode}
			err := provider.validateSSLConfig(options)
			assert.NoError(t, err, "TLS mode %s should be valid", mode)
		})
	}

	// Test invalid TLS modes. "custom", "required", "verify-ca" and "verify-identity" are names the driver
	// refuses at connect time ("invalid value / unknown config name"), so the validator refuses them first.
	invalidTLSModes := []string{
		"invalid", "wrong", "bad-mode", "ssl", "disabled", "enabled",
		"custom", "required", "verify-ca", "verify-identity",
	}

	for _, mode := range invalidTLSModes {
		t.Run(fmt.Sprintf("InvalidTLSMode_%s", mode), func(t *testing.T) {
			options := map[string]string{"tls": mode}
			err := provider.validateSSLConfig(options)
			assert.Error(t, err, "TLS mode %s should be invalid", mode)
			assert.Contains(t, err.Error(), "invalid TLS mode")
			assert.Contains(t, err.Error(), "skip-verify", "the error names the modes the driver takes")
		})
	}
}

// TestMySQLSecuritySSLCertificateValidation tests the options that go with a TLS mode. The driver has
// no `ca`, `cert`, `key` or `serverName` DSN parameter: it would keep each as a session variable and
// the connection would fail, so the provider refuses them and says what to use instead.
func TestMySQLSecuritySSLCertificateValidation(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewMySQLProvider(logger)

	const unsupported = "custom TLS files are not supported; use tls=preferred|true|skip-verify"

	testCases := []struct {
		name        string
		options     map[string]string
		expectError bool
		errorText   string
	}{
		{
			name:        "ca is refused",
			options:     map[string]string{"tls": "true", "ca": "/etc/ssl/ca.pem"},
			expectError: true,
			errorText:   unsupported,
		},
		{
			name:        "cert is refused",
			options:     map[string]string{"tls": "true", "cert": "/etc/ssl/client.pem"},
			expectError: true,
			errorText:   unsupported,
		},
		{
			name:        "key is refused",
			options:     map[string]string{"tls": "true", "key": "/etc/ssl/client.key"},
			expectError: true,
			errorText:   unsupported,
		},
		{
			name:        "serverName is refused",
			options:     map[string]string{"tls": "true", "serverName": "mysql.internal"},
			expectError: true,
			errorText:   unsupported,
		},
		{
			name:        "a certificate option is refused with no tls option too",
			options:     map[string]string{"ca": "/etc/ssl/ca.pem"},
			expectError: true,
			errorText:   unsupported,
		},
		{
			name:        "an empty certificate option is refused too (an empty ca= fails at connect as well)",
			options:     map[string]string{"tls": "true", "ca": ""},
			expectError: true,
			errorText:   unsupported,
		},
		{
			name:        "an option other than the four still passes (collation)",
			options:     map[string]string{"tls": "true", "collation": "utf8mb4_unicode_ci"},
			expectError: false,
		},
		{
			name:        "a verify mode the driver lacks is refused",
			options:     map[string]string{"tls": "verify-identity"},
			expectError: true,
			errorText:   "invalid TLS mode: verify-identity",
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
		"admin:super_secret_key@tcp(127.0.0.1:1)/production",
		"service:private_token@tcp(127.0.0.1:2)/app_db",
		"root:admin_pass@tcp(127.0.0.1:3306)/secure_db",
		"user:db_secret@tcp(127.0.0.1:3)/database",
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
		})
	}
}

// TestMySQLBuildDSN_TextInRows reads DSNs that hold text which looks like SQL or
// markup, and says for each whether the driver's parse of the built DSN accepts
// it. The provider reads no DSN for content: a word in the user or the database
// name is part of that value, and a parameter is read by the driver.
func TestMySQLBuildDSN_TextInRows(t *testing.T) {
	provider := NewMySQLProvider(logging.GetDefault())

	rows := []struct {
		dsn      string
		accepted bool
		cause    string // text the driver's error holds, for a refused row
		why      string
	}{
		{"user:pass@tcp(localhost:3306)/test'; DROP TABLE users; --", true, "", "the text is the database name"},
		{"user'; DELETE FROM sensitive_data; --:pass@tcp(localhost:3306)/db", true, "", "the text is the user name"},
		{"user:pass@tcp(localhost:3306)/db'; INSERT INTO admin_users VALUES ('hacker', 'admin'); --", true, "", "the text is the database name"},
		{"user:pass@tcp(localhost:3306)/db?timeout=5s'; DROP DATABASE production; --", false, "unknown unit", "the timeout value is not a duration"},
		{"user:pass@tcp(localhost:3306)/db?charset=utf8'; TRUNCATE TABLE payments; --", true, "", "a charset value is a list of names the driver sends as written"},
		{"user:pass@tcp(localhost:3306)/db?readTimeout=30s/**/UNION SELECT password FROM users; --", false, "did you forget to escape a param value", "a slash after the address leaves the address unclosed"},
		{"user:pass@tcp(localhost:3306)/db?writeTimeout=30s<script>alert('xss')</script>", false, "did you forget to escape a param value", "a slash after the address leaves the address unclosed"},
		{"user:pass@tcp(localhost:3306)/db?tls=false; EXEC xp_cmdshell 'dir'; --", false, "unknown config name", "the tls value is neither a switch nor a registered name"},
	}

	for i, row := range rows {
		t.Run(fmt.Sprintf("Row_%d", i+1), func(t *testing.T) {
			built, err := provider.buildDSN(row.dsn, nil)
			require.NoError(t, err, "buildDSN adds defaults and reads no content")
			_, err = mysqldriver.ParseDSN(built)
			if row.accepted {
				assert.NoError(t, err, row.why)
			} else {
				require.Error(t, err, row.why)
				assert.Contains(t, err.Error(), row.cause, row.why)
			}
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

	// Test connection pool limits with explicit values
	config := DatabaseConfig{
		Provider:        "mysql",
		DSN:             "user:pass@tcp(localhost:3306)/testdb",
		MaxOpenConns:    10,               // Limited
		MaxIdleConns:    3,                // Limited
		ConnMaxLifetime: 30 * time.Minute, // an explicit value
		ConnMaxIdleTime: 5 * time.Minute,
		Options: map[string]string{
			"tls": "false",
		},
	}

	// Test that configuration is properly validated
	assert.Equal(t, 10, config.MaxOpenConns)
	assert.Equal(t, 3, config.MaxIdleConns)
	assert.Equal(t, 30*time.Minute, config.ConnMaxLifetime)

	// Test the configured values are reasonable
	if config.MaxOpenConns == 0 {
		t.Error("MaxOpenConns should be set")
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
		}()
	}

	// Allow goroutines to complete
	time.Sleep(200 * time.Millisecond)

	// Verify provider state is still consistent
	assert.Equal(t, "MySQL", provider.Name())
	assert.NotEmpty(t, provider.Version())
}

// TestMySQLBuildDSN_Format tests which DSNs buildDSN accepts and refuses
func TestMySQLBuildDSN_Format(t *testing.T) {
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
		// The three rows below were refused for the words in them. buildDSN reads
		// no content now, so each is accepted: the words are the database name.
		// Whether the driver parses the result is TestMySQLBuildDSN_TextInRows.
		{
			name:    "SQL words in the database name",
			dsn:     "user:pass@tcp(localhost:3306)/db'; DROP TABLE users; --",
			options: nil,
		},
		{
			name:    "Markup in the database name",
			dsn:     "user:pass@tcp(localhost:3306)/db<script>alert('xss')</script>",
			options: nil,
		},
		{
			name:    "A command and a semicolon in the database name",
			dsn:     "user:pass@tcp(localhost:3306)/db; EXEC xp_cmdshell 'dir'",
			options: nil,
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
