package provider

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// PostgreSQLTestContainer wraps PostgreSQL container functionality
type PostgreSQLTestContainer struct {
	Host     string
	Port     string
	Database string
	Username string
	Password string
	DSN      string
}

// SetupPostgreSQLContainer creates a PostgreSQL container for testing
// This function attempts to use Docker testcontainers if available,
// otherwise falls back to using environment variables for connection
func SetupPostgreSQLContainer(t *testing.T) *PostgreSQLTestContainer {
	// Try to use environment variables first (for CI/CD or existing PostgreSQL)
	if host := os.Getenv("POSTGRES_TEST_HOST"); host != "" {
		container := &PostgreSQLTestContainer{
			Host:     host,
			Port:     getEnvOrDefault("POSTGRES_TEST_PORT", "5432"),
			Database: getEnvOrDefault("POSTGRES_TEST_DB", "test_shelly_manager"),
			Username: getEnvOrDefault("POSTGRES_TEST_USER", "postgres"),
			Password: getEnvOrDefault("POSTGRES_TEST_PASSWORD", "postgres"),
		}
		container.DSN = fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
			container.Username, container.Password, container.Host, container.Port, container.Database)

		// Test connection
		if err := testPostgreSQLConnection(container.DSN); err != nil {
			t.Skip("PostgreSQL not available for testing: " + err.Error())
		}

		return container
	}

	// Skip if neither Docker nor environment is available
	t.Skip("PostgreSQL testing requires either Docker or POSTGRES_TEST_HOST environment variable")
	return nil
}

// CleanupPostgreSQLContainer cleans up the PostgreSQL container
func (c *PostgreSQLTestContainer) Cleanup(t *testing.T) {
	// For environment-based testing, we don't need to cleanup
	// For Docker containers, this would handle container cleanup
}

// CreateTestDatabase creates a fresh test database
func (c *PostgreSQLTestContainer) CreateTestDatabase(t *testing.T, dbName string) string {
	// Connect to postgres database to create test database
	adminDSN := fmt.Sprintf("postgres://%s:%s@%s:%s/postgres",
		c.Username, c.Password, c.Host, c.Port)

	db, err := sql.Open("pgx", adminDSN)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Drop database if it exists
	_, _ = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName))

	// Create database
	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
	require.NoError(t, err)

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		c.Username, c.Password, c.Host, c.Port, dbName)
}

// testPostgreSQLConnection tests if PostgreSQL is reachable
func testPostgreSQLConnection(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return db.PingContext(ctx)
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// CreateTestProvider creates a provider with test configuration
func CreateTestProvider(t *testing.T, container *PostgreSQLTestContainer) (*PostgreSQLProvider, DatabaseConfig) {
	logger := logging.GetDefault()
	provider := NewPostgreSQLProvider(logger)

	config := DatabaseConfig{
		Provider:           "postgresql",
		DSN:                container.DSN,
		MaxOpenConns:       5,
		MaxIdleConns:       2,
		ConnMaxLifetime:    time.Hour,
		ConnMaxIdleTime:    10 * time.Minute,
		SlowQueryThreshold: 200 * time.Millisecond,
		LogLevel:           "error",
		Options: map[string]string{
			"sslmode": "disable", // Use disable for testing
		},
	}

	return provider, config
}

// TestModelUser represents a test user model
type TestModelUser struct {
	ID        uint      `gorm:"primaryKey"`
	Username  string    `gorm:"uniqueIndex;not null"`
	Email     string    `gorm:"uniqueIndex;not null"`
	Password  string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TestModelProduct represents a test product model
type TestModelProduct struct {
	ID          uint   `gorm:"primaryKey"`
	Name        string `gorm:"not null"`
	Description string
	Price       float64       `gorm:"not null"`
	UserID      uint          `gorm:"not null"`
	User        TestModelUser `gorm:"foreignKey:UserID"`
	CreatedAt   time.Time     `gorm:"autoCreateTime"`
}

// CreateTestSchema creates test tables for integration tests
func CreateTestSchema(t *testing.T, provider DatabaseProvider) {
	err := provider.Migrate(&TestModelUser{}, &TestModelProduct{})
	require.NoError(t, err)
}

// CleanupTestSchema drops test tables
func CleanupTestSchema(t *testing.T, provider DatabaseProvider) {
	err := provider.DropTables(&TestModelProduct{}, &TestModelUser{})
	require.NoError(t, err)
}

// SeedTestData creates sample test data
func SeedTestData(t *testing.T, provider DatabaseProvider) {
	db := provider.GetDB()
	require.NotNil(t, db)

	// Create test users
	users := []TestModelUser{
		{Username: "alice", Email: "alice@example.com", Password: "hashed_password_1"},
		{Username: "bob", Email: "bob@example.com", Password: "hashed_password_2"},
		{Username: "charlie", Email: "charlie@example.com", Password: "hashed_password_3"},
	}

	for _, user := range users {
		err := db.Create(&user).Error
		require.NoError(t, err)
	}

	// Create test products
	var firstUser TestModelUser
	err := db.First(&firstUser).Error
	require.NoError(t, err)

	products := []TestModelProduct{
		{Name: "Product A", Description: "Description A", Price: 19.99, UserID: firstUser.ID},
		{Name: "Product B", Description: "Description B", Price: 29.99, UserID: firstUser.ID},
		{Name: "Product C", Description: "Description C", Price: 39.99, UserID: firstUser.ID},
	}

	for _, product := range products {
		err := db.Create(&product).Error
		require.NoError(t, err)
	}
}
