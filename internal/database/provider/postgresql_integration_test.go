//go:build integration
// +build integration

package provider

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// PostgreSQLIntegrationTestSuite provides comprehensive integration testing
type PostgreSQLIntegrationTestSuite struct {
	suite.Suite
	container *PostgreSQLTestContainer
	provider  *PostgreSQLProvider
	config    DatabaseConfig
}

// SetupSuite initializes the test suite
func (suite *PostgreSQLIntegrationTestSuite) SetupSuite() {
	suite.container = SetupPostgreSQLContainer(suite.T())
	if suite.container == nil {
		suite.T().Skip("PostgreSQL container not available")
	}
}

// SetupTest prepares each test
func (suite *PostgreSQLIntegrationTestSuite) SetupTest() {
	suite.provider, suite.config = CreateTestProvider(suite.T(), suite.container)
}

// TearDownTest cleans up after each test
func (suite *PostgreSQLIntegrationTestSuite) TearDownTest() {
	if suite.provider != nil {
		_ = suite.provider.Close()
	}
}

// TearDownSuite cleans up the test suite
func (suite *PostgreSQLIntegrationTestSuite) TearDownSuite() {
	if suite.container != nil {
		suite.container.Cleanup(suite.T())
	}
}

// TestPostgreSQLIntegrationFull runs the full integration test suite
func TestPostgreSQLIntegrationFull(t *testing.T) {
	suite.Run(t, new(PostgreSQLIntegrationTestSuite))
}

// Test full connection lifecycle
func (suite *PostgreSQLIntegrationTestSuite) TestConnectionLifecycle() {
	// Test connection
	err := suite.provider.Connect(suite.config)
	suite.Require().NoError(err)

	// Verify connection is active
	err = suite.provider.Ping()
	suite.NoError(err)

	// Test double connect fails
	err = suite.provider.Connect(suite.config)
	suite.Error(err)
	suite.Contains(err.Error(), "already connected")

	// Test close
	err = suite.provider.Close()
	suite.NoError(err)

	// Test operations after close fail
	err = suite.provider.Ping()
	suite.Error(err)

	// Test double close doesn't error
	err = suite.provider.Close()
	suite.NoError(err)
}

// Test comprehensive migration scenarios
func (suite *PostgreSQLIntegrationTestSuite) TestMigrationScenarios() {
	err := suite.provider.Connect(suite.config)
	suite.Require().NoError(err)

	// Test initial migration
	CreateTestSchema(suite.T(), suite.provider)
	defer CleanupTestSchema(suite.T(), suite.provider)

	// Verify tables exist by inserting data
	SeedTestData(suite.T(), suite.provider)

	// Test migration with additional columns (schema evolution)
	type ExtendedUser struct {
		TestModelUser
		Bio       string `gorm:"type:text"`
		LastLogin *time.Time
	}

	err = suite.provider.Migrate(&ExtendedUser{})
	suite.NoError(err)

	// Verify new columns exist
	db := suite.provider.GetDB()
	var result ExtendedUser
	err = db.First(&result).Error
	suite.NoError(err)

	// Test that we can update new columns
	result.Bio = "Test bio"
	result.LastLogin = &time.Time{}
	*result.LastLogin = time.Now()
	err = db.Save(&result).Error
	suite.NoError(err)
}

// Test transaction isolation levels and rollback scenarios
func (suite *PostgreSQLIntegrationTestSuite) TestTransactionIsolation() {
	err := suite.provider.Connect(suite.config)
	suite.Require().NoError(err)

	CreateTestSchema(suite.T(), suite.provider)
	defer CleanupTestSchema(suite.T(), suite.provider)

	// Test concurrent transactions
	var wg sync.WaitGroup
	errors := make(chan error, 2)

	// Transaction 1: Insert user
	wg.Add(1)
	go func() {
		defer wg.Done()
		tx, err := suite.provider.BeginTransaction()
		if err != nil {
			errors <- err
			return
		}

		user := TestModelUser{
			Username: "tx_test_user",
			Email:    "tx@example.com",
			Password: "password",
		}

		if err := tx.GetDB().Create(&user).Error; err != nil {
			tx.Rollback()
			errors <- err
			return
		}

		// Simulate work
		time.Sleep(100 * time.Millisecond)

		if err := tx.Commit(); err != nil {
			errors <- err
		}
	}()

	// Transaction 2: Try to insert same user (should fail after tx1 commits)
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond) // Start after tx1

		tx, err := suite.provider.BeginTransaction()
		if err != nil {
			errors <- err
			return
		}

		user := TestModelUser{
			Username: "tx_test_user", // Same username
			Email:    "tx2@example.com",
			Password: "password2",
		}

		if err := tx.GetDB().Create(&user).Error; err != nil {
			tx.Rollback()
			// This error is expected due to unique constraint
			return
		}

		if err := tx.Commit(); err != nil {
			errors <- err
		}
	}()

	wg.Wait()
	close(errors)

	// Check for unexpected errors (constraint violations are expected)
	for err := range errors {
		if err != nil {
			suite.Contains(err.Error(), "duplicate key") // PostgreSQL constraint error
		}
	}

	// Verify only one user was created
	var count int64
	err = suite.provider.GetDB().Model(&TestModelUser{}).
		Where("username = ?", "tx_test_user").Count(&count).Error
	suite.NoError(err)
	suite.Equal(int64(1), count)
}

// Test connection pool exhaustion and recovery
func (suite *PostgreSQLIntegrationTestSuite) TestConnectionPoolExhaustion() {
	// Use a small connection pool for testing
	config := suite.config
	config.MaxOpenConns = 3
	config.MaxIdleConns = 1

	err := suite.provider.Connect(config)
	suite.Require().NoError(err)

	CreateTestSchema(suite.T(), suite.provider)
	defer CleanupTestSchema(suite.T(), suite.provider)

	// Start more transactions than pool size
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			tx, err := suite.provider.BeginTransaction()
			if err != nil {
				errors <- fmt.Errorf("tx %d begin failed: %w", id, err)
				return
			}

			// Hold transaction for a short time
			time.Sleep(200 * time.Millisecond)

			user := TestModelUser{
				Username: fmt.Sprintf("pool_test_%d", id),
				Email:    fmt.Sprintf("pool_test_%d@example.com", id),
				Password: "password",
			}

			if err := tx.GetDB().Create(&user).Error; err != nil {
				tx.Rollback()
				errors <- fmt.Errorf("tx %d create failed: %w", id, err)
				return
			}

			if err := tx.Commit(); err != nil {
				errors <- fmt.Errorf("tx %d commit failed: %w", id, err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// All operations should succeed (may be queued)
	for err := range errors {
		suite.NoError(err, "Connection pool should handle all requests")
	}

	// Verify all users were created
	var count int64
	err = suite.provider.GetDB().Model(&TestModelUser{}).
		Where("username LIKE ?", "pool_test_%").Count(&count).Error
	suite.NoError(err)
	suite.Equal(int64(8), count)
}

// Test performance under load
func (suite *PostgreSQLIntegrationTestSuite) TestPerformanceUnderLoad() {
	err := suite.provider.Connect(suite.config)
	suite.Require().NoError(err)

	CreateTestSchema(suite.T(), suite.provider)
	defer CleanupTestSchema(suite.T(), suite.provider)

	// Seed initial data
	SeedTestData(suite.T(), suite.provider)

	start := time.Now()
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent reads and writes
	for i := 0; i < 50; i++ {
		// Read operations
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			var users []TestModelUser
			if err := suite.provider.GetDB().Limit(10).Find(&users).Error; err != nil {
				errors <- err
			}

			var count int64
			if err := suite.provider.GetDB().Model(&TestModelProduct{}).Count(&count).Error; err != nil {
				errors <- err
			}
		}(i)

		// Write operations
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			product := TestModelProduct{
				Name:        fmt.Sprintf("Load Test Product %d", id),
				Description: fmt.Sprintf("Description for product %d", id),
				Price:       float64(id * 10),
				UserID:      1, // Assume first user exists
			}

			if err := suite.provider.GetDB().Create(&product).Error; err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)
	duration := time.Since(start)

	// Check for errors
	for err := range errors {
		suite.NoError(err)
	}

	// Performance should be reasonable
	suite.Less(duration, 30*time.Second, "Operations should complete within 30 seconds")

	// Check statistics
	stats := suite.provider.GetStats()
	suite.Greater(stats.TotalQueries, int64(0))
	suite.GreaterOrEqual(stats.OpenConnections, 0)
}

// Test SSL/TLS connection security
func (suite *PostgreSQLIntegrationTestSuite) TestSSLSecurity() {
	// Test different SSL modes if SSL is configured
	sslModes := []string{"disable", "prefer"}

	for _, sslMode := range sslModes {
		suite.Run(fmt.Sprintf("SSL_Mode_%s", sslMode), func() {
			provider, config := CreateTestProvider(suite.T(), suite.container)
			config.Options["sslmode"] = sslMode

			err := provider.Connect(config)
			defer provider.Close()

			// Connection should succeed for both modes in test environment
			suite.NoError(err)

			// Test that connection is functional
			err = provider.Ping()
			suite.NoError(err)

			// Get connection stats
			stats := provider.GetStats()
			suite.Equal("PostgreSQL", stats.ProviderName)
		})
	}
}

// Test health monitoring and alerts
func (suite *PostgreSQLIntegrationTestSuite) TestHealthMonitoring() {
	err := suite.provider.Connect(suite.config)
	suite.Require().NoError(err)

	// Test healthy state
	ctx := context.Background()
	status := suite.provider.HealthCheck(ctx)

	suite.True(status.Healthy)
	suite.Empty(status.Error)
	suite.NotZero(status.ResponseTime)
	suite.NotZero(status.CheckedAt)

	// Verify health details
	suite.Contains(status.Details, "database_size")
	suite.Contains(status.Details, "connection_count")
	suite.Contains(status.Details, "version")

	// Test health check with timeout
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	status = suite.provider.HealthCheck(ctxWithTimeout)
	suite.True(status.Healthy) // Should complete within timeout

	// Test multiple concurrent health checks
	var wg sync.WaitGroup
	healthResults := make(chan HealthStatus, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status := suite.provider.HealthCheck(ctx)
			healthResults <- status
		}()
	}

	wg.Wait()
	close(healthResults)

	// All health checks should succeed
	for status := range healthResults {
		suite.True(status.Healthy)
	}
}

// Test backup and recovery patterns (if implemented)
func (suite *PostgreSQLIntegrationTestSuite) TestDataConsistency() {
	err := suite.provider.Connect(suite.config)
	suite.Require().NoError(err)

	CreateTestSchema(suite.T(), suite.provider)
	defer CleanupTestSchema(suite.T(), suite.provider)

	// Create test data
	SeedTestData(suite.T(), suite.provider)

	// Verify data integrity with foreign key relationships
	db := suite.provider.GetDB()

	var products []TestModelProduct
	err = db.Preload("User").Find(&products).Error
	suite.NoError(err)
	suite.NotEmpty(products)

	for _, product := range products {
		suite.NotZero(product.UserID)
		suite.NotEmpty(product.User.Username)
		suite.NotEmpty(product.User.Email)
	}

	// Test cascading deletes (if configured)
	var firstUser TestModelUser
	err = db.First(&firstUser).Error
	suite.NoError(err)

	// Count products before deletion
	var productCountBefore int64
	err = db.Model(&TestModelProduct{}).Where("user_id = ?", firstUser.ID).Count(&productCountBefore).Error
	suite.NoError(err)
	suite.Greater(productCountBefore, int64(0))

	// Delete user (products should remain due to foreign key constraint)
	err = db.Delete(&firstUser).Error
	// This should fail due to foreign key constraint
	suite.Error(err)

	// Verify products still exist
	var productCountAfter int64
	err = db.Model(&TestModelProduct{}).Where("user_id = ?", firstUser.ID).Count(&productCountAfter).Error
	suite.NoError(err)
	suite.Equal(productCountBefore, productCountAfter)
}
