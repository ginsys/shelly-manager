package provider

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// MySQLProvider implements DatabaseProvider for MySQL databases
type MySQLProvider struct {
	db     *gorm.DB
	config DatabaseConfig
	logger *logging.Logger

	// Statistics tracking
	stats         DatabaseStats
	statsMu       sync.RWMutex
	queryCount    int64
	slowQueries   int64
	failedQueries int64
	totalLatency  int64

	// Connection management
	connected bool
	connMu    sync.RWMutex
}

// NewMySQLProvider creates a new MySQL database provider
func NewMySQLProvider(logger *logging.Logger) *MySQLProvider {
	if logger == nil {
		logger = logging.GetDefault()
	}

	return &MySQLProvider{
		logger: logger,
		stats: DatabaseStats{
			ProviderName:    "MySQL",
			ProviderVersion: "Unknown", // Will be updated after connection
			Metadata:        make(map[string]interface{}),
		},
	}
}

// Connect establishes connection to the MySQL database
func (m *MySQLProvider) Connect(config DatabaseConfig) error {
	m.connMu.Lock()
	defer m.connMu.Unlock()

	if m.connected {
		return fmt.Errorf("already connected to database")
	}

	m.config = config

	// Parse and validate DSN
	dsn, err := m.buildDSN(config.DSN, config.Options)
	if err != nil {
		return fmt.Errorf("failed to build DSN: %w", err)
	}

	// Configure GORM logger based on config
	gormConfig := &gorm.Config{
		Logger: m.createGormLogger(),
	}

	// Open database connection with MySQL driver
	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL database: %w", m.sanitizeError(err))
	}

	m.db = db

	// Configure connection pool settings
	if err := m.configureConnectionPool(); err != nil {
		m.db = nil
		return fmt.Errorf("failed to configure connection pool: %w", m.sanitizeError(err))
	}

	// Test the connection
	if err := m.Ping(); err != nil {
		m.db = nil
		return fmt.Errorf("failed to ping database: %w", m.sanitizeError(err))
	}

	// Get MySQL version
	if err := m.updateProviderVersion(); err != nil {
		m.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Warn("Failed to get MySQL version")
	}

	m.connected = true
	m.logger.WithFields(map[string]any{
		"provider": "mysql",
		"host":     m.getHostFromDSN(dsn),
		"database": m.getDatabaseFromDSN(dsn),
	}).Info("Connected to MySQL database")

	return nil
}

// Close closes the database connection
func (m *MySQLProvider) Close() error {
	m.connMu.Lock()
	defer m.connMu.Unlock()

	if !m.connected || m.db == nil {
		return nil
	}

	sqlDB, err := m.db.DB()
	if err == nil {
		err = sqlDB.Close()
	}

	m.db = nil
	m.connected = false

	if err != nil {
		return fmt.Errorf("failed to close database: %w", m.sanitizeError(err))
	}

	m.logger.Info("Closed MySQL database connection")
	return nil
}

// Ping checks if the database connection is alive
func (m *MySQLProvider) Ping() error {
	if !m.connected || m.db == nil {
		return fmt.Errorf("not connected to database")
	}

	sqlDB, err := m.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying SQL DB: %w", m.sanitizeError(err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return sqlDB.PingContext(ctx)
}

// Migrate performs database migration
func (m *MySQLProvider) Migrate(models ...interface{}) error {
	if !m.connected || m.db == nil {
		return fmt.Errorf("not connected to database")
	}

	start := time.Now()
	err := m.db.AutoMigrate(models...)
	duration := time.Since(start)

	if err != nil {
		m.logger.WithFields(map[string]any{
			"error":    err.Error(),
			"duration": duration,
			"models":   len(models),
		}).Error("Database migration failed")
		atomic.AddInt64(&m.failedQueries, 1)
		return fmt.Errorf("migration failed: %w", err)
	}

	m.logger.WithFields(map[string]any{
		"duration": duration,
		"models":   len(models),
	}).Info("Database migration completed successfully")

	return nil
}

// DropTables drops the specified tables
func (m *MySQLProvider) DropTables(models ...interface{}) error {
	if !m.connected || m.db == nil {
		return fmt.Errorf("not connected to database")
	}

	return m.db.Migrator().DropTable(models...)
}

// BeginTransaction starts a new database transaction
func (m *MySQLProvider) BeginTransaction() (Transaction, error) {
	if !m.connected || m.db == nil {
		return nil, fmt.Errorf("not connected to database")
	}

	tx := m.db.Begin(&sql.TxOptions{
		Isolation: sql.LevelReadCommitted, // MySQL default
	})
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", m.sanitizeError(tx.Error))
	}

	return &mysqlTransaction{tx: tx}, nil
}

// GetDB returns the underlying GORM database instance
func (m *MySQLProvider) GetDB() *gorm.DB {
	return m.db
}

// GetStats returns database statistics
func (m *MySQLProvider) GetStats() DatabaseStats {
	m.statsMu.RLock()
	defer m.statsMu.RUnlock()

	// Update runtime statistics
	m.updateStats()

	stats := m.stats
	stats.TotalQueries = atomic.LoadInt64(&m.queryCount)
	stats.SlowQueries = atomic.LoadInt64(&m.slowQueries)
	stats.FailedQueries = atomic.LoadInt64(&m.failedQueries)

	if stats.TotalQueries > 0 {
		stats.AverageLatency = time.Duration(atomic.LoadInt64(&m.totalLatency) / stats.TotalQueries)
	}

	return stats
}

// SetLogger sets the logger for the provider
func (m *MySQLProvider) SetLogger(logger *logging.Logger) {
	m.logger = logger
}

// Name returns the provider name
func (m *MySQLProvider) Name() string {
	return "MySQL"
}

// Version returns the provider version
func (m *MySQLProvider) Version() string {
	return m.stats.ProviderVersion
}

// HealthCheck implements HealthChecker interface
func (m *MySQLProvider) HealthCheck(ctx context.Context) HealthStatus {
	status := HealthStatus{
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	start := time.Now()

	if err := m.Ping(); err != nil {
		status.Healthy = false
		status.Error = err.Error()
		status.ResponseTime = time.Since(start)
		return status
	}

	status.Healthy = true
	status.ResponseTime = time.Since(start)

	// Add health details
	stats := m.GetStats()
	status.Details["database_size"] = stats.DatabaseSize
	status.Details["total_queries"] = stats.TotalQueries
	status.Details["connection_count"] = stats.OpenConnections
	status.Details["version"] = m.Version()

	return status
}

// sanitizeError removes sensitive information from error messages
func (m *MySQLProvider) sanitizeError(err error) error {
	if err == nil {
		return nil
	}

	errorMsg := err.Error()

	// Patterns to match and remove credentials from MySQL DSN-like strings
	credentialPatterns := []*regexp.Regexp{
		regexp.MustCompile(`:[^:@/]+@`),        // Remove password in user:password@host format
		regexp.MustCompile(`user=\w+`),         // Remove user parameter
		regexp.MustCompile(`password=[^\s&]+`), // Remove password parameter
		regexp.MustCompile(`dbname=\w+`),       // Remove database name
	}

	// Replace credentials with safe placeholders
	sanitizedMsg := errorMsg
	for _, pattern := range credentialPatterns {
		switch pattern.String() {
		case `:[^:@/]+@`:
			sanitizedMsg = pattern.ReplaceAllString(sanitizedMsg, ":***@")
		default:
			matches := pattern.FindAllString(sanitizedMsg, -1)
			for _, match := range matches {
				parts := strings.Split(match, "=")
				if len(parts) == 2 {
					sanitizedMsg = strings.ReplaceAll(sanitizedMsg, match, parts[0]+"=***")
				}
			}
		}
	}

	// Remove any remaining potential credential information in URLs
	urlPattern := regexp.MustCompile(`://[^:]+:[^@]+@`)
	sanitizedMsg = urlPattern.ReplaceAllString(sanitizedMsg, "://***:***@")

	return fmt.Errorf("%s", sanitizedMsg)
}

// validateDSNInput validates and sanitizes DSN components for security
func (m *MySQLProvider) validateDSNInput(dsn string) error {
	// Check for SQL injection patterns
	dangerousPatterns := []string{
		"DROP TABLE", "DROP DATABASE", "DELETE FROM", "INSERT INTO",
		"UPDATE SET", "CREATE TABLE", "ALTER TABLE", "TRUNCATE",
		"--", "/*", "*/", ";", "EXEC", "EXECUTE",
		"sp_", "xp_", "UNION SELECT", "INFORMATION_SCHEMA",
		"<script", "javascript:", "onload=", "onerror=",
	}

	upperDSN := strings.ToUpper(dsn)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(upperDSN, strings.ToUpper(pattern)) {
			return fmt.Errorf("potentially dangerous DSN content detected: contains pattern similar to %s", pattern)
		}
	}

	return nil
}

// buildDSN constructs a MySQL DSN with SSL/TLS settings and security validation
func (m *MySQLProvider) buildDSN(baseDSN string, options map[string]string) (string, error) {
	// Validate DSN format
	if baseDSN == "" {
		return "", fmt.Errorf("DSN cannot be empty")
	}

	// Security validation: Check for dangerous patterns
	if err := m.validateDSNInput(baseDSN); err != nil {
		return "", fmt.Errorf("DSN security validation failed: %w", err)
	}

	// Parse DSN to validate format
	// MySQL DSN format: [user[:password]@][net[(addr)]]/dbname[?param1=value1&...&paramN=valueN]
	// or tcp(host:port)/database?param=value
	if !strings.Contains(baseDSN, "/") {
		return "", fmt.Errorf("invalid MySQL DSN format, expected format like user:pass@tcp(host:port)/database")
	}

	// Validate SSL configuration if provided
	if err := m.validateSSLConfig(options); err != nil {
		return "", fmt.Errorf("SSL validation failed: %w", err)
	}

	// Start with the base DSN
	dsn := baseDSN

	// Add SSL mode if not specified (default to preferred for security)
	if !strings.Contains(dsn, "tls=") {
		tlsMode := "preferred" // Default to preferred SSL
		if mode, ok := options["tls"]; ok {
			tlsMode = mode
		}

		if strings.Contains(dsn, "?") {
			dsn += "&tls=" + tlsMode
		} else {
			dsn += "?tls=" + tlsMode
		}
	}

	// Add connection timeout if not specified
	if !strings.Contains(dsn, "timeout=") {
		timeout := "10s" // 10 seconds default
		if t, ok := options["timeout"]; ok {
			timeout = t
		}

		if strings.Contains(dsn, "?") {
			dsn += "&timeout=" + timeout
		} else {
			dsn += "?timeout=" + timeout
		}
	}

	// Add read timeout if not specified
	if !strings.Contains(dsn, "readTimeout=") {
		readTimeout := "30s" // 30 seconds default
		if t, ok := options["readTimeout"]; ok {
			readTimeout = t
		}

		if strings.Contains(dsn, "?") {
			dsn += "&readTimeout=" + readTimeout
		} else {
			dsn += "?readTimeout=" + readTimeout
		}
	}

	// Add write timeout if not specified
	if !strings.Contains(dsn, "writeTimeout=") {
		writeTimeout := "30s" // 30 seconds default
		if t, ok := options["writeTimeout"]; ok {
			writeTimeout = t
		}

		if strings.Contains(dsn, "?") {
			dsn += "&writeTimeout=" + writeTimeout
		} else {
			dsn += "?writeTimeout=" + writeTimeout
		}
	}

	// Set character set to utf8mb4 if not specified (for full Unicode support)
	if !strings.Contains(dsn, "charset=") {
		charset := "utf8mb4"
		if c, ok := options["charset"]; ok {
			charset = c
		}

		if strings.Contains(dsn, "?") {
			dsn += "&charset=" + charset
		} else {
			dsn += "?charset=" + charset
		}
	}

	// Add other MySQL-specific options with validation
	for key, value := range options {
		// Security validation: Check for dangerous patterns in options
		if err := m.validateDSNInput(key + "=" + value); err != nil {
			continue // Skip dangerous options silently to prevent information disclosure
		}

		switch key {
		case "ca", "cert", "key", "serverName", "collation", "loc", "maxAllowedPacket":
			if !strings.Contains(dsn, key+"=") {
				if strings.Contains(dsn, "?") {
					dsn += "&" + key + "=" + url.QueryEscape(value)
				} else {
					dsn += "?" + key + "=" + url.QueryEscape(value)
				}
			}
		}
	}

	return dsn, nil
}

// configureConnectionPool sets up MySQL connection pool parameters
func (m *MySQLProvider) configureConnectionPool() error {
	sqlDB, err := m.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying SQL DB: %w", m.sanitizeError(err))
	}

	// Set connection pool parameters with MySQL-appropriate defaults
	maxOpenConns := m.config.MaxOpenConns
	if maxOpenConns == 0 {
		maxOpenConns = 20 // MySQL default conservative setting
	}
	sqlDB.SetMaxOpenConns(maxOpenConns)

	maxIdleConns := m.config.MaxIdleConns
	if maxIdleConns == 0 {
		maxIdleConns = 5
	}
	sqlDB.SetMaxIdleConns(maxIdleConns)

	connMaxLifetime := m.config.ConnMaxLifetime
	if connMaxLifetime == 0 {
		connMaxLifetime = 30 * time.Minute // MySQL connections should be rotated more frequently
	}
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	connMaxIdleTime := m.config.ConnMaxIdleTime
	if connMaxIdleTime == 0 {
		connMaxIdleTime = 5 * time.Minute
	}
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)

	m.logger.WithFields(map[string]any{
		"max_open_conns":     maxOpenConns,
		"max_idle_conns":     maxIdleConns,
		"conn_max_lifetime":  connMaxLifetime,
		"conn_max_idle_time": connMaxIdleTime,
	}).Debug("Configured MySQL connection pool")

	return nil
}

// updateProviderVersion retrieves and stores the MySQL version
func (m *MySQLProvider) updateProviderVersion() error {
	var version string
	if err := m.db.Raw("SELECT VERSION()").Scan(&version).Error; err != nil {
		return err
	}

	// Extract version number from version string
	if version != "" {
		parts := strings.Fields(version)
		if len(parts) >= 1 {
			m.statsMu.Lock()
			m.stats.ProviderVersion = parts[0]
			m.stats.Metadata["full_version"] = version
			m.statsMu.Unlock()
		}
	}

	return nil
}

// createGormLogger creates a GORM logger instance based on configuration
func (m *MySQLProvider) createGormLogger() logger.Interface {
	var logLevel logger.LogLevel

	switch m.config.LogLevel {
	case "silent":
		logLevel = logger.Silent
	case "error":
		logLevel = logger.Error
	case "warn":
		logLevel = logger.Warn
	case "info":
		logLevel = logger.Info
	default:
		logLevel = logger.Warn
	}

	return logger.New(
		log.New(&gormLogWriter{logger: m.logger}, "", 0),
		logger.Config{
			SlowThreshold:             m.config.SlowQueryThreshold,
			LogLevel:                  logLevel,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)
}

// updateStats updates runtime database statistics
func (m *MySQLProvider) updateStats() {
	if !m.connected || m.db == nil {
		return
	}

	sqlDB, err := m.db.DB()
	if err != nil {
		return
	}

	stats := sqlDB.Stats()
	m.stats.OpenConnections = stats.OpenConnections
	m.stats.InUseConnections = stats.InUse
	m.stats.IdleConnections = stats.Idle

	// Get database size (MySQL specific query)
	var dbSize int64
	if err := m.db.Raw("SELECT SUM(data_length + index_length) AS size FROM information_schema.tables WHERE table_schema = DATABASE()").Scan(&dbSize).Error; err == nil {
		m.stats.DatabaseSize = dbSize
	}
}

// getHostFromDSN extracts host from MySQL DSN for logging
func (m *MySQLProvider) getHostFromDSN(dsn string) string {
	// MySQL DSN format: [user[:password]@][tcp[(addr)]]/dbname
	if strings.Contains(dsn, "tcp(") {
		start := strings.Index(dsn, "tcp(") + 4
		end := strings.Index(dsn[start:], ")")
		if end > 0 {
			return dsn[start : start+end]
		}
	}
	// Extract from simple user@host format
	if strings.Contains(dsn, "@") && strings.Contains(dsn, "/") {
		start := strings.LastIndex(dsn, "@") + 1
		end := strings.Index(dsn[start:], "/")
		if end > 0 {
			return dsn[start : start+end]
		}
	}
	return "unknown"
}

// getDatabaseFromDSN extracts database name from MySQL DSN for logging
func (m *MySQLProvider) getDatabaseFromDSN(dsn string) string {
	// Find the last / and extract database name
	lastSlash := strings.LastIndex(dsn, "/")
	if lastSlash >= 0 {
		dbPart := dsn[lastSlash+1:]
		// Remove query parameters if present
		if questionMark := strings.Index(dbPart, "?"); questionMark >= 0 {
			dbPart = dbPart[:questionMark]
		}
		if dbPart != "" {
			return dbPart
		}
	}
	return "unknown"
}

// validateSSLConfig validates SSL certificate configuration for MySQL
func (m *MySQLProvider) validateSSLConfig(options map[string]string) error {
	tlsMode, hasTlsMode := options["tls"]
	if !hasTlsMode {
		return nil // Default TLS mode will be applied
	}

	// Validate TLS mode
	validTLSModes := map[string]bool{
		"false":           true, // Disable SSL
		"true":            true, // Enable SSL without verification
		"skip-verify":     true, // Enable SSL but skip certificate verification
		"preferred":       true, // Prefer SSL, fallback to non-SSL
		"custom":          true, // Use custom TLS config
		"required":        true, // Require SSL connection
		"verify-ca":       true, // Verify CA certificate
		"verify-identity": true, // Verify CA certificate and server hostname
	}

	if !validTLSModes[tlsMode] {
		return fmt.Errorf("invalid TLS mode: %s", tlsMode)
	}

	// If using custom TLS or verification modes, ensure certificate files exist
	if tlsMode == "custom" || tlsMode == "verify-ca" || tlsMode == "verify-identity" {
		if ca, ok := options["ca"]; ok && ca != "" {
			if _, err := os.Stat(ca); os.IsNotExist(err) {
				return fmt.Errorf("CA certificate not found: %s", ca)
			}
		}

		if cert, ok := options["cert"]; ok && cert != "" {
			if _, err := os.Stat(cert); os.IsNotExist(err) {
				return fmt.Errorf("client certificate not found: %s", cert)
			}
		}

		if key, ok := options["key"]; ok && key != "" {
			if _, err := os.Stat(key); os.IsNotExist(err) {
				return fmt.Errorf("client key not found: %s", key)
			}
		}
	}

	return nil
}

// mysqlTransaction implements the Transaction interface for MySQL
type mysqlTransaction struct {
	tx *gorm.DB
}

func (t *mysqlTransaction) GetDB() *gorm.DB {
	return t.tx
}

func (t *mysqlTransaction) Commit() error {
	return t.tx.Commit().Error
}

func (t *mysqlTransaction) Rollback() error {
	return t.tx.Rollback().Error
}
