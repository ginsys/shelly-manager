package provider

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// SQLiteProvider implements DatabaseProvider for SQLite databases
type SQLiteProvider struct {
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

// NewSQLiteProvider creates a new SQLite database provider
func NewSQLiteProvider(logger *logging.Logger) *SQLiteProvider {
	if logger == nil {
		logger = logging.GetDefault()
	}

	return &SQLiteProvider{
		logger: logger,
		stats: DatabaseStats{
			ProviderName:    "SQLite",
			ProviderVersion: "3.x",
			Metadata:        make(map[string]interface{}),
		},
	}
}

// Connect establishes connection to the SQLite database
func (s *SQLiteProvider) Connect(config DatabaseConfig) error {
	s.connMu.Lock()
	defer s.connMu.Unlock()

	if s.connected {
		return fmt.Errorf("already connected to database")
	}

	s.config = config

	// Validate and create directory if needed
	if err := s.prepareDatabasePath(config.DSN); err != nil {
		return fmt.Errorf("failed to prepare database path: %w", err)
	}

	// Configure GORM logger based on config
	gormConfig := &gorm.Config{
		Logger: s.createGormLogger(),
	}

	// Open database connection
	db, err := gorm.Open(sqlite.Open(config.DSN), gormConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SQLite database: %w", err)
	}

	s.db = db

	// Configure SQLite-specific options
	if err := s.configureDatabase(); err != nil {
		s.db = nil
		return fmt.Errorf("failed to configure database: %w", err)
	}

	s.connected = true
	s.logger.WithFields(map[string]any{
		"provider": "sqlite",
		"dsn":      config.DSN,
	}).Info("Connected to SQLite database")

	return nil
}

// Close closes the database connection
func (s *SQLiteProvider) Close() error {
	s.connMu.Lock()
	defer s.connMu.Unlock()

	if !s.connected || s.db == nil {
		return nil
	}

	sqlDB, err := s.db.DB()
	if err == nil {
		err = sqlDB.Close()
	}

	s.db = nil
	s.connected = false

	if err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	s.logger.Info("Closed SQLite database connection")
	return nil
}

// Ping checks if the database connection is alive
func (s *SQLiteProvider) Ping() error {
	if !s.connected || s.db == nil {
		return fmt.Errorf("not connected to database")
	}

	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying SQL DB: %w", err)
	}

	return sqlDB.Ping()
}

// Migrate performs database migration
func (s *SQLiteProvider) Migrate(models ...interface{}) error {
	if !s.connected || s.db == nil {
		return fmt.Errorf("not connected to database")
	}

	start := time.Now()
	err := s.db.AutoMigrate(models...)
	duration := time.Since(start)

	if err != nil {
		s.logger.WithFields(map[string]any{
			"error":    err.Error(),
			"duration": duration,
			"models":   len(models),
		}).Error("Database migration failed")
		atomic.AddInt64(&s.failedQueries, 1)
		return fmt.Errorf("migration failed: %w", err)
	}

	s.logger.WithFields(map[string]any{
		"duration": duration,
		"models":   len(models),
	}).Info("Database migration completed successfully")

	return nil
}

// DropTables drops the specified tables
func (s *SQLiteProvider) DropTables(models ...interface{}) error {
	if !s.connected || s.db == nil {
		return fmt.Errorf("not connected to database")
	}

	return s.db.Migrator().DropTable(models...)
}

// BeginTransaction starts a new database transaction
func (s *SQLiteProvider) BeginTransaction() (Transaction, error) {
	if !s.connected || s.db == nil {
		return nil, fmt.Errorf("not connected to database")
	}

	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	return &sqliteTransaction{tx: tx}, nil
}

// GetDB returns the underlying GORM database instance
func (s *SQLiteProvider) GetDB() *gorm.DB {
	return s.db
}

// GetStats returns database statistics
func (s *SQLiteProvider) GetStats() DatabaseStats {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()

	// Update runtime statistics
	s.updateStats()

	stats := s.stats
	stats.TotalQueries = atomic.LoadInt64(&s.queryCount)
	stats.SlowQueries = atomic.LoadInt64(&s.slowQueries)
	stats.FailedQueries = atomic.LoadInt64(&s.failedQueries)

	if stats.TotalQueries > 0 {
		stats.AverageLatency = time.Duration(atomic.LoadInt64(&s.totalLatency) / stats.TotalQueries)
	}

	return stats
}

// SetLogger sets the logger for the provider
func (s *SQLiteProvider) SetLogger(logger *logging.Logger) {
	s.logger = logger
}

// Name returns the provider name
func (s *SQLiteProvider) Name() string {
	return "SQLite"
}

// Version returns the provider version
func (s *SQLiteProvider) Version() string {
	return "3.x"
}

// prepareDatabasePath creates the directory structure for the database file
func (s *SQLiteProvider) prepareDatabasePath(dsn string) error {
	// Skip for in-memory databases
	if dsn == ":memory:" {
		return nil
	}

	dir := filepath.Dir(dsn)
	if dir != "" && dir != "." {
		// Create directory if it doesn't exist
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create database directory %s: %w", dir, err)
		}

		// Test write permissions
		testFile := filepath.Join(dir, ".db_write_test")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			return fmt.Errorf("insufficient permissions for database directory %s: %w", dir, err)
		}
		// Clean up test file
		if err := os.Remove(testFile); err != nil {
			// Log but don't fail since it's just cleanup
			_ = err // Ignore cleanup error
		}
	}

	return nil
}

// configureDatabase applies SQLite-specific configuration options
func (s *SQLiteProvider) configureDatabase() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying SQL DB: %w", err)
	}

	// Apply connection limits (SQLite specific)
	sqlDB.SetMaxOpenConns(1) // SQLite only supports 1 concurrent write connection
	sqlDB.SetMaxIdleConns(1)

	// Apply SQLite pragmas from options
	pragmas := s.getSQLitePragmas()
	for pragma, value := range pragmas {
		query := fmt.Sprintf("PRAGMA %s = %s", pragma, value)
		if err := s.db.Exec(query).Error; err != nil {
			s.logger.WithFields(map[string]any{
				"pragma": pragma,
				"value":  value,
				"error":  err.Error(),
			}).Warn("Failed to set SQLite pragma")
		}
	}

	return nil
}

// getSQLitePragmas returns SQLite pragma settings from configuration
func (s *SQLiteProvider) getSQLitePragmas() map[string]string {
	pragmas := make(map[string]string)

	// Default pragmas for optimal performance and reliability
	pragmas["foreign_keys"] = "ON"
	pragmas["journal_mode"] = "WAL"
	pragmas["synchronous"] = "NORMAL"
	pragmas["cache_size"] = "-64000" // 64MB
	pragmas["busy_timeout"] = "5000" // 5 seconds

	// Override with configuration options
	for key, value := range s.config.Options {
		switch key {
		case "foreign_keys", "journal_mode", "synchronous", "cache_size", "busy_timeout":
			pragmas[key] = value
		}
	}

	return pragmas
}

// createGormLogger creates a GORM logger instance based on configuration
func (s *SQLiteProvider) createGormLogger() logger.Interface {
	var logLevel logger.LogLevel

	switch s.config.LogLevel {
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
		log.New(&gormLogWriter{logger: s.logger}, "", 0),
		logger.Config{
			SlowThreshold:             s.config.SlowQueryThreshold,
			LogLevel:                  logLevel,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)
}

// updateStats updates runtime database statistics
func (s *SQLiteProvider) updateStats() {
	if !s.connected || s.db == nil {
		return
	}

	sqlDB, err := s.db.DB()
	if err != nil {
		return
	}

	stats := sqlDB.Stats()
	s.stats.OpenConnections = stats.OpenConnections
	s.stats.InUseConnections = stats.InUse
	s.stats.IdleConnections = stats.Idle

	// Get database size
	if s.config.DSN != ":memory:" {
		if stat, err := os.Stat(s.config.DSN); err == nil {
			s.stats.DatabaseSize = stat.Size()
		}
	}
}

// sqliteTransaction implements the Transaction interface for SQLite
type sqliteTransaction struct {
	tx *gorm.DB
}

func (t *sqliteTransaction) GetDB() *gorm.DB {
	return t.tx
}

func (t *sqliteTransaction) Commit() error {
	return t.tx.Commit().Error
}

func (t *sqliteTransaction) Rollback() error {
	return t.tx.Rollback().Error
}

// gormLogWriter adapts our logging.Logger to GORM's log writer interface
type gormLogWriter struct {
	logger *logging.Logger
}

func (w *gormLogWriter) Write(p []byte) (n int, err error) {
	message := string(p)

	// Log GORM messages through our logger
	w.logger.Debug(message)

	return len(p), nil
}

// HealthCheck implements HealthChecker interface
func (s *SQLiteProvider) HealthCheck(ctx context.Context) HealthStatus {
	status := HealthStatus{
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	start := time.Now()

	if err := s.Ping(); err != nil {
		status.Healthy = false
		status.Error = err.Error()
		status.ResponseTime = time.Since(start)
		return status
	}

	status.Healthy = true
	status.ResponseTime = time.Since(start)

	// Add health details
	stats := s.GetStats()
	status.Details["database_size"] = stats.DatabaseSize
	status.Details["total_queries"] = stats.TotalQueries
	status.Details["connection_count"] = stats.OpenConnections

	return status
}
