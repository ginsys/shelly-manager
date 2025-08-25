package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

func setupTestCollector(t *testing.T) (*Collector, *Service) {
	t.Helper()

	service, _ := setupTestService(t)
	logger := logging.GetDefault()
	collector := NewCollector(service, logger, time.Second)

	return collector, service
}

func TestNewCollector(t *testing.T) {
	service, _ := setupTestService(t)
	logger := logging.GetDefault()

	collector := NewCollector(service, logger, 5*time.Second)

	if collector == nil {
		t.Fatal("NewCollector returned nil")
	}

	if collector.service != service {
		t.Error("Collector service not set correctly")
	}

	if collector.logger != logger {
		t.Error("Collector logger not set correctly")
	}

	if collector.interval != 5*time.Second {
		t.Errorf("Expected interval 5s, got %v", collector.interval)
	}

	if collector.running {
		t.Error("Collector should not be running initially")
	}
}

func TestNewCollectorWithZeroInterval(t *testing.T) {
	service, _ := setupTestService(t)
	logger := logging.GetDefault()

	collector := NewCollector(service, logger, 0)

	if collector.interval != 5*time.Minute {
		t.Errorf("Expected default interval 5m, got %v", collector.interval)
	}
}

func TestNewCollectorWithNegativeInterval(t *testing.T) {
	service, _ := setupTestService(t)
	logger := logging.GetDefault()

	collector := NewCollector(service, logger, -1*time.Second)

	if collector.interval != 5*time.Minute {
		t.Errorf("Expected default interval 5m, got %v", collector.interval)
	}
}

func TestCollectorIsRunning(t *testing.T) {
	collector, _ := setupTestCollector(t)

	// Should not be running initially
	if collector.IsRunning() {
		t.Error("Collector should not be running initially")
	}

	// Start collector
	ctx := context.Background()
	err := collector.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start collector: %v", err)
	}

	// Should be running now
	if !collector.IsRunning() {
		t.Error("Collector should be running after Start()")
	}

	// Stop collector
	err = collector.Stop()
	if err != nil {
		t.Logf("Failed to stop collector: %v", err)
	}
	if err != nil {
		t.Fatalf("Failed to stop collector: %v", err)
	}

	// Should not be running after stop
	if collector.IsRunning() {
		t.Error("Collector should not be running after Stop()")
	}
}

func TestCollectorStartTwice(t *testing.T) {
	collector, _ := setupTestCollector(t)

	ctx := context.Background()

	// Start first time
	err := collector.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start collector first time: %v", err)
	}

	// Start second time should be no-op
	err = collector.Start(ctx)
	if err != nil {
		t.Errorf("Starting already running collector should not error: %v", err)
	}

	if !collector.IsRunning() {
		t.Error("Collector should still be running after second start")
	}

	// Clean up
	if err := collector.Stop(); err != nil {
		t.Logf("Failed to stop collector: %v", err)
	}
}

func TestCollectorStopTwice(t *testing.T) {
	collector, _ := setupTestCollector(t)

	ctx := context.Background()
	err := collector.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start collector: %v", err)
	}

	// Stop first time
	err = collector.Stop()
	if err != nil {
		t.Logf("Failed to stop collector: %v", err)
	}
	if err != nil {
		t.Fatalf("Failed to stop collector first time: %v", err)
	}

	// Stop second time should be no-op
	err = collector.Stop()
	if err != nil {
		t.Logf("Failed to stop collector: %v", err)
	}
	if err != nil {
		t.Errorf("Stopping already stopped collector should not error: %v", err)
	}

	if collector.IsRunning() {
		t.Error("Collector should not be running after double stop")
	}
}

func TestCollectorSetInterval(t *testing.T) {
	collector, _ := setupTestCollector(t)

	originalInterval := collector.GetInterval()
	newInterval := 10 * time.Second

	collector.SetInterval(newInterval)

	if collector.GetInterval() != newInterval {
		t.Errorf("Expected interval %v, got %v", newInterval, collector.GetInterval())
	}

	if collector.GetInterval() == originalInterval {
		t.Error("Interval should have changed")
	}
}

func TestCollectorGetInterval(t *testing.T) {
	service, _ := setupTestService(t)
	logger := logging.GetDefault()
	expectedInterval := 30 * time.Second

	collector := NewCollector(service, logger, expectedInterval)

	if collector.GetInterval() != expectedInterval {
		t.Errorf("Expected interval %v, got %v", expectedInterval, collector.GetInterval())
	}
}

func TestCollectorTriggerCollection(t *testing.T) {
	collector, service := setupTestCollector(t)

	// Start collector
	ctx := context.Background()
	err := collector.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start collector: %v", err)
	}
	defer func() {
		if stopErr := collector.Stop(); stopErr != nil {
			t.Logf("Failed to stop collector: %v", stopErr)
		}
	}()

	// Get initial collection time
	initialTime := service.GetLastCollectionTime()

	// Trigger manual collection
	err = collector.TriggerCollection(ctx)
	if err != nil {
		t.Fatalf("Failed to trigger collection: %v", err)
	}

	// Should have updated collection time
	newTime := service.GetLastCollectionTime()
	if !newTime.After(initialTime) {
		t.Error("Collection time should have been updated after trigger")
	}
}

func TestCollectorTriggerCollectionNotRunning(t *testing.T) {
	collector, _ := setupTestCollector(t)

	// Don't start collector

	ctx := context.Background()
	err := collector.TriggerCollection(ctx)
	if err != nil {
		t.Errorf("TriggerCollection on stopped collector should not error: %v", err)
	}
}

func TestCollectorPeriodicCollection(t *testing.T) {
	// Use very short interval for testing
	service, _ := setupTestService(t)
	logger := logging.GetDefault()
	collector := NewCollector(service, logger, 50*time.Millisecond)

	// Start collector
	ctx := context.Background()
	err := collector.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start collector: %v", err)
	}
	defer func() {
		if stopErr := collector.Stop(); stopErr != nil {
			t.Logf("Failed to stop collector: %v", stopErr)
		}
	}()

	// Get initial collection time
	initialTime := service.GetLastCollectionTime()

	// Wait for at least 2 collection cycles (50ms * 2 + buffer)
	time.Sleep(120 * time.Millisecond)

	// Should have updated collection time
	newTime := service.GetLastCollectionTime()
	if !newTime.After(initialTime) {
		t.Error("Collection time should have been updated by periodic collection")
	}
}

func TestCollectorContextCancellation(t *testing.T) {
	collector, _ := setupTestCollector(t)

	// Create context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Start collector
	err := collector.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start collector: %v", err)
	}

	// Verify it's running
	if !collector.IsRunning() {
		t.Error("Collector should be running")
	}

	// Cancel context
	cancel()

	// Give it a moment to stop
	time.Sleep(10 * time.Millisecond) // Reduced for faster tests

	// Should still report as running (context cancellation stops loop but doesn't change running flag)
	// The running flag is only changed by explicit Stop() call
	if !collector.IsRunning() {
		t.Error("Collector should still report as running (only Stop() changes this)")
	}

	// Explicit stop should work
	err = collector.Stop()
	if err != nil {
		t.Logf("Failed to stop collector: %v", err)
	}
	if err != nil {
		t.Fatalf("Failed to stop collector: %v", err)
	}
}

func TestCollectorIntervalChange(t *testing.T) {
	// Test changing interval while running
	service, _ := setupTestService(t)
	logger := logging.GetDefault()
	collector := NewCollector(service, logger, 100*time.Millisecond)

	ctx := context.Background()
	err := collector.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start collector: %v", err)
	}
	defer func() {
		if stopErr := collector.Stop(); stopErr != nil {
			t.Logf("Failed to stop collector: %v", stopErr)
		}
	}()

	// Change interval
	collector.SetInterval(200 * time.Millisecond)

	// Verify new interval is set
	if collector.GetInterval() != 200*time.Millisecond {
		t.Errorf("Expected interval 200ms, got %v", collector.GetInterval())
	}

	// The collector should continue running with new interval
	// (testing this thoroughly would require complex timing tests)
	if !collector.IsRunning() {
		t.Error("Collector should still be running after interval change")
	}
}

func TestCollectorConcurrentOperations(t *testing.T) {
	collector, _ := setupTestCollector(t)

	ctx := context.Background()

	// Test concurrent start/stop operations
	done := make(chan bool, 4)

	// Start collector
	go func() {
		err := collector.Start(ctx)
		if err != nil {
			t.Errorf("Concurrent start failed: %v", err)
		}
		done <- true
	}()

	// Try to start again concurrently
	go func() {
		err := collector.Start(ctx)
		if err != nil {
			t.Errorf("Concurrent start failed: %v", err)
		}
		done <- true
	}()

	// Check running status
	go func() {
		time.Sleep(10 * time.Millisecond)
		_ = collector.IsRunning() // Should not race
		done <- true
	}()

	// Change interval
	go func() {
		time.Sleep(5 * time.Millisecond)
		collector.SetInterval(2 * time.Second)
		done <- true
	}()

	// Wait for all operations
	for i := 0; i < 4; i++ {
		<-done
	}

	// Should be running
	if !collector.IsRunning() {
		t.Error("Collector should be running after concurrent operations")
	}

	// Clean stop
	err := collector.Stop()
	if err != nil {
		t.Logf("Failed to stop collector: %v", err)
	}
	if err != nil {
		t.Fatalf("Failed to stop collector: %v", err)
	}
}

func TestCollectorStopTimeout(t *testing.T) {
	// Test that Stop() waits for collection loop to finish
	collector, _ := setupTestCollector(t)

	ctx := context.Background()
	err := collector.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start collector: %v", err)
	}

	// Stop should complete without hanging
	stopDone := make(chan bool, 1)
	go func() {
		err := collector.Stop()
		if err != nil {
			t.Logf("Failed to stop collector: %v", err)
		}
		if err != nil {
			t.Errorf("Stop failed: %v", err)
		}
		stopDone <- true
	}()

	// Should complete within reasonable time
	select {
	case <-stopDone:
		// Good
	case <-time.After(5 * time.Second):
		t.Error("Stop() took too long, may be hanging")
	}

	if collector.IsRunning() {
		t.Error("Collector should not be running after stop")
	}
}

func TestCollectorMemoryCleanup(t *testing.T) {
	// Test that channels are properly cleaned up
	collector, _ := setupTestCollector(t)

	ctx := context.Background()

	// Start and stop multiple times
	for i := 0; i < 3; i++ {
		err := collector.Start(ctx)
		if err != nil {
			t.Fatalf("Failed to start collector (iteration %d): %v", i, err)
		}

		err = collector.Stop()
		if err != nil {
			t.Logf("Failed to stop collector: %v", err)
		}
		if err != nil {
			t.Fatalf("Failed to stop collector (iteration %d): %v", i, err)
		}

		if collector.IsRunning() {
			t.Errorf("Collector should not be running after stop (iteration %d)", i)
		}
	}

	// Should still work after multiple start/stop cycles
	err := collector.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start collector after multiple cycles: %v", err)
	}

	if !collector.IsRunning() {
		t.Error("Collector should be running after restart")
	}

	err = collector.Stop()
	if err != nil {
		t.Logf("Failed to stop collector: %v", err)
	}
	if err != nil {
		t.Fatalf("Failed to final stop: %v", err)
	}
}
