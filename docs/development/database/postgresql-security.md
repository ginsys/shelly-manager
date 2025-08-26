# PostgreSQL Security Guide

## Overview

This guide covers comprehensive security measures for the PostgreSQL database provider in Shelly Manager, including SSL/TLS configuration, credential management, access control, and security best practices for production deployments.

## SSL/TLS Security

### SSL Mode Configuration

The PostgreSQL provider supports all PostgreSQL SSL modes with security-focused defaults:

| SSL Mode | Security Level | Description | Recommended Use |
|----------|----------------|-------------|-----------------|
| `disable` | **None** | No encryption | ⚠️ Development only, isolated networks |
| `allow` | **Low** | Encrypt if server supports | ⚠️ Legacy compatibility only |
| `prefer` | **Medium** | Prefer encryption, allow fallback | 🔶 Development environments |
| `require` | **High** | Require encryption (default) | ✅ Production standard |
| `verify-ca` | **Higher** | Encrypt + verify CA certificate | ✅ High-security environments |
| `verify-full` | **Highest** | Full certificate validation | ✅ Maximum security |

### Default Security Configuration

The provider enforces secure defaults:

```go
// Default SSL enforcement in buildDSN()
if !strings.Contains(dsn, "sslmode=") {
    sslMode := "require" // Default to requiring SSL
    if mode, ok := options["sslmode"]; ok {
        sslMode = mode
    }
    dsn += "?sslmode=" + sslMode
}
```

**Configuration Example:**
```yaml
database:
  provider: "postgresql"
  dsn: "postgres://user:pass@host:5432/db"  # SSL automatically added
  options:
    sslmode: "require"  # Explicit configuration (recommended)
```

### SSL Certificate Management

#### Certificate-Based Authentication

For maximum security environments, use client certificate authentication:

```yaml
database:
  options:
    sslmode: "verify-full"
    sslcert: "/secure/certs/client.crt"
    sslkey: "/secure/certs/client.key"
    sslrootcert: "/secure/certs/ca.crt"
```

#### Certificate File Security

**File Permissions:**
```bash
# Root CA certificate (readable by application)
chmod 640 /secure/certs/ca.crt
chown postgres:shelly-manager /secure/certs/ca.crt

# Client certificate (readable by application)
chmod 640 /secure/certs/client.crt
chown postgres:shelly-manager /secure/certs/client.crt

# Private key (secured, application-only access)
chmod 400 /secure/certs/client.key
chown shelly-manager:shelly-manager /secure/certs/client.key
```

**Certificate Validation:**
```go
func (p *PostgreSQLProvider) validateSSLConfig(options map[string]string) error {
    // Validate certificate files exist and are accessible
    if sslMode == "verify-ca" || sslMode == "verify-full" {
        if sslRootCert, ok := options["sslrootcert"]; ok {
            if _, err := os.Stat(sslRootCert); os.IsNotExist(err) {
                return fmt.Errorf("SSL root certificate not found: %s", sslRootCert)
            }
        }
    }
}
```

#### Certificate Rotation

**Automated Certificate Rotation:**
```bash
#!/bin/bash
# cert-rotation.sh - Automated certificate rotation

CERT_DIR="/secure/certs"
BACKUP_DIR="/secure/certs/backup"
SERVICE="shelly-manager"

echo "Starting certificate rotation..."

# Backup current certificates
mkdir -p "$BACKUP_DIR/$(date +%Y%m%d-%H%M%S)"
cp "$CERT_DIR"/*.{crt,key} "$BACKUP_DIR/$(date +%Y%m%d-%H%M%S)/"

# Deploy new certificates
cp /tmp/new-certs/* "$CERT_DIR/"

# Set proper permissions
chmod 640 "$CERT_DIR"/*.crt
chmod 400 "$CERT_DIR"/*.key
chown shelly-manager:shelly-manager "$CERT_DIR"/*

# Test new certificates
if timeout 10 openssl s_client -connect "$POSTGRES_HOST:5432" \
   -cert "$CERT_DIR/client.crt" -key "$CERT_DIR/client.key" \
   -CAfile "$CERT_DIR/ca.crt" -verify_return_error < /dev/null; then
    echo "Certificate validation successful"
    
    # Restart service to use new certificates
    systemctl restart "$SERVICE"
    
    # Verify service health
    sleep 5
    if systemctl is-active --quiet "$SERVICE"; then
        echo "Certificate rotation completed successfully"
    else
        echo "Service failed after certificate rotation, rolling back..."
        cp "$BACKUP_DIR/$(ls -1t $BACKUP_DIR | head -1)"/* "$CERT_DIR/"
        systemctl restart "$SERVICE"
    fi
else
    echo "Certificate validation failed, rolling back..."
    cp "$BACKUP_DIR/$(ls -1t $BACKUP_DIR | head -1)"/* "$CERT_DIR/"
fi
```

## Credential Management

### Secure Credential Storage

#### Environment Variable Pattern

**Recommended Approach:**
```bash
# Store in environment variables (never in configuration files)
export POSTGRES_PASSWORD="$(cat /secure/secrets/postgres-password)"
export POSTGRES_SSL_KEY_PASSPHRASE="$(cat /secure/secrets/ssl-key-passphrase)"
```

**Configuration Reference:**
```yaml
database:
  dsn: "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:5432/${POSTGRES_DB}?sslmode=require"
```

#### Kubernetes Secret Management

**Secret Definition:**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: shelly-postgres-credentials
type: Opaque
stringData:
  username: "shelly_user"
  password: "secure_random_password_here"
  ssl-key-passphrase: "ssl_key_encryption_passphrase"
```

**Pod Configuration:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: shelly-manager
spec:
  template:
    spec:
      containers:
      - name: shelly-manager
        env:
        - name: POSTGRES_USER
          valueFrom:
            secretKeyRef:
              name: shelly-postgres-credentials
              key: username
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: shelly-postgres-credentials
              key: password
```

#### HashiCorp Vault Integration

**Vault Configuration:**
```bash
# Store PostgreSQL credentials in Vault
vault kv put secret/shelly-manager/postgres \
    username="shelly_user" \
    password="secure_password" \
    host="postgres.internal" \
    database="shelly_manager"

# Grant access to Shelly Manager service account
vault policy write shelly-manager-policy - <<EOF
path "secret/data/shelly-manager/postgres" {
  capabilities = ["read"]
}
EOF
```

### Credential Protection in Code

#### Password Masking

The provider implements comprehensive credential sanitization:

```go
// maskPassword masks credentials in DSN for logging
func maskPassword(dsn string) string {
    if idx := strings.Index(dsn, "://"); idx != -1 {
        if idx2 := strings.Index(dsn[idx+3:], "@"); idx2 != -1 {
            return dsn[:idx+3] + "****:****" + dsn[idx+3+idx2:]
        }
    }
    return dsn
}
```

#### Safe Error Handling

Errors never expose credentials:

```go
func (p *PostgreSQLProvider) Connect(config DatabaseConfig) error {
    // Log connection attempt without credentials
    p.logger.WithFields(map[string]any{
        "provider": "postgresql",
        "host":     p.getHostFromDSN(dsn), // Only host, no credentials
        "database": p.getDatabaseFromDSN(dsn),
    }).Info("Connecting to PostgreSQL database")
    
    if err := p.db.Ping(); err != nil {
        // Generic error, no credential exposure
        return fmt.Errorf("failed to connect to database: connection refused")
    }
}
```

#### Logging Security

All logging is sanitized to prevent credential leakage:

```go
// Safe host extraction without credentials
func (p *PostgreSQLProvider) getHostFromDSN(dsn string) string {
    if parsedURL, err := url.Parse(dsn); err == nil && parsedURL.Host != "" {
        return parsedURL.Host  // Host only, excludes userinfo
    }
    return "unknown"
}
```

## Access Control

### Database User Management

#### Principle of Least Privilege

**Application User:**
```sql
-- Create application user with minimal privileges
CREATE USER shelly_app WITH PASSWORD 'secure_password';

-- Grant only necessary privileges
GRANT CONNECT ON DATABASE shelly_manager TO shelly_app;
GRANT USAGE ON SCHEMA public TO shelly_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO shelly_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO shelly_app;

-- Grant schema modification for migrations (if needed)
GRANT CREATE ON SCHEMA public TO shelly_app;

-- Revoke superuser and creation privileges
REVOKE CREATEDB, CREATEROLE, SUPERUSER FROM shelly_app;
```

**Migration User (separate from application):**
```sql
-- Create migration user with schema modification privileges
CREATE USER shelly_migrate WITH PASSWORD 'migration_password';

-- Grant schema modification privileges
GRANT CONNECT ON DATABASE shelly_manager TO shelly_migrate;
GRANT ALL PRIVILEGES ON SCHEMA public TO shelly_migrate;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO shelly_migrate;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO shelly_migrate;

-- Still no superuser privileges
REVOKE CREATEDB, CREATEROLE, SUPERUSER FROM shelly_migrate;
```

#### Role-Based Access Control

**Define Roles:**
```sql
-- Create roles for different access levels
CREATE ROLE shelly_readonly;
CREATE ROLE shelly_readwrite;
CREATE ROLE shelly_admin;

-- Grant appropriate privileges to roles
GRANT USAGE ON SCHEMA public TO shelly_readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO shelly_readonly;

GRANT shelly_readonly TO shelly_readwrite;
GRANT INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO shelly_readwrite;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO shelly_readwrite;

GRANT shelly_readwrite TO shelly_admin;
GRANT CREATE, DROP ON SCHEMA public TO shelly_admin;

-- Assign roles to users
GRANT shelly_readwrite TO shelly_app;
GRANT shelly_admin TO shelly_migrate;
```

### Connection Security

#### Connection Timeout Configuration

Prevent resource exhaustion attacks:

```yaml
database:
  options:
    connect_timeout: "30"                 # 30 second connection timeout
    statement_timeout: "300000"           # 5 minute statement timeout
    idle_in_transaction_session_timeout: "600000"  # 10 minute idle timeout
```

#### IP Address Restrictions

**PostgreSQL Configuration (`postgresql.conf`):**
```conf
# Restrict connections to specific networks
listen_addresses = '10.0.0.0/8,172.16.0.0/12,192.168.0.0/16'

# Enable SSL
ssl = on
ssl_cert_file = '/etc/postgresql/certs/server.crt'
ssl_key_file = '/etc/postgresql/certs/server.key'
ssl_ca_file = '/etc/postgresql/certs/ca.crt'

# Require SSL for all connections
ssl_min_protocol_version = 'TLSv1.2'
ssl_prefer_server_ciphers = on
```

**Host-Based Authentication (`pg_hba.conf`):**
```conf
# Require SSL for all TCP connections
hostssl all all 10.0.0.0/8 md5
hostssl all all 172.16.0.0/12 md5
hostssl all all 192.168.0.0/16 md5

# Require client certificates for high-security environments
hostssl all shelly_app 10.0.1.0/24 cert clientcert=1

# Deny all other connections
host all all all reject
```

## Security Monitoring

### Connection Monitoring

**PostgreSQL Log Configuration:**
```conf
# Enable comprehensive logging
logging_collector = on
log_destination = 'stderr,csvlog'
log_filename = 'postgresql-%Y-%m-%d_%H%M%S.log'
log_rotation_age = 1d
log_rotation_size = 100MB

# Log security-relevant events
log_connections = on
log_disconnections = on
log_checkpoints = on
log_lock_waits = on
log_statement = 'ddl'              # Log schema changes
log_min_duration_statement = 1000  # Log slow queries
log_line_prefix = '%t [%p]: user=%u,db=%d,app=%a,client=%h '
```

### Application-Level Security Monitoring

The provider includes built-in security monitoring:

```go
// Track connection failures for security monitoring
func (p *PostgreSQLProvider) Connect(config DatabaseConfig) error {
    start := time.Now()
    
    if err := p.db.Ping(); err != nil {
        // Log security event without exposing credentials
        p.logger.WithFields(map[string]any{
            "event": "connection_failure",
            "duration": time.Since(start),
            "host": p.getHostFromDSN(dsn),
            "error_type": "connection_refused",
        }).Warn("Database connection failed")
        
        atomic.AddInt64(&p.failedQueries, 1)
        return fmt.Errorf("failed to connect to database")
    }
    
    // Log successful connection
    p.logger.WithFields(map[string]any{
        "event": "connection_success",
        "duration": time.Since(start),
        "provider": "postgresql",
        "version": p.Version(),
    }).Info("Database connection established")
}
```

### Security Alerting

**Prometheus Metrics for Security Monitoring:**
```go
// Security-focused metrics
var (
    connectionFailures = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "postgres_connection_failures_total",
            Help: "Total number of failed PostgreSQL connections",
        },
        []string{"host", "database", "error_type"},
    )
    
    sslConnectionAttempts = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "postgres_ssl_connection_attempts_total",
            Help: "Total number of SSL connection attempts by mode",
        },
        []string{"ssl_mode", "status"},
    )
)
```

**Grafana Alert Rules:**
```yaml
# Alert on excessive connection failures
- alert: PostgreSQLConnectionFailures
  expr: increase(postgres_connection_failures_total[5m]) > 10
  for: 1m
  labels:
    severity: warning
  annotations:
    summary: "High PostgreSQL connection failure rate"
    description: "{{ $value }} connection failures in the last 5 minutes"

# Alert on non-SSL connections (if SSL required)
- alert: PostgreSQLNonSSLConnection
  expr: postgres_ssl_connection_attempts_total{ssl_mode="disable"} > 0
  for: 0m
  labels:
    severity: critical
  annotations:
    summary: "Non-SSL PostgreSQL connection attempted"
    description: "SSL is required but non-SSL connection was attempted"
```

## Network Security

### Firewall Configuration

**iptables Rules:**
```bash
#!/bin/bash
# postgresql-firewall.sh - PostgreSQL firewall rules

# Allow PostgreSQL connections only from application networks
iptables -A INPUT -p tcp --dport 5432 -s 10.0.1.0/24 -j ACCEPT   # App servers
iptables -A INPUT -p tcp --dport 5432 -s 10.0.2.0/24 -j ACCEPT   # Admin network

# Block all other PostgreSQL connections
iptables -A INPUT -p tcp --dport 5432 -j DROP

# Log blocked attempts for security monitoring
iptables -A INPUT -p tcp --dport 5432 -j LOG --log-prefix "PG_BLOCKED: "
```

### VPN/Private Network Access

**WireGuard VPN Configuration:**
```ini
# /etc/wireguard/postgres-access.conf
[Interface]
PrivateKey = <private-key>
Address = 10.200.200.1/24
ListenPort = 51820

# Application server peer
[Peer]
PublicKey = <app-server-public-key>
AllowedIPs = 10.200.200.2/32

# Admin access peer
[Peer]
PublicKey = <admin-public-key>
AllowedIPs = 10.200.200.10/32
```

## Backup Security

### Encrypted Backups

**pg_dump with Encryption:**
```bash
#!/bin/bash
# secure-backup.sh - Encrypted PostgreSQL backups

BACKUP_DIR="/secure/backups"
GPG_RECIPIENT="backup@shelly-manager.com"
DATE=$(date +%Y%m%d-%H%M%S)

# Create encrypted backup
pg_dump -h "$POSTGRES_HOST" -U "$POSTGRES_USER" "$POSTGRES_DB" | \
    gzip | \
    gpg --trust-model always --encrypt -r "$GPG_RECIPIENT" \
    > "$BACKUP_DIR/shelly_backup_$DATE.sql.gz.gpg"

# Verify backup integrity
if [ $? -eq 0 ]; then
    echo "Backup created successfully: shelly_backup_$DATE.sql.gz.gpg"
    
    # Test backup decryption (without restoring)
    gpg --decrypt "$BACKUP_DIR/shelly_backup_$DATE.sql.gz.gpg" | \
        gunzip | head -n 10 > /dev/null
    
    if [ $? -eq 0 ]; then
        echo "Backup integrity verified"
    else
        echo "Backup integrity check failed!"
        exit 1
    fi
else
    echo "Backup creation failed!"
    exit 1
fi
```

### Backup Access Control

**Backup Storage Security:**
```bash
# Secure backup directory permissions
chmod 700 /secure/backups
chown postgres:postgres /secure/backups

# Restrict backup file access
find /secure/backups -name "*.gpg" -exec chmod 600 {} \;
find /secure/backups -name "*.gpg" -exec chown postgres:postgres {} \;
```

## Security Best Practices

### Production Security Checklist

#### SSL/TLS Security
- [ ] SSL mode set to `require` or higher
- [ ] TLS version 1.2 or higher enforced
- [ ] Client certificates implemented for high-security environments
- [ ] Certificate rotation process automated
- [ ] SSL connection monitoring enabled

#### Access Control
- [ ] Application uses dedicated database user with minimal privileges
- [ ] Migration user separate from application user
- [ ] Role-based access control implemented
- [ ] Connection timeouts configured
- [ ] IP address restrictions in place

#### Credential Management
- [ ] Passwords stored securely (environment variables, Vault, K8s secrets)
- [ ] No credentials in configuration files or code
- [ ] Credential rotation process documented and tested
- [ ] Error messages sanitized to prevent credential exposure

#### Monitoring and Alerting
- [ ] Connection failure monitoring enabled
- [ ] SSL connection attempts tracked
- [ ] Security events logged appropriately
- [ ] Alerting configured for security incidents

#### Network Security
- [ ] Firewall rules restrict database access
- [ ] VPN or private network access enforced
- [ ] Database not exposed to public internet
- [ ] Network traffic encrypted in transit

#### Backup Security
- [ ] Backups encrypted at rest
- [ ] Backup integrity verification automated
- [ ] Backup access properly restricted
- [ ] Backup restoration process tested regularly

This comprehensive security guide ensures the PostgreSQL provider meets enterprise security requirements while maintaining usability and performance.