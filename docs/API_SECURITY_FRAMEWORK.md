# API Security Framework Documentation

## Overview

The Shelly Manager API Security Framework provides comprehensive protection against common web application vulnerabilities and attacks. This document describes the implemented security measures, configuration options, and monitoring capabilities.

## Security Features Implemented

### 1. Comprehensive Security Headers

**Purpose**: Protect against common web vulnerabilities including XSS, clickjacking, and MIME-type sniffing.

**Headers Implemented**:
- **Content-Security-Policy (CSP)**: Prevents XSS and code injection attacks
- **X-Frame-Options**: Prevents clickjacking attacks (set to DENY)
- **X-Content-Type-Options**: Prevents MIME-type sniffing (set to nosniff)
- **X-XSS-Protection**: Enables browser XSS filtering (set to 1; mode=block)
- **Referrer-Policy**: Controls referrer information leakage (set to strict-origin-when-cross-origin)
- **Strict-Transport-Security (HSTS)**: Enforces HTTPS connections (configurable)
- **Permissions-Policy**: Controls browser feature access

**Default CSP**: `default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self';`

### 2. Advanced Rate Limiting

**Features**:
- IP-based rate limiting with configurable limits and time windows
- Path-specific rate limits for sensitive endpoints
- Automatic rate limit violation tracking
- Rate limit headers in responses (X-RateLimit-*)

**Default Limits**:
- General API: 1,000 requests per hour per IP
- Device control endpoints: 100 requests per hour per IP
- Provisioning endpoints: 50 requests per hour per IP
- Bulk operations: 20 requests per hour per IP

### 3. Request Validation Framework

**Content Type Validation**:
- Configurable allowed content types
- Strict content-type enforcement option
- Media type parsing with parameter support

**Header Validation**:
- Maximum header count and size limits
- Forbidden header detection
- Suspicious header content filtering
- User-agent validation

**JSON Validation**:
- JSON syntax validation
- Configurable nesting depth limits
- Array size limits
- Structure validation

**Query Parameter Validation**:
- Parameter count and size limits
- Forbidden parameter name detection
- Suspicious content filtering

### 4. Security Monitoring and Alerting

**Attack Detection**:
- SQL injection attempt detection
- XSS attempt detection
- Path traversal attempt detection
- Automated scanner detection
- Suspicious user agent detection

**IP Blocking**:
- Automatic IP blocking based on attack patterns
- Configurable blocking duration
- Temporary blocks with automatic expiration

**Security Metrics**:
- Real-time security statistics
- Attack type categorization
- Top attacker IP tracking
- Alert level distribution
- Active threat monitoring

### 5. Standardized API Responses

**Response Format**:
```json
{
  "success": true|false,
  "data": {...},
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable message",
    "details": {...}
  },
  "meta": {
    "count": 10,
    "pagination": {...},
    "version": "v1"
  },
  "timestamp": "2024-01-01T12:00:00Z",
  "request_id": "req-123"
}
```

**Error Codes**:
- Client errors (4xx): `BAD_REQUEST`, `UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`, etc.
- Server errors (5xx): `INTERNAL_SERVER_ERROR`, `SERVICE_UNAVAILABLE`, etc.
- Security errors: `RATE_LIMIT_EXCEEDED`, `IP_BLOCKED`, `VALIDATION_FAILED`
- Application errors: `DEVICE_NOT_FOUND`, `CONFIGURATION_ERROR`, etc.

## Configuration

### Security Configuration

```go
type SecurityConfig struct {
    // Content Security Policy
    CSP string
    
    // Rate limiting
    RateLimit         int           // requests per window
    RateLimitWindow   time.Duration // time window for rate limiting
    RateLimitByPath   map[string]int // path-specific rate limits
    
    // Request limits
    MaxRequestSize    int64         // maximum request body size in bytes
    RequestTimeout    time.Duration // maximum request processing time
    
    // Security headers
    EnableHSTS        bool          // enable Strict-Transport-Security
    HSTSMaxAge        int           // HSTS max-age in seconds
    PermissionsPolicy string        // permissions policy header
    
    // Logging and monitoring
    LogSecurityEvents bool          // enable security event logging
    LogAllRequests    bool          // enable request/response logging
    EnableMonitoring  bool          // enable security monitoring and alerting
    
    // Attack detection
    EnableIPBlocking  bool          // enable automatic IP blocking
    BlockDuration     time.Duration // how long to block suspicious IPs
}
```

### Validation Configuration

```go
type ValidationConfig struct {
    // Content type validation
    AllowedContentTypes map[string]bool
    StrictContentType   bool // enforce exact content-type matching
    
    // Header validation
    RequiredHeaders     []string
    ForbiddenHeaders    []string
    MaxHeaderSize       int // maximum size of individual headers
    MaxHeaderCount      int // maximum number of headers
    
    // JSON validation
    ValidateJSON        bool // validate JSON syntax for JSON requests
    MaxJSONDepth        int  // maximum nesting depth in JSON
    MaxJSONArraySize    int  // maximum array size in JSON
    
    // Query parameter validation
    MaxQueryParamSize   int      // maximum size of query parameters
    MaxQueryParamCount  int      // maximum number of query parameters
    ForbiddenParams     []string // forbidden parameter names
    
    // Security validation
    BlockSuspiciousUserAgents bool
    BlockSuspiciousHeaders    bool
    
    // Logging
    LogValidationErrors bool
}
```

## Middleware Stack Order

The security middleware is applied in a carefully designed order to ensure maximum protection:

1. **Recovery Middleware**: Catches panics and prevents information disclosure
2. **IP Blocking Middleware**: Blocks requests from known malicious IPs early
3. **Security Monitoring Middleware**: Tracks all requests for security analysis
4. **Security Logging Middleware**: Logs security events and suspicious activities
5. **Security Headers Middleware**: Sets comprehensive security headers
6. **Request Timeout Middleware**: Prevents resource exhaustion attacks
7. **Rate Limiting Middleware**: Prevents DoS and brute force attacks
8. **Request Size Middleware**: Limits payload size to prevent memory exhaustion
9. **Request Validation Middleware**: Validates headers, content types, and parameters
10. **Enhanced CORS Middleware**: Security-aware cross-origin request handling
11. **Standard Logging Middleware**: Existing HTTP request logging

## API Endpoints

### Security Metrics Endpoint

**GET `/metrics/security`**

Returns comprehensive security metrics and statistics.

**Response**:
```json
{
  "success": true,
  "data": {
    "total_requests": 12345,
    "blocked_requests": 45,
    "suspicious_requests": 123,
    "rate_limit_violations": 34,
    "validation_failures": 67,
    "attacks_by_type": {
      "sql_injection": 12,
      "xss_attempt": 8,
      "path_traversal": 5
    },
    "top_attacker_ips": [
      {
        "ip": "192.168.1.100",
        "total_requests": 500,
        "suspicious_requests": 45,
        "attack_types": {"sql_injection": 20, "xss_attempt": 25},
        "first_seen": "2024-01-01T10:00:00Z",
        "last_seen": "2024-01-01T11:30:00Z",
        "blocked": true
      }
    ],
    "alerts_by_level": {
      "CRITICAL": 5,
      "HIGH": 12,
      "MEDIUM": 23,
      "LOW": 45
    },
    "blocked_ips": 3,
    "active_threats": 2,
    "last_updated": "2024-01-01T12:00:00Z"
  }
}
```

## Usage Examples

### Migrating Existing Handlers

**Before (Old Pattern)**:
```go
func (h *Handler) GetDevices(w http.ResponseWriter, r *http.Request) {
    devices, err := h.DB.GetDevices()
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        w.Header().Set("Content-Type", "application/json")
        h.writeJSON(w, map[string]interface{}{
            "success": false,
            "error":   "Failed to get devices: " + err.Error(),
        })
        return
    }
    w.Header().Set("Content-Type", "application/json")
    h.writeJSON(w, map[string]interface{}{
        "success": true,
        "devices": devices,
    })
}
```

**After (New Pattern)**:
```go
func (h *Handler) GetDevices(w http.ResponseWriter, r *http.Request) {
    respWrapper := response.NewHandlerWrapper(h.logger)
    
    devices, err := h.DB.GetDevices()
    if err != nil {
        respWrapper.InternalError(w, r, err)
        return
    }
    
    meta := &response.Metadata{
        Count: &[]int{len(devices)}[0],
    }
    
    respWrapper.SuccessWithMeta(w, r, map[string]interface{}{
        "devices": devices,
    }, meta)
}
```

### Custom Security Configuration

```go
// Create custom security configuration
securityConfig := &middleware.SecurityConfig{
    RateLimit: 500,
    RateLimitWindow: 30 * time.Minute,
    EnableHSTS: true,
    HSTSMaxAge: 31536000,
    LogSecurityEvents: true,
    EnableMonitoring: true,
    EnableIPBlocking: true,
    BlockDuration: 2 * time.Hour,
}

validationConfig := &middleware.ValidationConfig{
    StrictContentType: true,
    MaxHeaderCount: 30,
    MaxJSONDepth: 5,
    LogValidationErrors: true,
}

// Setup router with custom configuration
router := api.SetupRoutesWithSecurity(handler, logger, securityConfig, validationConfig)
```

## Security Considerations

### Production Deployment

1. **Enable HSTS**: Set `EnableHSTS: true` for HTTPS deployments
2. **Configure CSP**: Customize Content Security Policy for your specific needs
3. **Rate Limiting**: Adjust rate limits based on expected traffic patterns
4. **Origin Validation**: Implement proper CORS origin validation
5. **Monitoring**: Enable comprehensive security logging and monitoring

### Monitoring and Alerting

1. **Security Metrics**: Regularly monitor `/metrics/security` endpoint
2. **Log Analysis**: Set up log aggregation for security events
3. **Alerting**: Configure alerts for security events and anomalies
4. **IP Blocking**: Monitor blocked IPs and adjust blocking policies

### Performance Impact

- **Overhead**: Security middleware adds <10ms overhead per request
- **Memory**: Rate limiter uses ~1KB per unique client IP
- **CPU**: Attack detection algorithms are optimized for performance
- **Storage**: Security logs and metrics require additional storage

## Troubleshooting

### Common Issues

1. **Rate Limit Exceeded**: Check rate limit configuration and client behavior
2. **Validation Failures**: Review request format and validation rules
3. **Blocked IPs**: Check security metrics and IP blocking policies
4. **High Memory Usage**: Monitor rate limiter and security monitor memory usage

### Debug Mode

Enable debug logging by setting `LogAllRequests: true` in security configuration.

### Health Checks

Monitor the security framework health through:
- Security metrics endpoint
- Application logs
- Rate limiter statistics
- Attack detection alerts

## Best Practices

1. **Regular Updates**: Keep security policies and configurations up to date
2. **Monitoring**: Implement comprehensive security monitoring
3. **Testing**: Regularly test security measures and incident response
4. **Documentation**: Maintain security documentation and procedures
5. **Training**: Ensure team understands security features and best practices

## API Security Checklist

- [x] Comprehensive security headers implemented
- [x] Rate limiting with path-specific limits
- [x] Request validation framework
- [x] Attack detection and IP blocking
- [x] Security monitoring and metrics
- [x] Standardized error responses
- [x] Security event logging
- [x] Input sanitization
- [x] Response size limiting
- [x] Request timeout protection
- [ ] Authentication and authorization (Task 2.1)
- [ ] Input validation enhancement (Task 4.1)
- [ ] API documentation with security notes

This security framework provides a robust foundation for protecting the Shelly Manager API against common web application vulnerabilities and attacks while maintaining high performance and usability.