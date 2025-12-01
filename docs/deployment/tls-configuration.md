# TLS Configuration

## Overview

Shelly Manager should be deployed behind a TLS-terminating reverse proxy in production. This guide covers common configurations for Nginx and Traefik, including HSTS and WebSocket support.

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

    # HSTS (2 years)
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

Example header:

```
Strict-Transport-Security: max-age=63072000; includeSubDomains; preload
```

## Traefik (Kubernetes) Example

See `deploy/kubernetes/traefik-ingress.yaml` for a complete example using TLS and security headers middleware.

---

## Notes

- Terminate TLS at the edge and forward HTTP to the service in-cluster.
- Ensure reverse proxy timeouts support long-lived connections (WebSockets).
- Prefer Let’s Encrypt or a trusted CA for certificates in production.

