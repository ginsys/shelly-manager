package testutil

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// NetworkTestConfig controls network test behavior
type NetworkTestConfig struct {
	SkipInShort    bool
	DefaultTimeout time.Duration
	QuickTimeout   time.Duration
}

// DefaultNetworkConfig returns sensible defaults for network testing
func DefaultNetworkConfig() NetworkTestConfig {
	return NetworkTestConfig{
		SkipInShort:    true,
		DefaultTimeout: 5 * time.Second,
		QuickTimeout:   100 * time.Millisecond,
	}
}

// ShouldSkipNetworkTest determines if network tests should be skipped
func ShouldSkipNetworkTest(t *testing.T, config NetworkTestConfig) bool {
	t.Helper()

	// Check environment variable override
	if os.Getenv("SHELLY_FORCE_NETWORK_TESTS") == "1" {
		return false
	}

	// Skip network tests in short mode unless explicitly enabled
	if config.SkipInShort && testing.Short() {
		return true
	}

	// Check for CI environment
	if os.Getenv("CI") == "true" {
		// In CI, only run network tests if explicitly enabled
		return os.Getenv("SHELLY_CI_NETWORK_TESTS") != "1"
	}

	return false
}

// SkipNetworkTestIfNeeded is a helper that skips network tests based on configuration
func SkipNetworkTestIfNeeded(t *testing.T, config NetworkTestConfig) {
	t.Helper()

	if ShouldSkipNetworkTest(t, config) {
		t.Skip("Skipping network test (use SHELLY_FORCE_NETWORK_TESTS=1 to enable)")
	}
}

// CreateNetworkTestContext creates a context with appropriate timeout for network tests
func CreateNetworkTestContext(config NetworkTestConfig) (context.Context, context.CancelFunc) {
	timeout := config.DefaultTimeout

	// Use shorter timeout in short mode for faster feedback
	if testing.Short() {
		timeout = config.QuickTimeout
	}

	// Allow environment override
	if envTimeout := os.Getenv("SHELLY_NETWORK_TEST_TIMEOUT"); envTimeout != "" {
		if parsedTimeout, err := time.ParseDuration(envTimeout); err == nil {
			timeout = parsedTimeout
		}
	}

	return context.WithTimeout(context.Background(), timeout)
}

// TestNetworkAddress returns safe test addresses for network tests
func TestNetworkAddress() string {
	// Use TEST-NET-1 address range (RFC 3330) - guaranteed to be non-routable
	return "192.0.2.1"
}

// TestNetworkCIDR returns a safe CIDR range for network tests
func TestNetworkCIDR() string {
	// Use small TEST-NET-1 range for fast tests
	return "192.0.2.0/30" // Only 4 addresses
}

// IsTestNetAddress validates that an address is in a safe TEST-NET range
func IsTestNetAddress(addr string) bool {
	// TEST-NET-1: 192.0.2.0/24
	// TEST-NET-2: 198.51.100.0/24
	// TEST-NET-3: 203.0.113.0/24
	testNetPrefixes := []string{
		"192.0.2.",
		"198.51.100.",
		"203.0.113.",
	}

	for _, prefix := range testNetPrefixes {
		if strings.HasPrefix(addr, prefix) {
			return true
		}
	}
	return false
}

// AssertTestNetAddress fails the test if the address is not in TEST-NET range
func AssertTestNetAddress(t *testing.T, addr string) {
	t.Helper()
	if !IsTestNetAddress(addr) {
		t.Fatalf("Address %s is not in TEST-NET range - tests should only use RFC 5737 TEST-NET addresses", addr)
	}
}

// LogTestNetUsage logs that a test is using TEST-NET addresses for transparency
func LogTestNetUsage(t *testing.T, addr string) {
	t.Helper()
	if IsTestNetAddress(addr) {
		t.Logf("Using safe TEST-NET address: %s (RFC 5737 - no real network traffic)", addr)
	}
}
