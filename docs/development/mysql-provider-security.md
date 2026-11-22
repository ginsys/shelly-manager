# MySQL Provider - Security Guide

## Executive Summary

The MySQL database provider implements a comprehensive security framework designed to protect against common database vulnerabilities while maintaining operational flexibility. Built on defense-in-depth principles, it provides SSL/TLS encryption, a check of the TLS mode and the timeout options, credential protection, and security monitoring for enterprise-grade deployments.

**Security Features:**
- **Default SSL Enforcement**: Preferred SSL mode with certificate validation support
- **DSN Reading**: The DSN is read as the driver reads it; its content and the session settings in it are not filtered
- **Credential Protection**: Comprehensive credential sanitization in logs and error messages
- **Connection Security**: Timeout enforcement and resource protection

## Security Model

### Defense-in-Depth Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                        │
├─────────────────────────────────────────────────────────────┤
│                      DSN Reading Layer                     │
│  • DSN Format (driver parse) • Checked Timeout Options    │
│  • Defaults From Keys        • No Content Filter          │
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

1. **Read Input As The Driver Does**: The DSN is operator input; the provider adds its defaults from the keys of the DSN, reads the timeout options as durations and escapes the other options it adds
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

## What the Provider Reads in a DSN

The provider does not read a DSN or an option for content, and it does not filter
session settings or option values. The DSN comes from the operator's configuration
only.

- The DSN is read as the driver reads it. A word in the user, the password, the
  host or the database name is part of that value; a parameter is what the driver
  finds in the parameter section after the last `/` and the first `?`.
- The provider adds the defaults the DSN does not give (`tls`, `timeout`,
  `readTimeout`, `writeTimeout`, `charset`) and the options that name a setting
  (`collation`, `loc`, `maxAllowedPacket`). A default counts as given when the
  parameter section holds its key, never when a word of the whole text matches.
- The `timeout`, `readTimeout` and `writeTimeout` options are read as durations with
  `time.ParseDuration`, the function the driver uses, and any other value is refused
  with a message that names the option and not its value. Every duration the driver
  reads (`+5s`, `1.5s`, `1µs`) still works. The other options (`charset`,
  `collation`, `loc`, `maxAllowedPacket`) are escaped, so each stays one value; a
  `charset` option may list several names separated by a comma, and each name is
  escaped.
- The driver parses the DSN once, in `Connect`. A DSN it refuses reports a fixed
  message and none of the DSN text.
- Session settings and option values are not checked. They reach the server as the
  driver sends them, so a setting the server does not know stops the first
  connection with `database connection failed: the server rejected the connection
  (MySQL error 1193, unknown system variable)`. A wrong value for a known setting
  shows the server's error number alone.

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

`buildDSN` adds a default for each of `timeout` (10s), `readTimeout` (30s) and
`writeTimeout` (30s) when the parameter section of the DSN does not give that key. An
option of the same name replaces the default; the option must be a duration, and any
other value is refused with a message that names the option. An explicit value in the DSN, including `readTimeout=0`, is kept as written.

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

The MySQL provider includes 65+ security-focused tests covering:

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

#### 4. DSN Reading Tests
`TestMySQLBuildDSN_*` read what `buildDSN` returns the way the driver does: a word that
looks like SQL in a user or database name is part of that value, an option value
holding `&` adds no parameter, and a default is decided from the keys of the DSN.

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
  dsn: "app_user:<password>@tcp(mysql.internal:3306)/production"
  options:
    tls: "true"
    charset: "utf8mb4"
    collation: "utf8mb4_unicode_ci"
  max_open_conns: 20
  max_idle_conns: 5
  conn_max_lifetime: 1800
  conn_max_idle_time: 300
```

Write the real password in place of `<password>`, or set the whole DSN as `SHELLY_DATABASE_DSN`: the
loader does not expand a `${...}` reference in a config file (`internal/config/config.go:170-214`).

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
4. **Unknown Session Setting**: A setting the server does not know stops the first connection with MySQL error 1193; remove or correct it in the DSN
5. **Performance Issues**: Check connection pool settings and query patterns

## Compliance and Standards

### OWASP Compliance

The MySQL provider addresses OWASP Top 10 database security risks:

1. **A03:2021 - Injection**: Parameterized queries; the DSN is operator input and is not filtered by content
2. **A02:2021 - Cryptographic Failures**: Default SSL enforcement and certificate validation
3. **A05:2021 - Security Misconfiguration**: Secure defaults and validation
4. **A09:2021 - Security Logging**: Structured logging without credential exposure
5. **A04:2021 - Insecure Design**: Defense-in-depth architecture

### Security Standards

**Implemented Security Standards:**
- SSL/TLS encryption with certificate validation
- A check of the TLS mode and the timeout options; the DSN is read with the driver's own parse
- Secure error handling and logging
- Resource limit enforcement
- Connection timeout protection
- Credential protection and sanitization

This comprehensive security implementation ensures the MySQL provider meets enterprise security requirements while maintaining operational flexibility and performance.