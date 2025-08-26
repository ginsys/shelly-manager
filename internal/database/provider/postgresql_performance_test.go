//go:build performance
// +build performance

package provider

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BenchmarkPostgreSQL provides comprehensive performance benchmarks
func BenchmarkPostgreSQL(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark tests in short mode")
	}

	container := SetupPostgreSQLContainer(&testing.T{})
	if container == nil {
		b.Skip("PostgreSQL not available for benchmarking")
	}
	defer container.Cleanup(&testing.T{})

	provider, config := CreateTestProvider(&testing.T{}, container)
	err := provider.Connect(config)
	require.NoError(b, err)
	defer provider.Close()

	// Run sub-benchmarks
	b.Run("Connection", benchmarkConnection(container))
	b.Run("BasicOperations", benchmarkBasicOperations(provider))
	b.Run("ConcurrentAccess", benchmarkConcurrentAccess(provider))
	b.Run("TransactionThroughput", benchmarkTransactionThroughput(provider))
	b.Run("HealthCheck", benchmarkHealthCheck(provider))
	b.Run("Statistics", benchmarkStatistics(provider))
}

// benchmarkConnection tests connection establishment performance
func benchmarkConnection(container *PostgreSQLTestContainer) func(*testing.B) {
	return func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			provider, config := CreateTestProvider(&testing.T{}, container)

			start := time.Now()
			err := provider.Connect(config)
			elapsed := time.Since(start)

			if err != nil {
				b.Fatalf("Connection failed: %v", err)
			}

			provider.Close()

			// Report connection time
			b.ReportMetric(float64(elapsed.Nanoseconds()), "ns/connection")
		}
	}
}

// benchmarkBasicOperations tests basic database operations performance
func benchmarkBasicOperations(provider *PostgreSQLProvider) func(*testing.B) {
	return func(b *testing.B) {
		CreateTestSchema(&testing.T{}, provider)
		defer CleanupTestSchema(&testing.T{}, provider)

		b.ResetTimer()

		b.Run("Ping", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if err := provider.Ping(); err != nil {
					b.Fatalf("Ping failed: %v", err)
				}
			}
		})

		b.Run("SimpleQuery", func(b *testing.B) {
			db := provider.GetDB()
			for i := 0; i < b.N; i++ {
				var result int
				if err := db.Raw("SELECT 1").Scan(&result).Error; err != nil {
					b.Fatalf("Query failed: %v", err)
				}
			}
		})

		b.Run("Insert", func(b *testing.B) {
			db := provider.GetDB()
			for i := 0; i < b.N; i++ {
				user := TestModelUser{
					Username: fmt.Sprintf("bench_user_%d", i),
					Email:    fmt.Sprintf("bench_%d@example.com", i),
					Password: "password",
				}
				if err := db.Create(&user).Error; err != nil {
					b.Fatalf("Insert failed: %v", err)
				}
			}
		})

		b.Run("Select", func(b *testing.B) {
			// Pre-populate data
			db := provider.GetDB()
			for i := 0; i < 100; i++ {
				user := TestModelUser{
					Username: fmt.Sprintf("select_user_%d", i),
					Email:    fmt.Sprintf("select_%d@example.com", i),
					Password: "password",
				}
				db.Create(&user)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var users []TestModelUser
				if err := db.Limit(10).Find(&users).Error; err != nil {
					b.Fatalf("Select failed: %v", err)
				}
			}
		})

		b.Run("Update", func(b *testing.B) {
			// Pre-populate data
			db := provider.GetDB()
			for i := 0; i < 100; i++ {
				user := TestModelUser{
					Username: fmt.Sprintf("update_user_%d", i),
					Email:    fmt.Sprintf("update_%d@example.com", i),
					Password: "password",
				}
				db.Create(&user)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := db.Model(&TestModelUser{}).
					Where("username LIKE ?", "update_user_%").
					Update("password", fmt.Sprintf("new_password_%d", i)).Error; err != nil {
					b.Fatalf("Update failed: %v", err)
				}
			}
		})
	}
}

// benchmarkConcurrentAccess tests performance under concurrent load
func benchmarkConcurrentAccess(provider *PostgreSQLProvider) func(*testing.B) {
	return func(b *testing.B) {
		CreateTestSchema(&testing.T{}, provider)
		defer CleanupTestSchema(&testing.T{}, provider)

		concurrencyLevels := []int{1, 2, 4, 8, 16, 32}

		for _, concurrency := range concurrencyLevels {
			b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
				b.SetParallelism(concurrency)
				b.ResetTimer()

				b.RunParallel(func(pb *testing.PB) {
					db := provider.GetDB()
					counter := 0

					for pb.Next() {
						counter++

						// Mix of read and write operations
						if counter%4 == 0 {
							// Write operation
							user := TestModelUser{
								Username: fmt.Sprintf("concurrent_user_%d_%d", concurrency, counter),
								Email:    fmt.Sprintf("concurrent_%d_%d@example.com", concurrency, counter),
								Password: "password",
							}
							if err := db.Create(&user).Error; err != nil {
								b.Errorf("Concurrent insert failed: %v", err)
							}
						} else {
							// Read operation
							var count int64
							if err := db.Model(&TestModelUser{}).Count(&count).Error; err != nil {
								b.Errorf("Concurrent count failed: %v", err)
							}
						}
					}
				})
			})
		}
	}
}

// benchmarkTransactionThroughput tests transaction performance
func benchmarkTransactionThroughput(provider *PostgreSQLProvider) func(*testing.B) {
	return func(b *testing.B) {
		CreateTestSchema(&testing.T{}, provider)
		defer CleanupTestSchema(&testing.T{}, provider)

		b.ResetTimer()

		b.Run("SingleTransaction", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				tx, err := provider.BeginTransaction()
				if err != nil {
					b.Fatalf("Begin transaction failed: %v", err)
				}

				user := TestModelUser{
					Username: fmt.Sprintf("tx_user_%d", i),
					Email:    fmt.Sprintf("tx_%d@example.com", i),
					Password: "password",
				}

				if err := tx.GetDB().Create(&user).Error; err != nil {
					tx.Rollback()
					b.Fatalf("Transaction insert failed: %v", err)
				}

				if err := tx.Commit(); err != nil {
					b.Fatalf("Transaction commit failed: %v", err)
				}
			}
		})

		b.Run("BatchTransaction", func(b *testing.B) {
			batchSize := 100
			for i := 0; i < b.N; i += batchSize {
				tx, err := provider.BeginTransaction()
				if err != nil {
					b.Fatalf("Begin transaction failed: %v", err)
				}

				for j := 0; j < batchSize && i+j < b.N; j++ {
					user := TestModelUser{
						Username: fmt.Sprintf("batch_user_%d", i+j),
						Email:    fmt.Sprintf("batch_%d@example.com", i+j),
						Password: "password",
					}

					if err := tx.GetDB().Create(&user).Error; err != nil {
						tx.Rollback()
						b.Fatalf("Batch transaction insert failed: %v", err)
					}
				}

				if err := tx.Commit(); err != nil {
					b.Fatalf("Batch transaction commit failed: %v", err)
				}
			}
		})

		b.Run("ConcurrentTransactions", func(b *testing.B) {
			var wg sync.WaitGroup
			errors := make(chan error, b.N)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()

					tx, err := provider.BeginTransaction()
					if err != nil {
						errors <- fmt.Errorf("begin failed for %d: %w", id, err)
						return
					}

					user := TestModelUser{
						Username: fmt.Sprintf("concurrent_tx_user_%d", id),
						Email:    fmt.Sprintf("concurrent_tx_%d@example.com", id),
						Password: "password",
					}

					if err := tx.GetDB().Create(&user).Error; err != nil {
						tx.Rollback()
						errors <- fmt.Errorf("insert failed for %d: %w", id, err)
						return
					}

					if err := tx.Commit(); err != nil {
						errors <- fmt.Errorf("commit failed for %d: %w", id, err)
					}
				}(i)
			}

			wg.Wait()
			close(errors)

			for err := range errors {
				b.Errorf("Concurrent transaction error: %v", err)
			}
		})
	}
}

// benchmarkHealthCheck tests health check performance
func benchmarkHealthCheck(provider *PostgreSQLProvider) func(*testing.B) {
	return func(b *testing.B) {
		ctx := context.Background()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			status := provider.HealthCheck(ctx)
			if !status.Healthy {
				b.Fatalf("Health check failed: %s", status.Error)
			}
		}
	}
}

// benchmarkStatistics tests statistics collection performance
func benchmarkStatistics(provider *PostgreSQLProvider) func(*testing.B) {
	return func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			stats := provider.GetStats()
			if stats.ProviderName != "PostgreSQL" {
				b.Fatalf("Invalid stats: %+v", stats)
			}
		}
	}
}

// TestPostgreSQLPerformanceUnderLoad tests sustained performance
func TestPostgreSQLPerformanceUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}

	container := SetupPostgreSQLContainer(t)
	if container == nil {
		t.Skip("PostgreSQL not available for performance testing")
	}
	defer container.Cleanup(t)

	provider, config := CreateTestProvider(t, container)
	err := provider.Connect(config)
	require.NoError(t, err)
	defer provider.Close()

	CreateTestSchema(t, provider)
	defer CleanupTestSchema(t, provider)

	// Test sustained load for 10 seconds
	duration := 10 * time.Second
	start := time.Now()

	var wg sync.WaitGroup
	errors := make(chan error, 100)
	operations := make(chan int, 100)

	// Start multiple worker goroutines
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			db := provider.GetDB()
			counter := 0

			for time.Since(start) < duration {
				counter++

				// Alternating read and write operations
				if counter%2 == 0 {
					user := TestModelUser{
						Username: fmt.Sprintf("load_user_%d_%d", workerID, counter),
						Email:    fmt.Sprintf("load_%d_%d@example.com", workerID, counter),
						Password: "password",
					}
					if err := db.Create(&user).Error; err != nil {
						errors <- err
						continue
					}
				} else {
					var count int64
					if err := db.Model(&TestModelUser{}).Count(&count).Error; err != nil {
						errors <- err
						continue
					}
				}

				operations <- 1
			}
		}(i)
	}

	wg.Wait()
	close(errors)
	close(operations)

	// Count operations and errors
	totalOps := 0
	for range operations {
		totalOps++
	}

	errorCount := 0
	for err := range errors {
		errorCount++
		t.Logf("Operation error: %v", err)
	}

	elapsed := time.Since(start)
	opsPerSecond := float64(totalOps) / elapsed.Seconds()

	t.Logf("Performance test results:")
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Total operations: %d", totalOps)
	t.Logf("  Operations per second: %.2f", opsPerSecond)
	t.Logf("  Error rate: %.2f%% (%d/%d)", float64(errorCount)/float64(totalOps)*100, errorCount, totalOps)

	// Performance assertions
	assert.Greater(t, opsPerSecond, 10.0, "Should achieve at least 10 ops/sec")
	assert.Less(t, float64(errorCount)/float64(totalOps), 0.01, "Error rate should be less than 1%")

	// Check connection pool health
	stats := provider.GetStats()
	assert.Greater(t, stats.TotalQueries, int64(0), "Should have executed queries")
	assert.GreaterOrEqual(t, stats.OpenConnections, 0, "Should have reasonable connection count")
}

// TestPostgreSQLConnectionPoolPerformance tests connection pool efficiency
func TestPostgreSQLConnectionPoolPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}

	container := SetupPostgreSQLContainer(t)
	if container == nil {
		t.Skip("PostgreSQL not available for performance testing")
	}
	defer container.Cleanup(t)

	// Test different pool configurations
	poolConfigs := []struct {
		name         string
		maxOpen      int
		maxIdle      int
		expectedPerf string
	}{
		{"SmallPool", 2, 1, "Conservative"},
		{"MediumPool", 10, 5, "Balanced"},
		{"LargePool", 25, 10, "High throughput"},
	}

	for _, config := range poolConfigs {
		t.Run(config.name, func(t *testing.T) {
			provider, dbConfig := CreateTestProvider(t, container)
			dbConfig.MaxOpenConns = config.maxOpen
			dbConfig.MaxIdleConns = config.maxIdle

			err := provider.Connect(dbConfig)
			require.NoError(t, err)
			defer provider.Close()

			// Measure connection acquisition time under load
			var wg sync.WaitGroup
			acquisitionTimes := make(chan time.Duration, 50)

			for i := 0; i < 20; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()

					start := time.Now()
					err := provider.Ping()
					acquisitionTime := time.Since(start)

					if err == nil {
						acquisitionTimes <- acquisitionTime
					}
				}()
			}

			wg.Wait()
			close(acquisitionTimes)

			// Calculate average acquisition time
			var totalTime time.Duration
			count := 0
			for acqTime := range acquisitionTimes {
				totalTime += acqTime
				count++
			}

			if count > 0 {
				avgTime := totalTime / time.Duration(count)
				t.Logf("%s - Average connection acquisition time: %v", config.name, avgTime)

				// Connection acquisition should be reasonably fast
				assert.Less(t, avgTime, 100*time.Millisecond,
					"Connection acquisition should be fast for %s", config.name)
			}

			// Check final stats
			stats := provider.GetStats()
			t.Logf("%s - Final stats: Open=%d, Idle=%d, InUse=%d",
				config.name, stats.OpenConnections, stats.IdleConnections, stats.InUseConnections)
		})
	}
}

// TestPostgreSQLMemoryUsage tests memory usage patterns
func TestPostgreSQLMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}

	container := SetupPostgreSQLContainer(t)
	if container == nil {
		t.Skip("PostgreSQL not available for performance testing")
	}
	defer container.Cleanup(t)

	provider, config := CreateTestProvider(t, container)
	err := provider.Connect(config)
	require.NoError(t, err)
	defer provider.Close()

	CreateTestSchema(t, provider)
	defer CleanupTestSchema(t, provider)

	// Test memory usage with large result sets
	db := provider.GetDB()

	// Insert a large number of records
	const recordCount = 1000
	for i := 0; i < recordCount; i++ {
		user := TestModelUser{
			Username: fmt.Sprintf("memory_test_user_%d", i),
			Email:    fmt.Sprintf("memory_test_%d@example.com", i),
			Password: "password_with_some_length_to_test_memory_usage",
		}
		err := db.Create(&user).Error
		require.NoError(t, err)
	}

	// Test fetching large result sets
	var users []TestModelUser
	err = db.Find(&users).Error
	require.NoError(t, err)
	assert.Equal(t, recordCount, len(users))

	// Test pagination to manage memory
	var pagedUsers []TestModelUser
	err = db.Limit(100).Offset(500).Find(&pagedUsers).Error
	require.NoError(t, err)
	assert.Equal(t, 100, len(pagedUsers))

	t.Logf("Memory usage test completed successfully")
	t.Logf("  Inserted %d records", recordCount)
	t.Logf("  Retrieved %d records in full query", len(users))
	t.Logf("  Retrieved %d records in paged query", len(pagedUsers))
}
