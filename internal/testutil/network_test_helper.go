package testutil

import (
	"context"
	"net"
	"os"
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

// TestNetworkAddress returns an address that no device answers and that never leaves this host: a
// closed port on the loopback, which refuses at once. A TEST-NET address (RFC 5737) is not safe for
// this: the host still sends the packet to its default gateway.
func TestNetworkAddress() string {
	return "127.0.0.1:1"
}

// IsLoopbackAddress reports whether addr, a host or a host:port, is a loopback address.
func IsLoopbackAddress(addr string) bool {
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// AssertLoopbackAddress fails the test if the address is not a loopback address, so a test that
// must send nothing off this host cannot be pointed at another host by mistake.
func AssertLoopbackAddress(t *testing.T, addr string) {
	t.Helper()
	if !IsLoopbackAddress(addr) {
		t.Fatalf("Address %s is not a loopback address - tests must send nothing off this host", addr)
	}
}
