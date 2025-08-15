package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// Collector manages periodic metrics collection
type Collector struct {
	service  *Service
	logger   *logging.Logger
	interval time.Duration

	// Control
	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// NewCollector creates a new metrics collector
func NewCollector(service *Service, logger *logging.Logger, interval time.Duration) *Collector {
	if interval <= 0 {
		interval = 5 * time.Minute // Default collection interval
	}

	return &Collector{
		service:  service,
		logger:   logger,
		interval: interval,
	}
}

// Start begins periodic metrics collection
func (c *Collector) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = true
	// Create new channels for this start cycle
	c.stopCh = make(chan struct{})
	c.doneCh = make(chan struct{})
	c.mu.Unlock()

	c.logger.WithFields(map[string]any{
		"interval":  c.interval,
		"component": "metrics_collector",
	}).Info("Starting metrics collector")

	// Start uptime counter
	c.service.StartUptimeCounter()

	// Start collection loop
	go c.collectLoop(ctx)

	return nil
}

// Stop stops the metrics collector
func (c *Collector) Stop() error {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}

	c.logger.WithFields(map[string]any{
		"component": "metrics_collector",
	}).Info("Stopping metrics collector")

	// Close stop channel if not already closed
	if c.stopCh != nil {
		close(c.stopCh)
	}
	c.mu.Unlock()

	// Wait for collection loop to finish
	if c.doneCh != nil {
		<-c.doneCh
	}

	c.mu.Lock()
	c.running = false
	c.mu.Unlock()

	c.logger.WithFields(map[string]any{
		"component": "metrics_collector",
	}).Info("Metrics collector stopped")

	return nil
}

// IsRunning returns whether the collector is currently running
func (c *Collector) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// SetInterval updates the collection interval
func (c *Collector) SetInterval(interval time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.interval = interval

	c.logger.WithFields(map[string]any{
		"new_interval": interval,
		"component":    "metrics_collector",
	}).Info("Updated metrics collection interval")
}

// GetInterval returns the current collection interval
func (c *Collector) GetInterval() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.interval
}

// collectLoop runs the periodic collection
func (c *Collector) collectLoop(ctx context.Context) {
	defer close(c.doneCh)

	// Perform initial collection
	if err := c.service.CollectMetrics(ctx); err != nil {
		c.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "metrics_collector",
		}).Error("Initial metrics collection failed")
	}

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.WithFields(map[string]any{
				"component": "metrics_collector",
			}).Info("Metrics collector context cancelled")
			return

		case <-c.stopCh:
			c.logger.WithFields(map[string]any{
				"component": "metrics_collector",
			}).Info("Metrics collector stop signal received")
			return

		case <-ticker.C:
			c.mu.RLock()
			currentInterval := c.interval
			c.mu.RUnlock()

			// Update ticker if interval changed
			if ticker.C == nil || currentInterval != c.interval {
				ticker.Stop()
				ticker = time.NewTicker(currentInterval)
			}

			// Collect metrics
			if err := c.service.CollectMetrics(ctx); err != nil {
				c.logger.WithFields(map[string]any{
					"error":     err.Error(),
					"component": "metrics_collector",
				}).Error("Periodic metrics collection failed")
			}
		}
	}
}

// TriggerCollection manually triggers a metrics collection
func (c *Collector) TriggerCollection(ctx context.Context) error {
	if !c.IsRunning() {
		return nil
	}

	return c.service.CollectMetrics(ctx)
}
