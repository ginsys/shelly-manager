package provider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// PostgreSQLProvider implements DatabaseProvider for PostgreSQL databases
type PostgreSQLProvider struct {
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

// NewPostgreSQLProvider creates a new PostgreSQL database provider
func NewPostgreSQLProvider(logger *logging.Logger) *PostgreSQLProvider {
	if logger == nil {
		logger = logging.GetDefault()
	}

	return &PostgreSQLProvider{
		logger: logger,
		stats: DatabaseStats{
			ProviderName:    "PostgreSQL",
			ProviderVersion: "Unknown", // Will be updated after connection
			Metadata:        make(map[string]interface{}),
		},
	}
}

// Connect establishes connection to the PostgreSQL database
func (p *PostgreSQLProvider) Connect(config DatabaseConfig) error {
	p.connMu.Lock()
	defer p.connMu.Unlock()

	if p.connected {
		return fmt.Errorf("already connected to database")
	}

	p.config = config

	// Parse and validate DSN
	dsn, err := p.buildDSN(config.DSN, config.Options)
	if err != nil {
		return fmt.Errorf("failed to build DSN: %w", p.sanitizeError(err))
	}

	// Configure GORM logger based on config
	gormConfig := &gorm.Config{
		Logger: p.createGormLogger(),
	}

	// Open database connection with PostgreSQL driver
	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL database: %w", p.sanitizeError(err))
	}

	p.db = db

	// Configure connection pool settings
	if err := p.configureConnectionPool(); err != nil {
		p.db = nil
		return fmt.Errorf("failed to configure connection pool: %w", p.sanitizeError(err))
	}

	// Mark the handle live before probing it: Ping (and updateProviderVersion)
	// refuse to run while connected is false, so pinging first made Connect
	// fail unconditionally with "not connected to database". Rolled back below
	// if the probe fails.
	p.connected = true

	// Test the connection
	if err := p.Ping(); err != nil {
		p.connected = false
		p.db = nil
		return fmt.Errorf("failed to ping database: %w", p.sanitizeError(err))
	}

	// Get PostgreSQL version
	if err := p.updateProviderVersion(); err != nil {
		p.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Warn("Failed to get PostgreSQL version")
	}

	p.logger.WithFields(map[string]any{
		"provider": "postgresql",
		"host":     p.getHostFromDSN(dsn),
		"database": p.getDatabaseFromDSN(dsn),
	}).Info("Connected to PostgreSQL database")

	return nil
}

// Close closes the database connection
func (p *PostgreSQLProvider) Close() error {
	p.connMu.Lock()
	defer p.connMu.Unlock()

	if !p.connected || p.db == nil {
		return nil
	}

	sqlDB, err := p.db.DB()
	if err == nil {
		err = sqlDB.Close()
	}

	p.db = nil
	p.connected = false

	if err != nil {
		return fmt.Errorf("failed to close database: %w", p.sanitizeError(err))
	}

	p.logger.Info("Closed PostgreSQL database connection")
	return nil
}

// Ping checks if the database connection is alive
func (p *PostgreSQLProvider) Ping() error {
	if !p.connected || p.db == nil {
		return fmt.Errorf("not connected to database")
	}

	sqlDB, err := p.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying SQL DB: %w", p.sanitizeError(err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return sqlDB.PingContext(ctx)
}

// Migrate performs database migration
func (p *PostgreSQLProvider) Migrate(models ...interface{}) error {
	if !p.connected || p.db == nil {
		return fmt.Errorf("not connected to database")
	}

	start := time.Now()
	err := p.db.AutoMigrate(models...)
	duration := time.Since(start)

	if err != nil {
		p.logger.WithFields(map[string]any{
			"error":    err.Error(),
			"duration": duration,
			"models":   len(models),
		}).Error("Database migration failed")
		atomic.AddInt64(&p.failedQueries, 1)
		return fmt.Errorf("migration failed: %w", err)
	}

	p.logger.WithFields(map[string]any{
		"duration": duration,
		"models":   len(models),
	}).Info("Database migration completed successfully")

	return nil
}

// DropTables drops the specified tables
func (p *PostgreSQLProvider) DropTables(models ...interface{}) error {
	if !p.connected || p.db == nil {
		return fmt.Errorf("not connected to database")
	}

	return p.db.Migrator().DropTable(models...)
}

// BeginTransaction starts a new database transaction
func (p *PostgreSQLProvider) BeginTransaction() (Transaction, error) {
	if !p.connected || p.db == nil {
		return nil, fmt.Errorf("not connected to database")
	}

	tx := p.db.Begin(&sql.TxOptions{
		Isolation: sql.LevelReadCommitted, // PostgreSQL default
	})
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", p.sanitizeError(tx.Error))
	}

	return &postgresTransaction{tx: tx}, nil
}

// GetDB returns the underlying GORM database instance
func (p *PostgreSQLProvider) GetDB() *gorm.DB {
	return p.db
}

// GetStats returns database statistics
func (p *PostgreSQLProvider) GetStats() DatabaseStats {
	p.statsMu.RLock()
	defer p.statsMu.RUnlock()

	// Update runtime statistics
	p.updateStats()

	stats := p.stats
	stats.TotalQueries = atomic.LoadInt64(&p.queryCount)
	stats.SlowQueries = atomic.LoadInt64(&p.slowQueries)
	stats.FailedQueries = atomic.LoadInt64(&p.failedQueries)

	if stats.TotalQueries > 0 {
		stats.AverageLatency = time.Duration(atomic.LoadInt64(&p.totalLatency) / stats.TotalQueries)
	}

	return stats
}

// SetLogger sets the logger for the provider
func (p *PostgreSQLProvider) SetLogger(logger *logging.Logger) {
	p.logger = logger
}

// Name returns the provider name
func (p *PostgreSQLProvider) Name() string {
	return "PostgreSQL"
}

// Version returns the provider version
func (p *PostgreSQLProvider) Version() string {
	return p.stats.ProviderVersion
}

// HealthCheck implements HealthChecker interface
func (p *PostgreSQLProvider) HealthCheck(ctx context.Context) HealthStatus {
	status := HealthStatus{
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	start := time.Now()

	if err := p.Ping(); err != nil {
		status.Healthy = false
		status.Error = p.sanitizeError(err).Error()
		status.ResponseTime = time.Since(start)
		return status
	}

	status.Healthy = true
	status.ResponseTime = time.Since(start)

	// Add health details
	stats := p.GetStats()
	status.Details["database_size"] = stats.DatabaseSize
	status.Details["total_queries"] = stats.TotalQueries
	status.Details["connection_count"] = stats.OpenConnections
	status.Details["version"] = p.Version()

	return status
}

// sanitizeError removes sensitive information from error messages
func (p *PostgreSQLProvider) sanitizeError(err error) error {
	if err == nil {
		return nil
	}

	sanitizedMsg := err.Error()

	// The passwords this provider was given are known, so remove them by value
	// (exact match, no pattern), in every spelling a driver or the URL package
	// may print them in, wherever in the text they land. The patterns below only
	// catch what looks like a credential.
	for _, password := range p.knownPasswords() {
		for _, form := range []string{
			password,
			url.QueryEscape(password),
			url.PathEscape(password),
			strings.TrimPrefix(url.UserPassword("", password).String(), ":"),
		} {
			sanitizedMsg = strings.ReplaceAll(sanitizedMsg, form, "***")
		}
	}

	// Patterns to match and remove credentials from DSN-like strings
	// Match user=username patterns
	credentialPatterns := []*regexp.Regexp{
		regexp.MustCompile(`user=\w+`),
		regexp.MustCompile(`password=\S+`),
		regexp.MustCompile(`dbname=\w+`),
		regexp.MustCompile(`application_name=\S+`),
	}

	// Replace credentials with safe placeholders
	for _, pattern := range credentialPatterns {
		matches := pattern.FindAllString(sanitizedMsg, -1)
		for _, match := range matches {
			parts := strings.SplitN(match, "=", 2)
			sanitizedMsg = strings.ReplaceAll(sanitizedMsg, match, parts[0]+"=***")
		}
	}

	// Remove any remaining potential credential information in URLs
	urlPattern := regexp.MustCompile(`://[^:]+:[^@]+@`)
	sanitizedMsg = urlPattern.ReplaceAllString(sanitizedMsg, "://***:***@")

	return fmt.Errorf("%s", sanitizedMsg)
}

// knownPasswords lists the password values the provider holds: the
// database.password field, and the userinfo password and password query values
// of the DSN where it parses.
func (p *PostgreSQLProvider) knownPasswords() []string {
	var out []string
	if p.config.Password != "" {
		out = append(out, p.config.Password)
	}
	if u, err := url.Parse(p.config.DSN); err == nil {
		if password, ok := u.User.Password(); ok && password != "" {
			out = append(out, password)
		}
		for _, value := range u.Query()["password"] {
			if value != "" {
				out = append(out, value)
			}
		}
	}
	return out
}

// validateDSNInput validates and sanitizes DSN components for security
func (p *PostgreSQLProvider) validateDSNInput(dsn string) error {
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

// credentialSafeQueryKeys are the DSN query keys accepted when
// database.username or database.password is set.
var credentialSafeQueryKeys = map[string]bool{
	"sslmode": true, "connect_timeout": true, "application_name": true, "search_path": true,
	"sslcert": true, "sslkey": true, "sslrootcert": true,
}

// errDSNNotAURL is what a DSN that url.Parse refused reports. url.Error quotes
// the DSN, and even its inner text quotes a slice of it (a password that holds
// a "/" ends up in the "invalid port" message), so nothing of it is passed on.
var errDSNNotAURL = errors.New("the DSN is not a valid URL (postgresql://[user[:password]@]host[:port]/database[?options])")

// buildDSN constructs a PostgreSQL DSN with SSL/TLS settings and security validation
func (p *PostgreSQLProvider) buildDSN(baseDSN string, options map[string]string) (string, error) {
	// Validate DSN format
	if baseDSN == "" {
		return "", fmt.Errorf("DSN cannot be empty")
	}

	// Security validation: Check for dangerous patterns
	if err := p.validateDSNInput(baseDSN); err != nil {
		return "", fmt.Errorf("DSN security validation failed: %w", err)
	}

	// Parse the base DSN to validate format
	parsedURL, err := url.Parse(baseDSN)
	if err != nil {
		return "", fmt.Errorf("invalid DSN format: %w", errDSNNotAURL)
	}

	// Ensure it has a scheme that looks like a valid database URL
	if parsedURL.Scheme == "" || (!strings.HasPrefix(parsedURL.Scheme, "postgres")) {
		return "", fmt.Errorf("invalid PostgreSQL DSN scheme, expected postgres:// or postgresql://")
	}

	// A URL the driver would read differently from net/url puts a password into
	// the host or the database name, and the driver prints both in its connect
	// error: "postgresql://h:1/secret@h/db" parses with "secret@h/db" as the
	// database. A database name is one path segment without "@" or a blank.
	if name := strings.TrimPrefix(parsedURL.Path, "/"); strings.ContainsAny(name, "/@ \t") {
		return "", fmt.Errorf("invalid DSN format: the database name must be one path segment without '@', '/' or blanks")
	}

	// database.username and database.password are the credentials kept out of
	// the DSN. Refuse the cases that have no single meaning; the messages name
	// the keys and never a value.
	hasCredentials := p.config.Username != "" || p.config.Password != ""
	if hasCredentials {
		if p.config.Username == "" {
			return "", fmt.Errorf("database.password is set without database.username")
		}
		if parsedURL.User != nil {
			return "", fmt.Errorf("the DSN carries credentials and database.username is set as well: use one of them")
		}
		// pgx reads the user, password, passfile and service parameters from the
		// query and lets them win over the userinfo. Its key parsing (trimmed,
		// decoded, a '#' that net/url treats as a fragment) differs from net/url's,
		// so inspecting keys would never agree with it. Accept only the literal
		// keys below; anything else is refused without printing it.
		if baseQuery := strings.SplitN(baseDSN, "?", 2); len(baseQuery) == 2 {
			for _, pair := range strings.Split(baseQuery[1], "&") {
				key, _, _ := strings.Cut(pair, "=")
				if !credentialSafeQueryKeys[key] {
					return "", fmt.Errorf("the DSN query holds a parameter that is not allowed together with database.username (allowed: sslmode, connect_timeout, application_name, search_path, sslcert, sslkey, sslrootcert)")
				}
			}
		}
	}

	// Start with the base DSN
	dsn := baseDSN

	// Add SSL mode if not specified (default to require for security)
	if !strings.Contains(dsn, "sslmode=") {
		sslMode := "require" // Default to requiring SSL
		if mode, ok := options["sslmode"]; ok {
			sslMode = mode
		}

		if strings.Contains(dsn, "?") {
			dsn += "&sslmode=" + sslMode
		} else {
			dsn += "?sslmode=" + sslMode
		}
	}

	// Add connection timeout if not specified
	if !strings.Contains(dsn, "connect_timeout=") {
		timeout := "10" // 10 seconds default
		if t, ok := options["connect_timeout"]; ok {
			timeout = t
		}

		if strings.Contains(dsn, "?") {
			dsn += "&connect_timeout=" + timeout
		} else {
			dsn += "?connect_timeout=" + timeout
		}
	}

	// Add other PostgreSQL-specific options with validation
	for key, value := range options {
		// Security validation: Check for dangerous patterns in options
		if err := p.validateDSNInput(key + "=" + value); err != nil {
			continue // Skip dangerous options silently to prevent information disclosure
		}

		switch key {
		case "sslcert", "sslkey", "sslrootcert", "application_name", "search_path":
			if !strings.Contains(dsn, key+"=") {
				if strings.Contains(dsn, "?") {
					dsn += "&" + key + "=" + url.QueryEscape(value)
				} else {
					dsn += "?" + key + "=" + url.QueryEscape(value)
				}
			}
		}
	}

	// The credentials go in last: a password holding "sslmode=" or "=" must not
	// change which defaults the checks above add to the DSN.
	if hasCredentials {
		withCredentials, err := url.Parse(dsn)
		if err != nil {
			return "", fmt.Errorf("invalid DSN format: %w", errDSNNotAURL)
		}
		if p.config.Password == "" {
			withCredentials.User = url.User(p.config.Username)
		} else {
			withCredentials.User = url.UserPassword(p.config.Username, p.config.Password)
		}
		dsn = withCredentials.String()
	}

	return dsn, nil
}

// configureConnectionPool sets up PostgreSQL connection pool parameters
func (p *PostgreSQLProvider) configureConnectionPool() error {
	sqlDB, err := p.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying SQL DB: %w", p.sanitizeError(err))
	}

	// Set connection pool parameters with PostgreSQL-appropriate defaults
	maxOpenConns := p.config.MaxOpenConns
	if maxOpenConns == 0 {
		maxOpenConns = 25 // PostgreSQL can handle more concurrent connections
	}
	sqlDB.SetMaxOpenConns(maxOpenConns)

	maxIdleConns := p.config.MaxIdleConns
	if maxIdleConns == 0 {
		maxIdleConns = 5
	}
	sqlDB.SetMaxIdleConns(maxIdleConns)

	connMaxLifetime := p.config.ConnMaxLifetime
	if connMaxLifetime == 0 {
		connMaxLifetime = time.Hour // PostgreSQL connections can live longer
	}
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	connMaxIdleTime := p.config.ConnMaxIdleTime
	if connMaxIdleTime == 0 {
		connMaxIdleTime = 10 * time.Minute
	}
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)

	p.logger.WithFields(map[string]any{
		"max_open_conns":     maxOpenConns,
		"max_idle_conns":     maxIdleConns,
		"conn_max_lifetime":  connMaxLifetime,
		"conn_max_idle_time": connMaxIdleTime,
	}).Debug("Configured PostgreSQL connection pool")

	return nil
}

// updateProviderVersion retrieves and stores the PostgreSQL version
func (p *PostgreSQLProvider) updateProviderVersion() error {
	var version string
	if err := p.db.Raw("SELECT version()").Scan(&version).Error; err != nil {
		return err
	}

	// Extract version number from version string
	if strings.Contains(version, "PostgreSQL") {
		parts := strings.Fields(version)
		if len(parts) >= 2 {
			p.statsMu.Lock()
			p.stats.ProviderVersion = parts[1]
			p.stats.Metadata["full_version"] = version
			p.statsMu.Unlock()
		}
	}

	return nil
}

// createGormLogger creates a GORM logger instance based on configuration
func (p *PostgreSQLProvider) createGormLogger() logger.Interface {
	var logLevel logger.LogLevel

	switch p.config.LogLevel {
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
		log.New(&gormLogWriter{logger: p.logger}, "", 0),
		logger.Config{
			SlowThreshold:             p.config.SlowQueryThreshold,
			LogLevel:                  logLevel,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true, // keep bound values, such as device logins, out of the SQL log
			Colorful:                  false,
		},
	)
}

// updateStats updates runtime database statistics
func (p *PostgreSQLProvider) updateStats() {
	if !p.connected || p.db == nil {
		return
	}

	sqlDB, err := p.db.DB()
	if err != nil {
		return
	}

	stats := sqlDB.Stats()
	p.stats.OpenConnections = stats.OpenConnections
	p.stats.InUseConnections = stats.InUse
	p.stats.IdleConnections = stats.Idle

	// Get database size (PostgreSQL specific query)
	var dbSize int64
	if err := p.db.Raw("SELECT pg_database_size(current_database())").Scan(&dbSize).Error; err == nil {
		p.stats.DatabaseSize = dbSize
	}
}

// getHostFromDSN extracts host from DSN for logging
func (p *PostgreSQLProvider) getHostFromDSN(dsn string) string {
	if parsedURL, err := url.Parse(dsn); err == nil && parsedURL.Host != "" {
		return parsedURL.Host
	}
	return "unknown"
}

// getDatabaseFromDSN extracts database name from DSN for logging
func (p *PostgreSQLProvider) getDatabaseFromDSN(dsn string) string {
	if parsedURL, err := url.Parse(dsn); err == nil && parsedURL.Path != "" {
		return strings.TrimPrefix(parsedURL.Path, "/")
	}
	return "unknown"
}

// validateSSLConfig validates SSL certificate configuration
func (p *PostgreSQLProvider) validateSSLConfig(options map[string]string) error {
	sslMode, hasSslMode := options["sslmode"]
	if !hasSslMode {
		return nil // Default SSL mode will be applied
	}

	// Validate SSL mode
	validSSLModes := map[string]bool{
		"disable":     true,
		"allow":       true,
		"prefer":      true,
		"require":     true,
		"verify-ca":   true,
		"verify-full": true,
	}

	if !validSSLModes[sslMode] {
		return fmt.Errorf("invalid SSL mode: %s", sslMode)
	}

	// If using verify-ca or verify-full, ensure certificate files exist
	if sslMode == "verify-ca" || sslMode == "verify-full" {
		if sslRootCert, ok := options["sslrootcert"]; ok {
			if _, err := os.Stat(sslRootCert); os.IsNotExist(err) {
				return fmt.Errorf("SSL root certificate not found: %s", sslRootCert)
			}
		}

		if sslCert, ok := options["sslcert"]; ok {
			if _, err := os.Stat(sslCert); os.IsNotExist(err) {
				return fmt.Errorf("SSL certificate not found: %s", sslCert)
			}
		}

		if sslKey, ok := options["sslkey"]; ok {
			if _, err := os.Stat(sslKey); os.IsNotExist(err) {
				return fmt.Errorf("SSL key not found: %s", sslKey)
			}
		}
	}

	return nil
}

// postgresTransaction implements the Transaction interface for PostgreSQL
type postgresTransaction struct {
	tx *gorm.DB
}

func (t *postgresTransaction) GetDB() *gorm.DB {
	return t.tx
}

func (t *postgresTransaction) Commit() error {
	return t.tx.Commit().Error
}

func (t *postgresTransaction) Rollback() error {
	return t.tx.Rollback().Error
}
