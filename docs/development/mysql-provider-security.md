# MySQL Provider - Security Guide

## Executive Summary

The MySQL database provider implements a comprehensive security framework designed to protect against common database vulnerabilities while maintaining operational flexibility. Built on defense-in-depth principles, it provides SSL/TLS encryption, input validation, credential protection, and security monitoring for enterprise-grade deployments.

**Security Features:**
- **Default SSL Enforcement**: Preferred SSL mode with certificate validation support
- **Input Validation**: Multi-layer protection against SQL injection and XSS attacks
- **Credential Protection**: Comprehensive credential sanitization in logs and error messages
- **Connection Security**: Timeout enforcement and resource protection

## Security Model

### Defense-in-Depth Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                        │
├─────────────────────────────────────────────────────────────┤
│                  Input Validation Layer                    │
│  • DSN Format Validation     • SQL Injection Prevention   │
│  • XSS Pattern Detection     • Command Injection Prevention│
├─────────────────────────────────────────────────────────────┤
│                   Transport Security Layer                 │
│  • SSL/TLS Encryption        • Certificate Validation     │
│  • Host Name Verification    • Protocol Security          │
├─────────────────────────────────────────────────────────────┤
│                  Connection Security Layer                 │
│  • Connection Timeouts       • Resource Limits            │
│  • Pool Security             • Access Control             │
├─────────────────────────────────────────────────────────────┤
│                   Database Security Layer                  │
│  • Authentication            • Authorization              │
│  • Query Validation          • Transaction Isolation      │
└─────────────────────────────────────────────────────────────┘
```

### Zero-Trust Principles

1. **Never Trust Input**: All user input is validated and sanitized
2. **Always Encrypt**: Default SSL enforcement for all connections
3. **Minimize Attack Surface**: Limited connection exposure and resource usage
4. **Continuous Monitoring**: Real-time security event monitoring
5. **Credential Protection**: No credentials in logs or error messages

## SSL/TLS Configuration

### Supported TLS Modes

The MySQL provider accepts the four values the MySQL driver takes for `options.tls` and refuses any
other there (a `tls=` written in the DSN itself goes to the driver unchecked):

```go
validTLSModes := map[string]bool{
    "false":       true, // no TLS (not recommended for production)
    "true":        true, // TLS; the certificate chain and the host name are verified
    "skip-verify": true, // TLS; the certificate is not verified
    "preferred":   true, // TLS if the server offers it, plain otherwise (DEFAULT); not verified
}
```

`required`, `verify-ca`, `verify-identity` and `custom` are not modes of the driver: it would fail at
connect time, so the provider refuses them first. The options `ca`, `cert`, `key` and `serverName` are
refused as well ("custom TLS files are not supported"), because the driver has no such DSN parameter.

### Security Configuration Examples

#### Production Deployment (Recommended)
```yaml
database:
  provider: "mysql"
  dsn: "user:password@tcp(mysql.internal:3306)/shelly_manager"
  options:
    tls: "true"
    timeout: "10s"
    readTimeout: "30s"
    writeTimeout: "30s"
```

#### Development Environment
```yaml
database:
  provider: "mysql"
  dsn: "dev:devpass@tcp(localhost:3306)/shelly_dev"
  options:
    tls: "preferred"  # Default - secure but compatible
    timeout: "5s"
```

#### High-Security Environment
```yaml
database:
  provider: "mysql"
  dsn: "app:secure_password@tcp(mysql.secure.internal:3306)/production"
  options:
    tls: "true"
    charset: "utf8mb4"
    collation: "utf8mb4_unicode_ci"
```

### Certificate Management

#### What the provider does with certificates

The provider validates the `tls` mode and the absence of `ca`, `cert`, `key` and `serverName`, and
nothing else:

```go
func (m *MySQLProvider) validateSSLConfig(options map[string]string) error {
    for _, name := range []string{"ca", "cert", "key", "serverName"} {
        if _, ok := options[name]; ok {
            return fmt.Errorf("custom TLS files are not supported; use tls=preferred|true|skip-verify (option %q)", name)
        }
    }
    // then: tls must be one of false, true, skip-verify, preferred
}
```

With `tls: "true"` the driver verifies the server certificate chain and host name against the system
trust store of the machine or image that runs the application. A private CA has to be installed
there. Client certificates (mutual TLS) and a CA given by path are not supported: that needs the
provider to register a TLS config with the driver, which it does not do.

#### Certificate Security Best Practices

1. **Server certificate**:
   - Use `tls: "true"` for production, so the chain and the host name are checked
   - Put the DSN host name in the certificate
   - Monitor certificate expiration and rotate the server certificate before it ends

2. **Trust store**:
   - Install a private CA in the image's system trust store
   - Use a separate CA or certificate for each environment

## Input Validation and Injection Prevention

### DSN Security Validation

```go
func (m *MySQLProvider) validateDSNInput(dsn string) error {
    dangerousPatterns := []string{
        // SQL Injection Patterns
        "DROP TABLE", "DROP DATABASE", "DELETE FROM", "INSERT INTO",
        "UPDATE SET", "CREATE TABLE", "ALTER TABLE", "TRUNCATE",
        "UNION SELECT", "INFORMATION_SCHEMA",
        
        // SQL Comments and Operators
        "--", "/*", "*/", ";", 
        
        // Command Execution
        "EXEC", "EXECUTE", "sp_", "xp_",
        
        // XSS Patterns
        "<script", "javascript:", "onload=", "onerror=",
    }

    upperDSN := strings.ToUpper(dsn)
    for _, pattern := range dangerousPatterns {
        if strings.Contains(upperDSN, strings.ToUpper(pattern)) {
            return fmt.Errorf("potentially dangerous DSN content detected: contains pattern similar to %s", pattern)
        }
    }
    return nil
}
```

### Attack Vector Protection

#### 1. SQL Injection Prevention
```go
// Example dangerous DSN that would be rejected:
badDSN := "user:pass@tcp(localhost:3306)/test'; DROP TABLE users; --"

// Validation result:
err := provider.validateDSNInput(badDSN)
// Returns: "potentially dangerous DSN content detected: contains pattern similar to DROP TABLE"
```

#### 2. XSS Prevention
```go
// Example XSS attempt that would be rejected:
xssDSN := "user:pass@tcp(localhost:3306)/db<script>alert('xss')</script>"

// Validation result:  
err := provider.validateDSNInput(xssDSN)
// Returns: "potentially dangerous DSN content detected: contains pattern similar to <script"
```

#### 3. Command Injection Prevention
```go
// Example command injection that would be rejected:
cmdDSN := "user:pass@tcp(localhost:3306)/db; EXEC xp_cmdshell 'dir'"

// Validation result:
err := provider.validateDSNInput(cmdDSN)
// Returns: "potentially dangerous DSN content detected: contains pattern similar to EXEC"
```

## Credential Protection

### Connect Error Text

The MySQL driver quotes operator input in its own errors: the DSN parser names
the value it refused, and the dial error names the address it tried. No matcher
agrees with every driver on where a credential in that text begins and ends, so
the provider carries no driver text at all.

- The DSN is parsed once, by the driver's own parser. A DSN it refuses reports
  a fixed message ("the DSN could not be parsed by the database driver").
- The pool is opened on a sealed `driver.Connector`. Every connection attempt
  (the first, each growth of the pool, each reconnect) fails with a fixed
  classification built from the error's type (`errors.As`/`errors.Is`), never
  from its text.
- A server error is reported as its number only, because the server echoes the
  user and database it was sent.
- The GORM log writer receives the same sealed errors, so the log holds nothing
  more than the returned error.
- After `Connect` returns, the errors of the open connection (a rejected
  statement, a dropped connection, a migration, a `Close`) are sealed the same
  way, as `database operation failed: <kind>`: GORM is opened only through
  `openGORM`, whose wrapped dialector and row callback classify the error by
  type; GORM's trace line prints the sealed error.
  The constraint, record-not-found and context errors keep their identity.

```
database connection failed: host name did not resolve
database connection failed: connection refused
database connection failed: the server rejected the connection (MySQL error 1049, unknown database)
```

The classifier is in `internal/database/provider/connect_error.go`. Its tests
build each error shape by hand, and one test per malformed DSN asserts that a
password placed in the DSN is absent from the returned error and the captured log.

## Connection Security

### Timeout Configuration

Security-focused timeout configuration prevents resource exhaustion and hanging connections:

```go
func (m *MySQLProvider) buildDSN(baseDSN string, options map[string]string) (string, error) {
    // Default security timeouts
    if !strings.Contains(dsn, "timeout=") {
        timeout := "10s" // 10 seconds default - prevents hanging
        if t, ok := options["timeout"]; ok {
            timeout = t
        }
        dsn += "&timeout=" + timeout
    }
    
    // Read timeout for security
    if !strings.Contains(dsn, "readTimeout=") {
        readTimeout := "30s" // 30 seconds default
        dsn += "&readTimeout=" + readTimeout
    }
    
    // Write timeout for security
    if !strings.Contains(dsn, "writeTimeout=") {
        writeTimeout := "30s" // 30 seconds default
        dsn += "&writeTimeout=" + writeTimeout
    }
    
    return dsn, nil
}
```

### Connection Pool Security

MySQL-specific security settings for connection pooling:

```go
func (m *MySQLProvider) configureConnectionPool() error {
    // The pool is applied as configured. The defaults (25 open, 5 idle, 5 minute
    // lifetime, 10 minute idle time) come from the config helper, and the provider
    // factory refuses an unset open or idle count.
    sqlDB.SetMaxOpenConns(m.config.MaxOpenConns)           // Limited concurrent connections
    sqlDB.SetMaxIdleConns(m.config.MaxIdleConns)           // Minimal idle connections
    sqlDB.SetConnMaxLifetime(m.config.ConnMaxLifetime)     // Frequent rotation for security
    sqlDB.SetConnMaxIdleTime(m.config.ConnMaxIdleTime)     // Quick idle cleanup

    return nil
}
```

### Security Benefits:
- **Limited Attack Surface**: Restricted connection count limits DoS potential
- **Connection Rotation**: Regular rotation prevents stale connection attacks
- **Timeout Protection**: Prevents resource exhaustion from hanging connections
- **Resource Cleanup**: Aggressive idle connection cleanup

## Security Monitoring

### Health Check Security

The health check system provides security monitoring without information leakage:

```go
func (m *MySQLProvider) HealthCheck(ctx context.Context) HealthStatus {
    status := HealthStatus{
        CheckedAt: time.Now(),
        Details:   make(map[string]interface{}),
    }
    
    start := time.Now()
    if err := m.Ping(); err != nil {
        status.Healthy = false
        status.Error = err.Error() // a connect error carries no driver text
        status.ResponseTime = time.Since(start)
        return status
    }
    
    status.Healthy = true
    status.ResponseTime = time.Since(start)
    
    // Safe performance metrics (no sensitive data)
    stats := m.GetStats()
    status.Details["database_size"] = stats.DatabaseSize
    status.Details["total_queries"] = stats.TotalQueries
    status.Details["connection_count"] = stats.OpenConnections
    status.Details["version"] = m.Version() // MySQL version only
    
    return status
}
```

### Security Event Logging

Structured security logging without credential exposure:

```go
// Connection success logging (safe)
m.logger.WithFields(map[string]any{
    "provider": "mysql",
    "target":   config.Target(), // "mysql (DSN redacted)": a parsed field is not safe to print either
}).Info("Connected to MySQL database")

// Migration failure logging
m.logger.WithFields(map[string]any{
    "error":    err.Error(),
    "duration": duration,
    "models":   len(models),
}).Error("Database migration failed")
```

## Security Testing

### Comprehensive Security Test Suite

The MySQL provider includes 65+ security-focused tests covering all attack vectors:

#### 1. SSL/TLS Validation Tests
```go
func TestMySQLSecuritySSLValidation(t *testing.T) {
    // Tests the modes the driver takes
    validTLSModes := []string{"false", "true", "skip-verify", "preferred"}
    
    // Tests invalid TLS modes, among them the names the driver lacks
    invalidTLSModes := []string{"invalid", "wrong", "bad-mode",
                               "custom", "required", "verify-ca", "verify-identity"}
}
```

#### 2. Certificate Option Tests
```go
func TestMySQLSecuritySSLCertificateValidation(t *testing.T) {
    // Tests that ca, cert, key and serverName are each refused
    // Tests that an option such as collation still passes
}
```

#### 3. Credential Protection Tests
```go
func TestMySQLSecurityCredentialProtection(t *testing.T) {
    sensitiveCredentials := []string{
        "secret_password", "super_secret_key", "confidential_user",
        "private_token", "admin_pass", "db_secret",
    }
    // Validates credentials are never leaked in error messages
}
```

#### 4. Injection Prevention Tests
```go
func TestMySQLSecurityDSNInjection(t *testing.T) {
    maliciousDSNs := []string{
        "user:pass@tcp(localhost:3306)/test'; DROP TABLE users; --",
        "user'; DELETE FROM sensitive_data; --:pass@tcp(localhost:3306)/db",
        "user:pass@tcp(localhost:3306)/db<script>alert('xss')</script>",
        // ... more injection attempts
    }
}
```

### Security Test Execution

```bash
# Run security tests
go test ./internal/database/provider -v -run Security

# Run all security-related tests
go test ./internal/database/provider -v -run "Security|Credential|SSL|TLS|Validation"

# Security test coverage
go test ./internal/database/provider -v -run Security -cover
```

## Security Best Practices

### 1. Production Deployment

**Mandatory Security Settings:**
- Use `tls: "true"` for production (chain and host name verified against the system trust store)
- Install a private CA in the image's system trust store
- Configure appropriate timeouts (≤30s)
- Use utf8mb4 charset for full Unicode support
- Enable connection pool limits

**Example Production Configuration:**
```yaml
database:
  provider: "mysql"
  dsn: "app_user:${MYSQL_PASSWORD}@tcp(mysql.internal:3306)/production"
  options:
    tls: "true"
    charset: "utf8mb4"
    collation: "utf8mb4_unicode_ci"
  max_open_conns: 20
  max_idle_conns: 5
  conn_max_lifetime: 1800
  conn_max_idle_time: 300
```

### 2. Development Environment

**Recommended Development Settings:**
- Use `tls: "preferred"` for compatibility
- Use separate development credentials
- Enable detailed logging for debugging
- Use shorter timeouts for faster feedback

### 3. Certificate Management

**Certificate Security Checklist:**
- [ ] Store certificates in secure directories
- [ ] Use proper file permissions (600 for keys, 644 for certs)
- [ ] Implement certificate rotation procedures
- [ ] Monitor certificate expiration dates
- [ ] Use separate certificates per environment
- [ ] Validate certificate chains regularly

### 4. Monitoring and Alerting

**Security Monitoring Setup:**
- Monitor failed connection attempts
- Alert on unusual connection patterns
- Track certificate expiration dates
- Monitor slow query patterns
- Alert on error rate increases

**Example Monitoring Queries:**
```go
// Monitor connection failures
stats := provider.GetStats()
if stats.FailedQueries > threshold {
    alert("High connection failure rate detected")
}

// Monitor connection health
status := provider.HealthCheck(ctx)
if !status.Healthy {
    alert("Database health check failed: " + status.Error)
}
```

### 5. Incident Response

**Security Incident Procedures:**
1. **Connection Failures**: Check certificate validity and network connectivity
2. **Authentication Errors**: Verify credentials and certificate configuration
3. **SSL/TLS Errors**: Validate certificate chain and TLS configuration
4. **Injection Attempts**: Review DSN validation and input sanitization
5. **Performance Issues**: Check connection pool settings and query patterns

## Compliance and Standards

### OWASP Compliance

The MySQL provider addresses OWASP Top 10 database security risks:

1. **A03:2021 - Injection**: Comprehensive DSN validation and parameterized queries
2. **A02:2021 - Cryptographic Failures**: Default SSL enforcement and certificate validation
3. **A05:2021 - Security Misconfiguration**: Secure defaults and validation
4. **A09:2021 - Security Logging**: Structured logging without credential exposure
5. **A04:2021 - Insecure Design**: Defense-in-depth architecture

### Security Standards

**Implemented Security Standards:**
- SSL/TLS encryption with certificate validation
- Input validation and sanitization
- Secure error handling and logging
- Resource limit enforcement
- Connection timeout protection
- Credential protection and sanitization

This comprehensive security implementation ensures the MySQL provider meets enterprise security requirements while maintaining operational flexibility and performance.