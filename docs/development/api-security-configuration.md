# API Security Configuration and Deployment Guide

## Executive Summary

This guide provides comprehensive configuration instructions for deploying the Shelly Manager API Security Framework across development, staging, and production environments. The framework supports environment-specific configurations while maintaining consistent security postures across all deployments.

### Configuration Overview

| Environment | Security Level | Performance Target | Monitoring Level | Recommended Use |
|-------------|---------------|-------------------|------------------|-----------------|
| Development | Medium | <15ms | Basic | Local development, unit testing |
| Staging | High | <12ms | Enhanced | Integration testing, security testing |
| Production | Maximum | <10ms | Comprehensive | Live deployment, user traffic |

## Security Configuration

### Default Security Configuration

The framework provides secure defaults that can be customized for specific environments:

```go
func DefaultSecurityConfig() *SecurityConfig {
    return &SecurityConfig{
        // Content Security Policy
        CSP: "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self';",
        
        // Rate limiting
        RateLimit:       1000,           // requests per window
        RateLimitWindow: time.Hour,      // 1 hour window
        RateLimitByPath: map[string]int{
            "/api/v1/devices/{id}/control": 100, // Device control endpoints
            "/api/v1/provisioning":         50,  // Provisioning endpoints
            "/api/v1/config/bulk":          20,  // Bulk operations
        },
        
        // Request limits
        MaxRequestSize:    10 * 1024 * 1024, // 10MB
        RequestTimeout:    30 * time.Second, // 30 seconds
        
        // Security headers
        EnableHSTS:        false,        // Enable for HTTPS
        HSTSMaxAge:        31536000,     // 1 year
        PermissionsPolicy: "geolocation=(), camera=(), microphone=(), payment=()",
        
        // Monitoring and logging
        LogSecurityEvents: true,
        LogAllRequests:    false,        // Enable for debugging
        EnableMonitoring:  true,
        
        // Attack detection
        EnableIPBlocking:  true,
        BlockDuration:     time.Hour,    // Block for 1 hour
    }
}
```

### Environment-Specific Configurations

#### Development Environment Configuration

Optimized for development productivity while maintaining basic security:

```go
func DevelopmentSecurityConfig() *SecurityConfig {
    config := DefaultSecurityConfig()
    
    // Relaxed rate limiting for testing
    config.RateLimit = 10000          // Higher limit for testing
    config.RateLimitWindow = time.Hour
    
    // Enhanced logging for debugging
    config.LogAllRequests = true      // Log all requests
    config.LogSecurityEvents = true
    
    // Shorter timeouts for faster feedback
    config.RequestTimeout = 10 * time.Second
    config.BlockDuration = 5 * time.Minute  // Shorter blocks
    
    // Relaxed CSP for development tools
    config.CSP = "default-src 'self' 'unsafe-eval' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' https:; connect-src 'self' ws: wss:; frame-ancestors 'self';"
    
    // Disable HSTS for HTTP development
    config.EnableHSTS = false
    
    return config
}
```

#### Staging Environment Configuration

Production-like security with enhanced monitoring:

```go
func StagingSecurityConfig() *SecurityConfig {
    config := DefaultSecurityConfig()
    
    // Production-like rate limiting
    config.RateLimit = 2000           // Slightly higher than production
    config.RateLimitWindow = time.Hour
    
    // Enhanced monitoring for testing
    config.LogAllRequests = true      // Log all requests for analysis
    config.LogSecurityEvents = true
    config.EnableMonitoring = true
    
    // Production-like security headers
    config.EnableHSTS = true          // Enable for HTTPS staging
    config.HSTSMaxAge = 86400        // 1 day for testing
    
    // Moderate blocking duration
    config.BlockDuration = 30 * time.Minute
    
    return config
}
```

#### Production Environment Configuration

Maximum security with optimized performance:

```go
func ProductionSecurityConfig() *SecurityConfig {
    config := DefaultSecurityConfig()
    
    // Strict rate limiting
    config.RateLimit = 1000
    config.RateLimitWindow = time.Hour
    
    // Path-specific limits for critical operations
    config.RateLimitByPath = map[string]int{
        "/api/v1/devices/{id}/control": 60,   // Reduced for production
        "/api/v1/provisioning":         30,   // Tighter control
        "/api/v1/config/bulk":          10,   // Very restrictive
        "/api/v1/auth/login":           10,   // Brute force protection
        "/api/v1/auth/forgot-password": 5,    // Account enumeration protection
    }
    
    // Production logging (selective)
    config.LogAllRequests = false     // Only log important requests
    config.LogSecurityEvents = true   // Always log security events
    config.EnableMonitoring = true
    
    // Strict security headers
    config.EnableHSTS = true
    config.HSTSMaxAge = 31536000      // 1 year
    config.CSP = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self';"
    
    // Strict request limits
    config.MaxRequestSize = 5 * 1024 * 1024  // 5MB for production
    config.RequestTimeout = 15 * time.Second  // Faster timeout
    
    // Longer blocking for persistent threats
    config.BlockDuration = 24 * time.Hour
    
    return config
}
```

### Validation Configuration

#### Default Validation Configuration

```go
func DefaultValidationConfig() *ValidationConfig {
    return &ValidationConfig{
        // Content type validation
        AllowedContentTypes: map[string]bool{
            "application/json":                  true,
            "application/x-www-form-urlencoded": true,
            "multipart/form-data":              true,
            "text/plain":                       false, // Disabled by default
        },
        StrictContentType: true,
        
        // Header validation
        RequiredHeaders:  []string{},                    // No required headers by default
        ForbiddenHeaders: []string{
            "x-forwarded-proto", "x-forwarded-host", 
            "x-original-url", "x-rewrite-url"
        },
        MaxHeaderSize:  8192,    // 8KB per header
        MaxHeaderCount: 50,      // Maximum 50 headers
        
        // JSON validation
        ValidateJSON:     true,
        MaxJSONDepth:     10,    // Maximum nesting depth
        MaxJSONArraySize: 1000,  // Maximum array size
        
        // Query parameter validation
        MaxQueryParamSize:  2048,   // 2KB per parameter
        MaxQueryParamCount: 50,     // Maximum 50 parameters
        ForbiddenParams: []string{
            "__proto__", "constructor", "prototype"
        },
        
        // Security validation
        BlockSuspiciousUserAgents: true,
        BlockSuspiciousHeaders:    true,
        LogValidationErrors:       true,
    }
}
```

#### Environment-Specific Validation

##### Development Validation
```go
func DevelopmentValidationConfig() *ValidationConfig {
    config := DefaultValidationConfig()
    
    // Allow text/plain for debugging
    config.AllowedContentTypes["text/plain"] = true
    
    // Relaxed limits for testing
    config.MaxJSONDepth = 20
    config.MaxJSONArraySize = 5000
    config.MaxQueryParamCount = 100
    
    // Less strict user agent checking
    config.BlockSuspiciousUserAgents = false
    
    return config
}
```

##### Production Validation
```go
func ProductionValidationConfig() *ValidationConfig {
    config := DefaultValidationConfig()
    
    // Stricter content types
    delete(config.AllowedContentTypes, "text/plain")
    
    // Tighter limits
    config.MaxJSONDepth = 8
    config.MaxJSONArraySize = 500
    config.MaxHeaderSize = 4096     // 4KB
    config.MaxHeaderCount = 30      // Fewer headers
    config.MaxQueryParamSize = 1024 // 1KB
    config.MaxQueryParamCount = 20  // Fewer parameters
    
    // Aggressive security validation
    config.BlockSuspiciousUserAgents = true
    config.BlockSuspiciousHeaders = true
    
    // Additional forbidden parameters
    config.ForbiddenParams = append(config.ForbiddenParams,
        "eval", "function", "setTimeout", "setInterval"
    )
    
    return config
}
```

## Rate Limiting Configuration

### Multi-Tier Rate Limiting Strategy

#### Global Rate Limits by Environment

```go
// Development: Permissive limits for testing
var DevelopmentRateLimits = map[string]int{
    "global":     10000,  // 10K requests/hour
    "auth":       1000,   // 1K auth attempts/hour  
    "api":        8000,   // 8K API calls/hour
}

// Staging: Production-like with buffer
var StagingRateLimits = map[string]int{
    "global":     2000,   // 2K requests/hour
    "auth":       100,    // 100 auth attempts/hour
    "api":        1500,   // 1.5K API calls/hour
}

// Production: Strict limits
var ProductionRateLimits = map[string]int{
    "global":     1000,   // 1K requests/hour
    "auth":       50,     // 50 auth attempts/hour  
    "api":        800,    // 800 API calls/hour
}
```

#### Path-Specific Rate Limiting

```go
// Production path-specific limits
var ProductionPathLimits = map[string]int{
    // Authentication endpoints - strict limits
    "/api/v1/auth/login":           10,  // Login attempts
    "/api/v1/auth/logout":          20,  // Logout requests
    "/api/v1/auth/refresh":         30,  // Token refresh
    "/api/v1/auth/forgot-password": 5,   // Password reset
    
    // Device management - moderate limits  
    "/api/v1/devices":              200, // Device listing/creation
    "/api/v1/devices/{id}":         500, // Individual device operations
    
    // Device control - controlled limits
    "/api/v1/devices/{id}/control": 60,  // Device control actions
    "/api/v1/devices/{id}/status":  120, // Status checks
    "/api/v1/devices/{id}/energy":  100, // Energy monitoring
    
    // Configuration - restrictive limits
    "/api/v1/config":               30,  // Configuration access
    "/api/v1/config/bulk":          10,  // Bulk operations
    "/api/v1/provisioning":         30,  // Device provisioning
    
    // Administrative - very restrictive
    "/api/v1/admin":                20,  // Admin operations
    "/api/v1/metrics":              100, // Metrics access
    "/api/v1/security/metrics":     50,  // Security metrics
}
```

### Rate Limit Response Configuration

```go
// Rate limit response headers
func SetRateLimitHeaders(w http.ResponseWriter, limit, remaining int, resetTime time.Time) {
    w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
    w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
    w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))
    w.Header().Set("Retry-After", strconv.Itoa(int(time.Until(resetTime).Seconds())))
}

// Rate limit exceeded response
var RateLimitResponse = map[string]interface{}{
    "success": false,
    "error": map[string]interface{}{
        "code":    "RATE_LIMIT_EXCEEDED",
        "message": "Too many requests. Please try again later.",
    },
    "meta": map[string]interface{}{
        "retry_after": "3600",  // seconds
        "limit":       "1000",  // requests per hour
    },
}
```

## Security Header Configuration

### Content Security Policy (CSP) Templates

#### Development CSP
```http
Content-Security-Policy: 
    default-src 'self' 'unsafe-eval' 'unsafe-inline'; 
    script-src 'self' 'unsafe-eval' 'unsafe-inline' localhost:* 127.0.0.1:*; 
    style-src 'self' 'unsafe-inline' fonts.googleapis.com; 
    img-src 'self' data: https: blob:; 
    font-src 'self' fonts.gstatic.com; 
    connect-src 'self' ws: wss: localhost:* 127.0.0.1:*; 
    frame-ancestors 'self';
```

#### Staging CSP
```http
Content-Security-Policy: 
    default-src 'self'; 
    script-src 'self' 'unsafe-inline'; 
    style-src 'self' 'unsafe-inline' fonts.googleapis.com; 
    img-src 'self' data: https:; 
    font-src 'self' fonts.gstatic.com; 
    connect-src 'self' wss:; 
    frame-ancestors 'none'; 
    base-uri 'self'; 
    form-action 'self';
```

#### Production CSP
```http
Content-Security-Policy: 
    default-src 'self'; 
    script-src 'self'; 
    style-src 'self'; 
    img-src 'self' https:; 
    font-src 'self'; 
    connect-src 'self'; 
    frame-ancestors 'none'; 
    base-uri 'self'; 
    form-action 'self'; 
    upgrade-insecure-requests;
```

### Security Header Templates

#### Complete Security Headers for Production
```go
func SetProductionSecurityHeaders(w http.ResponseWriter) {
    // CSP
    w.Header().Set("Content-Security-Policy", ProductionCSP)
    
    // Frame protection
    w.Header().Set("X-Frame-Options", "DENY")
    
    // Content type protection
    w.Header().Set("X-Content-Type-Options", "nosniff")
    
    // XSS protection
    w.Header().Set("X-XSS-Protection", "1; mode=block")
    
    // Referrer policy
    w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
    
    // Permissions policy
    w.Header().Set("Permissions-Policy", 
        "geolocation=(), camera=(), microphone=(), payment=(), usb=()")
    
    // HSTS (HTTPS only)
    if r.TLS != nil {
        w.Header().Set("Strict-Transport-Security", 
            "max-age=31536000; includeSubDomains; preload")
    }
    
    // Cache control for sensitive endpoints
    if strings.Contains(r.URL.Path, "/api/v1/") {
        w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
        w.Header().Set("Pragma", "no-cache")
        w.Header().Set("Expires", "0")
    }
}
```

## IP Blocking and Monitoring Configuration

### Automatic IP Blocking Configuration

```go
type IPBlockingConfig struct {
    // Blocking criteria
    SuspiciousRequestThreshold int           // 10 suspicious requests
    RateLimitViolationThreshold int          // 5 rate limit violations  
    AttackTypeThreshold        int           // 3 different attack types
    AttackRateThreshold        float64       // 5 attacks per hour
    
    // Blocking duration by severity
    BlockDurations map[string]time.Duration
    
    // Whitelist for internal/admin IPs
    WhitelistedIPs []string
    
    // Geographic blocking
    BlockedCountries []string
}

var ProductionIPBlockingConfig = IPBlockingConfig{
    SuspiciousRequestThreshold: 5,           // Stricter in production
    RateLimitViolationThreshold: 3,          // Lower tolerance
    AttackTypeThreshold: 2,                  // Block faster
    AttackRateThreshold: 3.0,                // 3 attacks/hour
    
    BlockDurations: map[string]time.Duration{
        "sql_injection":     24 * time.Hour,    // 24 hours
        "xss_attempt":       12 * time.Hour,    // 12 hours  
        "automated_scanner": 48 * time.Hour,    // 48 hours
        "rate_limit":        1 * time.Hour,     // 1 hour
        "general_suspicious": 6 * time.Hour,    // 6 hours
    },
    
    WhitelistedIPs: []string{
        "10.0.0.0/8",     // Internal networks
        "172.16.0.0/12",  // Private networks
        "192.168.0.0/16", // Local networks
    },
    
    BlockedCountries: []string{}, // Configure as needed
}
```

### Security Monitoring Configuration

```go
type MonitoringConfig struct {
    // Alert thresholds
    AlertThresholds map[string]int
    
    // Metrics retention
    MetricsRetentionHours int
    
    // Alert destinations
    AlertWebhooks []string
    AlertEmails   []string
    
    // Reporting intervals
    ReportInterval time.Duration
}

var ProductionMonitoringConfig = MonitoringConfig{
    AlertThresholds: map[string]int{
        "attacks_per_minute":     10,  // 10 attacks/minute
        "blocked_ips_per_hour":   20,  // 20 IPs blocked/hour
        "rate_violations_per_minute": 50, // 50 violations/minute
        "validation_failures_per_minute": 100, // 100 failures/minute
    },
    
    MetricsRetentionHours: 168, // 7 days
    
    AlertWebhooks: []string{
        "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK",
        "https://api.pagerduty.com/integration/YOUR/PAGERDUTY/KEY",
    },
    
    AlertEmails: []string{
        "security@company.com",
        "devops@company.com",
    },
    
    ReportInterval: 1 * time.Hour,
}
```

## Deployment Configuration

### Docker Configuration

#### Dockerfile Security Configurations
```dockerfile
FROM golang:1.21-alpine AS builder

# Security: Run as non-root user
RUN adduser -D -s /bin/sh appuser

# Security: Update packages and install CA certificates
RUN apk update && apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy and build application
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o shelly-manager ./cmd/shelly-manager

# Final stage
FROM scratch

# Security: Copy CA certificates for HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/passwd /etc/passwd

# Copy binary
COPY --from=builder /build/shelly-manager /shelly-manager

# Security: Run as non-root user
USER appuser

# Expose port
EXPOSE 8080

ENTRYPOINT ["/shelly-manager"]
```

#### Docker Compose Security Configuration
```yaml
version: '3.8'

services:
  shelly-manager:
    image: shelly-manager:latest
    container_name: shelly-manager-prod
    
    # Security: Non-root user
    user: "1000:1000"
    
    # Security: Read-only root filesystem
    read_only: true
    
    # Security: No new privileges
    security_opt:
      - no-new-privileges:true
    
    # Security: Drop all capabilities
    cap_drop:
      - ALL
    
    # Resource limits
    deploy:
      resources:
        limits:
          cpus: '1.0'
          memory: 512M
        reservations:
          cpus: '0.5'
          memory: 256M
    
    # Environment variables
    environment:
      - ENVIRONMENT=production
      - LOG_LEVEL=info
      - SECURITY_LEVEL=maximum
      - RATE_LIMIT_ENABLED=true
      - IP_BLOCKING_ENABLED=true
    
    # Health check
    healthcheck:
      test: ["CMD", "/shelly-manager", "health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    
    # Restart policy
    restart: unless-stopped
    
    # Ports
    ports:
      - "8080:8080"
    
    # Volumes (temporary filesystems)
    tmpfs:
      - /tmp
      - /var/tmp
    
    # Networks
    networks:
      - shelly-network

networks:
  shelly-network:
    driver: bridge
    driver_opts:
      com.docker.network.bridge.enable_icc: "false"
      com.docker.network.bridge.enable_ip_masquerade: "true"
```

### Kubernetes Configuration

#### Security Context and Pod Security Standards
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: shelly-manager
  namespace: shelly-system
spec:
  replicas: 3
  selector:
    matchLabels:
      app: shelly-manager
  template:
    metadata:
      labels:
        app: shelly-manager
      annotations:
        # Security: Pod security standards
        pod-security.kubernetes.io/enforce: restricted
        pod-security.kubernetes.io/audit: restricted
        pod-security.kubernetes.io/warn: restricted
    spec:
      # Security: Service account
      serviceAccountName: shelly-manager
      
      # Security: Security context
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        runAsGroup: 1000
        fsGroup: 1000
        seccompProfile:
          type: RuntimeDefault
      
      containers:
      - name: shelly-manager
        image: shelly-manager:1.0.0
        imagePullPolicy: Always
        
        # Security: Container security context
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          runAsUser: 1000
          capabilities:
            drop:
              - ALL
        
        # Environment configuration
        env:
        - name: ENVIRONMENT
          value: "production"
        - name: LOG_LEVEL
          value: "info"
        - name: SECURITY_LEVEL
          value: "maximum"
        
        # Resource limits
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 256Mi
        
        # Health checks
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 3
        
        # Ports
        ports:
        - containerPort: 8080
          protocol: TCP
        
        # Volume mounts
        volumeMounts:
        - name: tmp
          mountPath: /tmp
        - name: var-tmp
          mountPath: /var/tmp
        - name: config
          mountPath: /etc/shelly-manager
          readOnly: true
      
      # Volumes
      volumes:
      - name: tmp
        emptyDir: {}
      - name: var-tmp
        emptyDir: {}
      - name: config
        configMap:
          name: shelly-manager-config
```

#### Network Policy for Security Isolation
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: shelly-manager-netpol
  namespace: shelly-system
spec:
  podSelector:
    matchLabels:
      app: shelly-manager
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-system
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: database-system
    ports:
    - protocol: TCP
      port: 5432
  - to: []
    ports:
    - protocol: UDP
      port: 53
```

## Configuration Validation and Testing

### Configuration Validation Scripts

#### Security Configuration Validator
```go
func ValidateSecurityConfig(config *SecurityConfig) []error {
    var errors []error
    
    // Rate limiting validation
    if config.RateLimit <= 0 {
        errors = append(errors, fmt.Errorf("rate limit must be positive"))
    }
    if config.RateLimitWindow <= 0 {
        errors = append(errors, fmt.Errorf("rate limit window must be positive"))
    }
    
    // Request size validation
    if config.MaxRequestSize <= 0 {
        errors = append(errors, fmt.Errorf("max request size must be positive"))
    }
    if config.MaxRequestSize > 100*1024*1024 { // 100MB
        errors = append(errors, fmt.Errorf("max request size too large (>100MB)"))
    }
    
    // Timeout validation
    if config.RequestTimeout <= 0 {
        errors = append(errors, fmt.Errorf("request timeout must be positive"))
    }
    if config.RequestTimeout > 5*time.Minute {
        errors = append(errors, fmt.Errorf("request timeout too long (>5min)"))
    }
    
    // CSP validation
    if config.CSP == "" {
        errors = append(errors, fmt.Errorf("CSP cannot be empty"))
    }
    
    // HSTS validation
    if config.EnableHSTS && config.HSTSMaxAge <= 0 {
        errors = append(errors, fmt.Errorf("HSTS max age must be positive when HSTS is enabled"))
    }
    
    return errors
}
```

### Configuration Testing Framework

#### Unit Tests for Security Configuration
```go
func TestSecurityConfigValidation(t *testing.T) {
    tests := []struct {
        name          string
        config        *SecurityConfig
        expectErrors  int
    }{
        {
            name:         "default config should be valid",
            config:       DefaultSecurityConfig(),
            expectErrors: 0,
        },
        {
            name: "invalid rate limit should fail",
            config: &SecurityConfig{
                RateLimit: -1,
            },
            expectErrors: 1,
        },
        // Add more test cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            errors := ValidateSecurityConfig(tt.config)
            if len(errors) != tt.expectErrors {
                t.Errorf("expected %d errors, got %d", tt.expectErrors, len(errors))
            }
        })
    }
}
```

## Monitoring and Alerting Configuration

### Metrics Collection Configuration

#### Prometheus Metrics Integration
```go
// Security metrics for Prometheus
var (
    securityRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "shelly_security_requests_total",
            Help: "Total number of requests processed by security middleware",
        },
        []string{"status", "attack_type"},
    )
    
    securityBlockedIPs = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "shelly_security_blocked_ips",
            Help: "Number of currently blocked IP addresses",
        },
    )
    
    securityRateLimitViolations = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "shelly_security_rate_limit_violations_total",
            Help: "Total number of rate limit violations",
        },
        []string{"path"},
    )
)
```

### Alert Configuration

#### Grafana Alert Rules
```yaml
groups:
  - name: shelly-security-alerts
    rules:
    - alert: HighAttackRate
      expr: rate(shelly_security_requests_total{status="blocked"}[5m]) > 10
      for: 2m
      labels:
        severity: critical
      annotations:
        summary: "High attack rate detected"
        description: "Attack rate is {{ $value }} requests/second over the last 5 minutes"
    
    - alert: SecurityServiceDown
      expr: up{job="shelly-manager"} == 0
      for: 1m
      labels:
        severity: critical
      annotations:
        summary: "Shelly Manager security service is down"
        description: "The Shelly Manager security service has been down for more than 1 minute"
    
    - alert: TooManyBlockedIPs
      expr: shelly_security_blocked_ips > 100
      for: 5m
      labels:
        severity: warning
      annotations:
        summary: "Too many blocked IP addresses"
        description: "{{ $value }} IP addresses are currently blocked"
```

This comprehensive configuration guide provides all necessary details for deploying the API security framework across different environments while maintaining optimal security postures and performance characteristics.