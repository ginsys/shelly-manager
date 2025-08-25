package shelly

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// Factory creates appropriate Shelly clients based on device generation
type Factory interface {
	// CreateClient creates a client for the device at the given IP with specific generation
	CreateClient(ip string, generation int, opts ...ClientOption) (Client, error)

	// DetectGeneration detects the generation of the device at the given IP
	DetectGeneration(ctx context.Context, ip string) (int, error)

	// CreateClientWithDetection creates a client after auto-detecting the generation
	CreateClientWithDetection(ctx context.Context, ip string, opts ...ClientOption) (Client, error)
}

// factory is the default implementation of Factory
type factory struct {
	httpClient *http.Client
	logger     *logging.Logger
}

// NewFactory creates a new device factory
func NewFactory() Factory {
	return NewFactoryWithLogger(logging.GetDefault())
}

// NewFactoryWithLogger creates a new device factory with a custom logger
func NewFactoryWithLogger(logger *logging.Logger) Factory {
	return &factory{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		logger: logger,
	}
}

// CreateClient creates a client for the specified generation
// Note: Due to import cycles, this returns an error. Use gen1.NewClient or gen2.NewClient directly.
func (f *factory) CreateClient(ip string, generation int, opts ...ClientOption) (Client, error) {
	// This method exists for interface compatibility but cannot be implemented
	// due to import cycles. Services should use DetectGeneration and then
	// create the appropriate client directly.
	return nil, fmt.Errorf("use DetectGeneration then gen1.NewClient or gen2.NewClient directly")
}

// DetectGeneration detects the device generation by probing its API
func (f *factory) DetectGeneration(ctx context.Context, ip string) (int, error) {
	f.logger.WithFields(map[string]any{
		"ip":        ip,
		"component": "shelly_factory",
	}).Debug("Detecting device generation")

	// Try Gen2+ RPC endpoint first
	gen2URL := fmt.Sprintf("http://%s/rpc/Shelly.GetDeviceInfo", ip)
	req, err := http.NewRequestWithContext(ctx, "GET", gen2URL, nil)
	if err != nil {
		return 0, err
	}

	resp, err := f.httpClient.Do(req)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer func() {
			if closeErr := resp.Body.Close(); closeErr != nil {
				// Log error if possible but continue
				_ = closeErr
			}
		}()
		var info struct {
			ID  string `json:"id"`
			Gen int    `json:"gen"`
		}
		if decodeErr := json.NewDecoder(resp.Body).Decode(&info); decodeErr == nil && info.Gen > 0 {
			f.logger.WithFields(map[string]any{
				"ip":         ip,
				"generation": info.Gen,
				"component":  "shelly_factory",
			}).Info("Detected Gen2+ device")
			return info.Gen, nil
		}
	}

	// Try Gen1 endpoint
	gen1URL := fmt.Sprintf("http://%s/shelly", ip)
	req, err = http.NewRequestWithContext(ctx, "GET", gen1URL, nil)
	if err != nil {
		return 0, err
	}

	resp, err = f.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("device not reachable at %s: %w", ip, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error if possible but continue
			_ = err
		}
	}()

	if resp.StatusCode == http.StatusOK {
		var info struct {
			Type string `json:"type"`
			MAC  string `json:"mac"`
			Auth bool   `json:"auth"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&info); err == nil && info.Type != "" {
			f.logger.WithFields(map[string]any{
				"ip":         ip,
				"generation": 1,
				"type":       info.Type,
				"component":  "shelly_factory",
			}).Info("Detected Gen1 device")
			return 1, nil
		}
	}

	return 0, ErrInvalidGeneration
}

// CreateClientWithDetection creates a client after auto-detecting the generation
func (f *factory) CreateClientWithDetection(ctx context.Context, ip string, opts ...ClientOption) (Client, error) {
	generation, err := f.DetectGeneration(ctx, ip)
	if err != nil {
		return nil, err
	}

	return f.CreateClient(ip, generation, opts...)
}

// DefaultFactory is the default factory instance
var DefaultFactory = NewFactory()
