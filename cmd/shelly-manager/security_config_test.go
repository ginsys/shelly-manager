package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ginsys/shelly-manager/internal/config"
)

// The config tests stop at the struct and the middleware tests start at SecurityConfig, so only this
// test shows that a proxy or CORS key reaches its own field.
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
