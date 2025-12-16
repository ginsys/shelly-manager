# TLS/Proxy Hardening Guides

**Priority**: LOW - Enhancement
**Status**: not-started
**Effort**: 3-4 hours

## Context

Production deployments need TLS termination and HSTS configuration. Documentation and example manifests would help users secure their installations.

## Success Criteria

- [ ] TLS termination documentation added
- [ ] HSTS enablement guide created
- [ ] Nginx example manifest provided
- [ ] Traefik example manifest provided
- [ ] Security best practices documented

## Implementation

### Step 1: Create TLS Documentation

**File**: `docs/deployment/tls-configuration.md`

```markdown
# TLS Configuration

## Overview

Shelly Manager should be deployed behind a TLS-terminating reverse proxy
in production. This guide covers common configurations.

## Nginx Configuration

### Basic TLS Termination

```nginx
server {
    listen 443 ssl http2;
    server_name shelly.example.com;

    ssl_certificate /etc/ssl/certs/shelly.crt;
    ssl_certificate_key /etc/ssl/private/shelly.key;

    # Modern TLS configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256;
    ssl_prefer_server_ciphers off;

    # HSTS
    add_header Strict-Transport-Security "max-age=63072000" always;

    location / {
        proxy_pass http://shelly-manager:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

### HSTS Preload

For maximum security, submit to HSTS preload list:
1. Add `includeSubDomains` and `preload` directives
2. Submit at https://hstspreload.org/
```

### Step 2: Create Traefik Example

**File**: `deployments/k8s/traefik-ingress.yaml`

```yaml
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: shelly-manager
spec:
  entryPoints:
    - websecure
  routes:
    - match: Host(`shelly.example.com`)
      kind: Rule
      services:
        - name: shelly-manager
          port: 8080
      middlewares:
        - name: security-headers
  tls:
    certResolver: letsencrypt
---
apiVersion: traefik.containo.us/v1alpha1
kind: Middleware
metadata:
  name: security-headers
spec:
  headers:
    stsSeconds: 63072000
    stsIncludeSubdomains: true
    stsPreload: true
```

### Step 3: Add Security Best Practices

**File**: `docs/deployment/security-best-practices.md`

Document:
- TLS version requirements
- Cipher suite selection
- HSTS configuration
- CSP headers
- Rate limiting

## Validation

- Documentation renders correctly
- Example configs are syntactically valid
- Tested with common proxy configurations

## Dependencies

None

## Risk

None - Documentation only
