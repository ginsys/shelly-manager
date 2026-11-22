package provider

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
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

	// The MySQL DSN has its own format and escaping, and nothing builds it from
	// separate credentials yet: refuse rather than ignore them silently.
	if config.Username != "" || config.Password != "" {
		return fmt.Errorf("database.username and database.password are not supported by the MySQL provider; put the credentials in the DSN")
	}

	m.config = config

	// Parse and validate DSN
	dsn, err := m.buildDSN(config.DSN, config.Options)
	if err != nil {
		return fmt.Errorf("failed to build DSN: %w", err)
	}

	// GORM logs nothing until the probes below are done: an error of an open
	// connection (a reset on a unix socket, a refused write) quotes the socket
	// path or the address, and the provider reports those errors sealed itself.
	gormConfig := &gorm.Config{
		Logger: logger.Discard,
	}

	// Parse the DSN with the driver's own parser, and open the pool on a sealed
	// connector, so no connect error of the pool carries driver text (the parser
	// and the dial both quote operator input).
	driverConfig, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("failed to parse DSN: %w", errDSNNotParsed)
	}
	// The driver logs a failed read, with the socket or address in it, to stderr
	// before the sealed connector sees the error: silence it for this connector.
	driverConfig.Logger = &mysqldriver.NopLogger{}
	connector, err := mysqldriver.NewConnector(driverConfig)
	if err != nil {
		return fmt.Errorf("failed to parse DSN: %w", errDSNNotParsed)
	}
	sqlDB := sql.OpenDB(sealConnector(connector))

	// Open database connection with the GORM MySQL dialector on that pool
	db, err := openGORM(gormmysql.New(gormmysql.Config{Conn: sqlDB, DSNConfig: driverConfig}), gormConfig)
	if err != nil {
		_ = sqlDB.Close()
		sealed := sealError(err)
		m.logger.WithFields(map[string]any{"error": sealed.Error()}).Error("Failed to connect to MySQL database")
		return fmt.Errorf("failed to connect to MySQL database: %w", sealed)
	}

	m.db = db

	// Configure connection pool settings
	if err := m.configureConnectionPool(); err != nil {
		m.db = nil
		_ = sqlDB.Close()
		return fmt.Errorf("failed to configure connection pool: %w", err)
	}

	// Mark the handle live before probing it: Ping (and updateProviderVersion)
	// refuse to run while connected is false, so pinging first made Connect
	// fail unconditionally with "not connected to database". Rolled back below
	// if the probe fails.
	m.connected = true

	// Test the connection
	if err := m.Ping(); err != nil {
		m.connected = false
		m.db = nil
		_ = sqlDB.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Get MySQL version
	if err := m.updateProviderVersion(); err != nil {
		m.logger.WithFields(map[string]any{
			"error": sealError(err).Error(),
		}).Warn("Failed to get MySQL version")
	}

	// From here on GORM logs as configured.
	db.Logger = m.createGormLogger()

	m.logger.WithFields(map[string]any{
		"provider": "mysql",
		"target":   config.Target(),
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
		return fmt.Errorf("failed to close database: %w", sealOpenError(err))
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
		return fmt.Errorf("failed to get underlying SQL DB: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return sealError(sqlDB.PingContext(ctx))
}

// Migrate performs database migration
func (m *MySQLProvider) Migrate(models ...interface{}) error {
	if !m.connected || m.db == nil {
		return fmt.Errorf("not connected to database")
	}

	start := time.Now()
	// The migrator reads some things from database/sql directly (Row().Scan,
	// rows.ColumnTypes, rows.Close), which neither the dialector nor the row
	// callback sees, so what it returns is sealed here, before it is logged.
	err := sealOpenError(m.db.AutoMigrate(models...))
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
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
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

// dsnParamSection returns the parameter section of a MySQL DSN and whether it has
// one, cut the way the driver cuts it: after the last "/" (the user, the password
// and the host may hold one) and after the first "?" that follows.
func dsnParamSection(dsn string) (string, bool) {
	_, section, found := strings.Cut(dsn[strings.LastIndex(dsn, "/")+1:], "?")
	return section, found
}

// dsnGivesKey reports whether the parameter section of the DSN sets key. A pair
// with no "=" sets nothing: the driver skips it.
func dsnGivesKey(dsn, key string) bool {
	section, _ := dsnParamSection(dsn)
	for _, pair := range strings.Split(section, "&") {
		if k, _, ok := strings.Cut(pair, "="); ok && k == key {
			return true
		}
	}
	return false
}

// addDSNParam appends one parameter to the parameter section of the DSN, or starts it.
func addDSNParam(dsn, key, value string) string {
	section, found := dsnParamSection(dsn)
	separator := "?"
	if found && section != "" {
		separator = "&"
	} else if found {
		separator = ""
	}
	return dsn + separator + key + "=" + value
}

// validateTimeoutOptions refuses a timeout option that is not a duration, the way
// validateSSLConfig refuses a TLS mode: the driver reads the same string with the
// same function, so every spelling it reads keeps working. The message names the
// option and not its value.
func validateTimeoutOptions(options map[string]string) error {
	for _, key := range []string{"timeout", "readTimeout", "writeTimeout"} {
		if value, ok := options[key]; ok {
			if _, err := time.ParseDuration(value); err != nil {
				return fmt.Errorf("invalid %s option: not a duration such as 10s or 1m", key)
			}
		}
	}
	return nil
}

// escapeOption escapes the value of an option so that it stays one value. A
// timeout is a checked duration and is added as it is, so that every spelling the
// driver reads keeps working. A charset option may list several names separated by
// a comma, and each name is escaped. Any other value, loc included (the driver
// decodes it), is query-escaped.
func escapeOption(key, value string) string {
	switch key {
	case "timeout", "readTimeout", "writeTimeout":
		return value
	case "charset":
		names := strings.Split(value, ",")
		for i, name := range names {
			names[i] = url.QueryEscape(name)
		}
		return strings.Join(names, ",")
	}
	return url.QueryEscape(value)
}

// buildDSN adds the TLS, timeout and character set settings the provider wants by
// default to a MySQL DSN, and the options that name one of them. It reads the DSN
// as the driver does: a word in the user, the password, the host or the database
// name is part of that value, and a setting is given when the parameter section
// holds its key. It does not read the text for content, and it does not filter
// session settings: those reach the server as the driver sends them. Connect
// parses the result with the driver.
func (m *MySQLProvider) buildDSN(baseDSN string, options map[string]string) (string, error) {
	// Validate DSN format
	if baseDSN == "" {
		return "", fmt.Errorf("DSN cannot be empty")
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
	if err := validateTimeoutOptions(options); err != nil {
		return "", err
	}

	// Start with the base DSN
	dsn := baseDSN

	// A default is added when the DSN does not give the key: TLS preferred, so a
	// server that offers TLS gets it, bounded timeouts, and utf8mb4 for full Unicode.
	// An option of the same name replaces the default.
	for _, setting := range []struct{ key, fallback string }{
		{"tls", "preferred"}, {"timeout", "10s"}, {"readTimeout", "30s"},
		{"writeTimeout", "30s"}, {"charset", "utf8mb4"},
	} {
		if dsnGivesKey(dsn, setting.key) {
			continue
		}
		value := setting.fallback
		if option, ok := options[setting.key]; ok {
			value = option
		}
		dsn = addDSNParam(dsn, setting.key, escapeOption(setting.key, value))
	}

	// The options that only name a setting are added when the DSN does not give it.
	for _, key := range []string{"collation", "loc", "maxAllowedPacket"} {
		if value, ok := options[key]; ok && !dsnGivesKey(dsn, key) {
			dsn = addDSNParam(dsn, key, escapeOption(key, value))
		}
	}

	return dsn, nil
}

// configureConnectionPool sets up MySQL connection pool parameters
func (m *MySQLProvider) configureConnectionPool() error {
	sqlDB, err := m.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying SQL DB: %w", err)
	}

	// Apply the pool as configured. The defaults live in the config helper (Config.GetDatabaseConfig),
	// and the factory refuses an unset open or idle count before this provider is built.
	sqlDB.SetMaxOpenConns(m.config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(m.config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(m.config.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(m.config.ConnMaxIdleTime)

	m.logger.WithFields(map[string]any{
		"max_open_conns":     m.config.MaxOpenConns,
		"max_idle_conns":     m.config.MaxIdleConns,
		"conn_max_lifetime":  m.config.ConnMaxLifetime,
		"conn_max_idle_time": m.config.ConnMaxIdleTime,
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
			ParameterizedQueries:      true, // keep bound values, such as device logins, out of the SQL log
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

// validateSSLConfig validates the TLS mode for MySQL. The driver takes these four values for `tls=`,
// or the name of a config registered with it, and this provider registers none, so any other value
// would fail at connect time with "invalid value / unknown config name". The driver also has no `ca`,
// `cert`, `key` or `serverName` parameter and would keep each as a session variable, which the server
// refuses, so those options are refused here with a message that says what to use.
func (m *MySQLProvider) validateSSLConfig(options map[string]string) error {
	for _, name := range []string{"ca", "cert", "key", "serverName"} {
		if _, ok := options[name]; ok {
			return fmt.Errorf("custom TLS files are not supported; use tls=preferred|true|skip-verify (option %q)", name)
		}
	}

	tlsMode, hasTlsMode := options["tls"]
	if !hasTlsMode {
		return nil // Default TLS mode will be applied
	}

	validTLSModes := map[string]bool{
		"false":       true, // no TLS
		"true":        true, // TLS; the certificate chain and the host name are verified
		"skip-verify": true, // TLS; the certificate is not verified
		"preferred":   true, // TLS if the server offers it, plain otherwise; not verified
	}

	if !validTLSModes[tlsMode] {
		return fmt.Errorf("invalid TLS mode: %s (the driver takes false, true, skip-verify or preferred)", tlsMode)
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
