package provider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"maps"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
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
		return fmt.Errorf("failed to build DSN: %w", err)
	}

	// The driver's own parser reads the DSN once, here, and what it refuses is
	// reported without a word of its error: it quotes the DSN (#587).
	connConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("failed to parse DSN: %w", errDSNNotParsed)
	}

	// Every connection the pool makes goes through the sealed connector, so a
	// failure carries a classification and never the driver's text, whether it
	// is the first connect or a reconnect long after.
	sqlDB := sql.OpenDB(sealConnector(stdlib.GetConnector(*connConfig)))

	// GORM logs nothing until the probes below are done: an error of an open
	// connection (a reset on a unix socket, a refused write) quotes the socket
	// path or the address, and the provider reports those errors sealed itself.
	gormConfig := &gorm.Config{
		Logger: logger.Discard,
	}

	// Open database connection with PostgreSQL driver
	db, err := openGORM(postgres.New(postgres.Config{Conn: sqlDB}), gormConfig)
	if err != nil {
		_ = sqlDB.Close()
		sealed := sealError(err)
		p.logger.WithFields(map[string]any{"error": sealed.Error()}).Error("Failed to connect to PostgreSQL database")
		return fmt.Errorf("failed to connect to PostgreSQL database: %w", sealed)
	}

	p.db = db

	// Configure connection pool settings
	if err := p.configureConnectionPool(); err != nil {
		p.db = nil
		_ = sqlDB.Close()
		return fmt.Errorf("failed to configure connection pool: %w", err)
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
		_ = sqlDB.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Get PostgreSQL version
	if err := p.updateProviderVersion(); err != nil {
		p.logger.WithFields(map[string]any{
			"error": sealError(err).Error(),
		}).Warn("Failed to get PostgreSQL version")
	}

	// From here on GORM logs as configured.
	db.Logger = p.createGormLogger()

	// Parsing does not make a field safe: a misplaced password can land in the
	// host or the database name, so only the target is printed.
	p.logger.WithFields(map[string]any{
		"provider": "postgresql",
		"target":   config.Target(),
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
		return fmt.Errorf("failed to close database: %w", sealOpenError(err))
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
		return fmt.Errorf("failed to get underlying SQL DB: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return sealError(sqlDB.PingContext(ctx))
}

// Migrate performs database migration
func (p *PostgreSQLProvider) Migrate(models ...interface{}) error {
	if !p.connected || p.db == nil {
		return fmt.Errorf("not connected to database")
	}

	start := time.Now()
	// The migrator reads some things from database/sql directly (Row().Scan,
	// rows.ColumnTypes, rows.Close), which neither the dialector nor the row
	// callback sees, so what it returns is sealed here, before it is logged.
	err := sealOpenError(p.db.AutoMigrate(models...))
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
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
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
		status.Error = err.Error()
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

// credentialSafeQueryKeys are the DSN query keys accepted when
// database.username or database.password is set. Each is a client option pgx
// reads itself (pgconn/config.go, conn.go) or a server setting that names no
// identity, and none names a credential, a host or a user. A key that is not
// listed is refused, so the keys that override them stay out: user, password,
// passfile, service, servicefile, sslpassword, host, port and database, and
// any key pgx would send to the server as a setting (role, for one).
// default_query_exec_mode is listed: pgx reads it itself and it names no identity.
var credentialSafeQueryKeys = map[string]bool{
	"sslmode": true, "connect_timeout": true, "application_name": true, "search_path": true,
	"default_query_exec_mode": true,
	"sslcert":                 true, "sslkey": true, "sslrootcert": true,
	"sslcertmode": true, "sslsni": true, "sslnegotiation": true,
	"channel_binding": true, "require_auth": true, "target_session_attrs": true,
	"statement_cache_capacity": true, "description_cache_capacity": true,
}

// errDSNNotAURL is what a DSN that url.Parse refused reports. url.Error quotes
// the DSN, and even its inner text quotes a slice of it (a password that holds
// a "/" ends up in the "invalid port" message), so nothing of it is passed on.
var errDSNNotAURL = errors.New("the DSN is not a valid URL (postgresql://[user[:password]@]host[:port]/database[?options])")

// buildDSN adds the SSL and timeout settings the provider wants by default to a
// PostgreSQL DSN, and the options that name a setting, and puts the credentials of
// database.username and database.password in last. It reads the DSN as net/url and
// pgx do: a word in a host, a database name or a password is part of that value,
// and a setting is given when the query holds its key. It does not read the text
// for content.
func (p *PostgreSQLProvider) buildDSN(baseDSN string, options map[string]string) (string, error) {
	// Validate DSN format
	if baseDSN == "" {
		return "", fmt.Errorf("DSN cannot be empty")
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
					return "", fmt.Errorf("the DSN query holds a parameter that is not allowed together with database.username (allowed: %s)",
						strings.Join(slices.Sorted(maps.Keys(credentialSafeQueryKeys)), ", "))
				}
			}
		}
	}

	// Start with the base DSN
	dsn := baseDSN

	// A setting is given when the query of the DSN holds its key, read as pgx reads
	// it (net/url) and kept current as settings are added. A word of the user, the
	// password or the host is never a key.
	given := parsedURL.Query()
	hasQuery := parsedURL.RawQuery != "" || parsedURL.ForceQuery
	add := func(key, value string) {
		separator := "?"
		if hasQuery {
			separator = "&"
		}
		dsn += separator + key + "=" + value
		hasQuery = true
		given.Set(key, value)
	}

	// Defaults: require SSL, and a bounded connect time. An option of the same name
	// replaces the default. The values are escaped, so a value holding "&" adds no key
	// (password, user) next to the configured credentials.
	for _, setting := range []struct{ key, fallback string }{{"sslmode", "require"}, {"connect_timeout", "10"}} {
		if given.Has(setting.key) {
			continue
		}
		value := setting.fallback
		if option, ok := options[setting.key]; ok {
			value = url.QueryEscape(option)
		}
		add(setting.key, value)
	}

	// The options that only name a setting.
	for _, key := range []string{"sslcert", "sslkey", "sslrootcert", "application_name", "search_path", "default_query_exec_mode"} {
		if value, ok := options[key]; ok && !given.Has(key) {
			add(key, url.QueryEscape(value))
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
		return fmt.Errorf("failed to get underlying SQL DB: %w", err)
	}

	// Apply the pool as configured. The defaults live in the config helper (Config.GetDatabaseConfig),
	// and the factory refuses an unset open or idle count before this provider is built.
	sqlDB.SetMaxOpenConns(p.config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(p.config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(p.config.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(p.config.ConnMaxIdleTime)

	p.logger.WithFields(map[string]any{
		"max_open_conns":     p.config.MaxOpenConns,
		"max_idle_conns":     p.config.MaxIdleConns,
		"conn_max_lifetime":  p.config.ConnMaxLifetime,
		"conn_max_idle_time": p.config.ConnMaxIdleTime,
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
