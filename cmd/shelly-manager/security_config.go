package main

import (
	"time"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
	"github.com/ginsys/shelly-manager/internal/config"
)

// securityConfigFrom builds the middleware's security configuration from the application's. The
// monitor and IP blocking are off unless the configuration turns them on, which is the deployment's
// default; middleware.DefaultSecurityConfig keeps both on for code that builds a router without a
// configuration. A nil configuration leaves them off.
func securityConfigFrom(cfg *config.Config) *middleware.SecurityConfig {
	secCfg := middleware.DefaultSecurityConfig()
	secCfg.EnableMonitoring = false
	secCfg.EnableIPBlocking = false
	if cfg == nil {
		return secCfg
	}

	secCfg.EnableMonitoring = cfg.Security.Monitoring.Enabled
	secCfg.EnableIPBlocking = cfg.Security.IPBlocking.Enabled
	secCfg.BlockDuration = time.Duration(cfg.Security.IPBlocking.BlockSeconds) * time.Second
	secCfg.SuspiciousThreshold = cfg.Security.IPBlocking.SuspiciousThreshold
	secCfg.RateViolationThreshold = cfg.Security.IPBlocking.RateLimitThreshold
	secCfg.AttackTypeThreshold = cfg.Security.IPBlocking.AttackTypeThreshold
	secCfg.HourlyThreshold = cfg.Security.IPBlocking.SuspiciousPerHourThreshold

	secCfg.UseProxyHeaders = cfg.Security.UseProxyHeaders
	secCfg.TrustedProxies = cfg.Security.TrustedProxies
	secCfg.CORSAllowedOrigins = cfg.Security.CORS.AllowedOrigins
	if len(cfg.Security.CORS.AllowedMethods) > 0 {
		secCfg.CORSAllowedMethods = cfg.Security.CORS.AllowedMethods
	}
	if len(cfg.Security.CORS.AllowedHeaders) > 0 {
		secCfg.CORSAllowedHeaders = cfg.Security.CORS.AllowedHeaders
	}
	if cfg.Security.CORS.MaxAge > 0 {
		secCfg.CORSMaxAge = cfg.Security.CORS.MaxAge
	}
	return secCfg
}
