package shelly

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/shelly/gen1"
	"github.com/ginsys/shelly-manager/internal/shelly/gen2"
)

// Factory creates appropriate Shelly clients based on device generation
type Factory interface {
	// CreateClient creates a client for the device at the given IP
	CreateClient(ip string, opts ...ClientOption) (Client, error)
	
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
func (f *factory) CreateClient(ip string, opts ...ClientOption) (Client, error) {
	// Apply options to get config
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	
	// Try to detect generation first
	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()
	
	generation, err := f.DetectGeneration(ctx, ip)
	if err != nil {
		return nil, fmt.Errorf("failed to detect device generation: %w", err)
	}
	
	return f.createClientForGeneration(ip, generation, cfg)
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
		defer resp.Body.Close()
		var info struct {
			ID  string `json:"id"`
			Gen int    `json:"gen"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&info); err == nil && info.Gen > 0 {
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
	defer resp.Body.Close()
	
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
	
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	
	return f.createClientForGeneration(ip, generation, cfg)
}

// createClientForGeneration creates the appropriate client based on generation
func (f *factory) createClientForGeneration(ip string, generation int, cfg *clientConfig) (Client, error) {
	switch generation {
	case 1:
		return gen1.NewClient(ip, cfg.ToGen1Options()...), nil
	case 2, 3: // Gen2 and Gen3 use the same RPC protocol
		return gen2.NewClient(ip, cfg.ToGen2Options()...), nil
	default:
		return nil, fmt.Errorf("unsupported device generation: %d", generation)
	}
}

// ToGen1Options converts generic options to Gen1-specific options
func (c *clientConfig) ToGen1Options() []gen1.ClientOption {
	var opts []gen1.ClientOption
	
	if c.username != "" && c.password != "" {
		opts = append(opts, gen1.WithAuth(c.username, c.password))
	}
	
	opts = append(opts, 
		gen1.WithTimeout(c.timeout),
		gen1.WithRetry(c.retryAttempts, c.retryDelay),
		gen1.WithUserAgent(c.userAgent),
	)
	
	if c.skipTLSVerify {
		opts = append(opts, gen1.WithSkipTLSVerify(true))
	}
	
	return opts
}

// ToGen2Options converts generic options to Gen2-specific options
func (c *clientConfig) ToGen2Options() []gen2.ClientOption {
	var opts []gen2.ClientOption
	
	if c.username != "" && c.password != "" {
		opts = append(opts, gen2.WithAuth(c.username, c.password))
	}
	
	opts = append(opts,
		gen2.WithTimeout(c.timeout),
		gen2.WithRetry(c.retryAttempts, c.retryDelay),
		gen2.WithUserAgent(c.userAgent),
	)
	
	if c.skipTLSVerify {
		opts = append(opts, gen2.WithSkipTLSVerify(true))
	}
	
	return opts
}

// DefaultFactory is the default factory instance
var DefaultFactory = NewFactory()