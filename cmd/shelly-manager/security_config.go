package main

import (
	"github.com/ginsys/shelly-manager/internal/api/middleware"
	"github.com/ginsys/shelly-manager/internal/config"
)

// securityConfigFrom builds the middleware's security configuration from the application's: the
// proxy and CORS settings. A nil configuration leaves the middleware defaults.
func securityConfigFrom(cfg *config.Config) *middleware.SecurityConfig {
	secCfg := middleware.DefaultSecurityConfig()
	if cfg == nil {
		return secCfg
	}

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
