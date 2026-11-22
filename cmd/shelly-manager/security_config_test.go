package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
	"github.com/ginsys/shelly-manager/internal/config"
)

// The mapping from the application's keys to the middleware's configuration
// (ginsys/shelly-manager#861). The config tests stop at the struct and the middleware tests start
// at SecurityConfig, so only this test shows that a key reaches its own field.

func TestSecurityConfigFromMapsEachKeyToItsField(t *testing.T) {
	cfg := &config.Config{}
	cfg.Security.Monitoring.Enabled = true
	cfg.Security.IPBlocking.Enabled = true
	cfg.Security.IPBlocking.BlockSeconds = 172800
	cfg.Security.IPBlocking.SuspiciousThreshold = 21
	cfg.Security.IPBlocking.RateLimitThreshold = 22
	cfg.Security.IPBlocking.AttackTypeThreshold = 23
	cfg.Security.IPBlocking.SuspiciousPerHourThreshold = 24

	got := securityConfigFrom(cfg)

	assert.True(t, got.EnableMonitoring)
	assert.True(t, got.EnableIPBlocking)
	assert.Equal(t, 48*time.Hour, got.BlockDuration, "seconds, not nanoseconds")
	assert.Equal(t, 21, got.SuspiciousThreshold)
	assert.Equal(t, 22, got.RateViolationThreshold)
	assert.Equal(t, 23, got.AttackTypeThreshold)
	assert.Equal(t, 24, got.HourlyThreshold)
}

func TestSecurityConfigFromKeepsTheProxyAndCORSMapping(t *testing.T) {
	cfg := &config.Config{}
	cfg.Security.UseProxyHeaders = true
	cfg.Security.TrustedProxies = []string{"10.0.0.0/8"}
	cfg.Security.CORS.AllowedOrigins = []string{"https://a.example"}
	cfg.Security.CORS.AllowedMethods = []string{"GET"}
	cfg.Security.CORS.AllowedHeaders = []string{"X-A"}
	cfg.Security.CORS.MaxAge = 60

	got := securityConfigFrom(cfg)

	assert.True(t, got.UseProxyHeaders)
	assert.Equal(t, []string{"10.0.0.0/8"}, got.TrustedProxies)
	assert.Equal(t, []string{"https://a.example"}, got.CORSAllowedOrigins)
	assert.Equal(t, []string{"GET"}, got.CORSAllowedMethods)
	assert.Equal(t, []string{"X-A"}, got.CORSAllowedHeaders)
	assert.Equal(t, 60, got.CORSMaxAge)
}

// Check 2: the configuration the deployment loads with no key set turns both off, and a nil
// configuration does too.
func TestSecurityConfigFromDefaultLoadIsOff(t *testing.T) {
	cfg := &config.Config{}
	got := securityConfigFrom(cfg)
	assert.False(t, got.EnableMonitoring)
	assert.False(t, got.EnableIPBlocking)

	got = securityConfigFrom(nil)
	assert.False(t, got.EnableMonitoring)
	assert.False(t, got.EnableIPBlocking)
}

// Check 4: turned on with no other key set, the monitor acts as it always did. The numbers live in
// config.setDefaults and in middleware.DefaultSecurityConfig, and this keeps the two equal.
func TestConfigDefaultsEqualDefaultSecurityConfig(t *testing.T) {
	loaded, err := config.Load("../../configs/shelly-manager.yaml")
	assert.NoError(t, err)

	got := securityConfigFrom(loaded)
	want := middleware.DefaultSecurityConfig()

	assert.Equal(t, want.BlockDuration, got.BlockDuration)
	assert.Equal(t, want.SuspiciousThreshold, got.SuspiciousThreshold)
	assert.Equal(t, want.RateViolationThreshold, got.RateViolationThreshold)
	assert.Equal(t, want.AttackTypeThreshold, got.AttackTypeThreshold)
	assert.Equal(t, want.HourlyThreshold, got.HourlyThreshold)
	assert.False(t, got.EnableMonitoring, "the shipped file leaves the monitor off")
	assert.False(t, got.EnableIPBlocking)
}
