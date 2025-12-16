# TLS Configuration & Proxy Hardening

This guide covers securing Shelly Manager with TLS termination and reverse proxy configuration for production deployments.

## Overview

Shelly Manager should be deployed behind a TLS-terminating reverse proxy in production. This provides:
- Encrypted communication (HTTPS/WSS)
- HTTP Strict Transport Security (HSTS)
- Centralized certificate management
- WebSocket upgrade support
- Request logging and monitoring

## Nginx Configuration

### Basic TLS Termination

Complete nginx configuration for Shelly Manager with TLS:

```nginx
upstream shelly_backend {
    server shelly-manager:8080;
    keepalive 32;
}

server {
    listen 80;
    server_name shelly.example.com;

    # Redirect all HTTP to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name shelly.example.com;

    # TLS Certificates
    ssl_certificate /etc/ssl/certs/shelly.example.com.crt;
    ssl_certificate_key /etc/ssl/private/shelly.example.com.key;

    # Modern TLS Configuration (Mozilla Intermediate)
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers off;

    # TLS Session Configuration
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 1h;
    ssl_session_tickets off;

    # OCSP Stapling
    ssl_stapling on;
    ssl_stapling_verify on;
    ssl_trusted_certificate /etc/ssl/certs/ca-chain.crt;
    resolver 8.8.8.8 8.8.4.4 valid=300s;
    resolver_timeout 5s;

    # Security Headers
    add_header Strict-Transport-Security "max-age=63072000; includeSubDomains; preload" always;
    add_header X-Frame-Options "DENY" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    # API and Web Application
    location / {
        proxy_pass http://shelly_backend;
        proxy_http_version 1.1;

        # Preserve original request information
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Forwarded-Port $server_port;

        # WebSocket support (for metrics, real-time updates)
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Timeout configuration
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 300s;  # 5 minutes for long-running operations

        # Buffer configuration
        proxy_buffering off;
        proxy_request_buffering off;

        # Connection keep-alive
        proxy_set_header Connection "";
    }

    # WebSocket-specific endpoints
    location /api/v1/metrics/ws {
        proxy_pass http://shelly_backend;
        proxy_http_version 1.1;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket upgrade headers
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Long timeout for WebSocket connections
        proxy_read_timeout 86400s;  # 24 hours
        proxy_send_timeout 86400s;

        # Disable buffering for WebSocket
        proxy_buffering off;
    }

    # Health check endpoints (no auth required)
    location ~ ^/(healthz|readyz) {
        proxy_pass http://shelly_backend;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        access_log off;  # Don't log health checks
    }

    # Metrics endpoint (optional, for Prometheus scraping)
    location /metrics/prometheus {
        proxy_pass http://shelly_backend;
        proxy_http_version 1.1;
        proxy_set_header Host $host;

        # Optional: Restrict to monitoring IPs
        # allow 10.0.0.0/8;
        # deny all;
    }

    # Custom error pages
    error_page 502 503 504 /50x.html;
    location = /50x.html {
        root /usr/share/nginx/html;
    }

    # Access logging
    access_log /var/log/nginx/shelly-access.log combined;
    error_log /var/log/nginx/shelly-error.log warn;
}
```

### Rate Limiting (Optional)

Add rate limiting to protect against abuse:

```nginx
# Define rate limit zones (place in http context)
limit_req_zone $binary_remote_addr zone=api_limit:10m rate=100r/m;
limit_req_zone $binary_remote_addr zone=auth_limit:10m rate=10r/m;

server {
    # ... existing configuration ...

    # Apply rate limiting to API endpoints
    location /api/ {
        limit_req zone=api_limit burst=20 nodelay;
        limit_req_status 429;

        # ... proxy configuration ...
    }

    # Stricter limits for auth endpoints
    location /api/v1/admin/ {
        limit_req zone=auth_limit burst=5 nodelay;

        # ... proxy configuration ...
    }
}
```

## Traefik Configuration

### Kubernetes Ingress (Traefik v2)

```yaml
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: shelly-manager-https
  namespace: shelly
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
        - name: rate-limit
  tls:
    secretName: shelly-tls-cert
    options:
      name: tls-options
---
apiVersion: traefik.containo.us/v1alpha1
kind: Middleware
metadata:
  name: security-headers
  namespace: shelly
spec:
  headers:
    frameDeny: true
    contentTypeNosniff: true
    browserXssFilter: true
    stsSeconds: 63072000
    stsIncludeSubdomains: true
    stsPreload: true
    referrerPolicy: "strict-origin-when-cross-origin"
---
apiVersion: traefik.containo.us/v1alpha1
kind: Middleware
metadata:
  name: rate-limit
  namespace: shelly
spec:
  rateLimit:
    average: 100
    period: 1m
    burst: 50
---
apiVersion: traefik.containo.us/v1alpha1
kind: TLSOption
metadata:
  name: tls-options
  namespace: shelly
spec:
  minVersion: VersionTLS12
  cipherSuites:
    - TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
    - TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
    - TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
    - TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
    - TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305
    - TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305
  curvePreferences:
    - CurveP521
    - CurveP384
  sniStrict: true
---
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: shelly-manager-http
  namespace: shelly
spec:
  entryPoints:
    - web
  routes:
    - match: Host(`shelly.example.com`)
      kind: Rule
      services:
        - name: shelly-manager
          port: 8080
      middlewares:
        - name: redirect-https
---
apiVersion: traefik.containo.us/v1alpha1
kind: Middleware
metadata:
  name: redirect-https
  namespace: shelly
spec:
  redirectScheme:
    scheme: https
    permanent: true
```

### Docker Compose (Traefik v2)

```yaml
version: '3.8'

services:
  traefik:
    image: traefik:v2.10
    command:
      - --api.dashboard=true
      - --providers.docker=true
      - --providers.docker.exposedbydefault=false
      - --entrypoints.web.address=:80
      - --entrypoints.websecure.address=:443
      - --certificatesresolvers.letsencrypt.acme.httpchallenge=true
      - --certificatesresolvers.letsencrypt.acme.httpchallenge.entrypoint=web
      - --certificatesresolvers.letsencrypt.acme.email=admin@example.com
      - --certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./letsencrypt:/letsencrypt
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.api.rule=Host(`traefik.example.com`)"
      - "traefik.http.routers.api.entrypoints=websecure"
      - "traefik.http.routers.api.tls.certresolver=letsencrypt"
      - "traefik.http.routers.api.service=api@internal"

  shelly-manager:
    image: ghcr.io/ginsys/shelly-manager:latest
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.shelly.rule=Host(`shelly.example.com`)"
      - "traefik.http.routers.shelly.entrypoints=websecure"
      - "traefik.http.routers.shelly.tls.certresolver=letsencrypt"
      - "traefik.http.services.shelly.loadbalancer.server.port=8080"
      # Security headers
      - "traefik.http.middlewares.shelly-headers.headers.frameDeny=true"
      - "traefik.http.middlewares.shelly-headers.headers.contentTypeNosniff=true"
      - "traefik.http.middlewares.shelly-headers.headers.browserXssFilter=true"
      - "traefik.http.middlewares.shelly-headers.headers.stsSeconds=63072000"
      - "traefik.http.middlewares.shelly-headers.headers.stsIncludeSubdomains=true"
      - "traefik.http.middlewares.shelly-headers.headers.stsPreload=true"
      - "traefik.http.routers.shelly.middlewares=shelly-headers@docker"
      # HTTP to HTTPS redirect
      - "traefik.http.routers.shelly-http.rule=Host(`shelly.example.com`)"
      - "traefik.http.routers.shelly-http.entrypoints=web"
      - "traefik.http.routers.shelly-http.middlewares=redirect-https@docker"
      - "traefik.http.middlewares.redirect-https.redirectscheme.scheme=https"
      - "traefik.http.middlewares.redirect-https.redirectscheme.permanent=true"
    environment:
      - SHELLY_HTTP_PORT=8080
      - SHELLY_SECURITY_ADMIN_API_KEY=${ADMIN_API_KEY}
    volumes:
      - ./config.yaml:/etc/shelly-manager/config.yaml:ro
      - shelly-data:/var/lib/shelly-manager

volumes:
  shelly-data:
```

## Certificate Management

### Let's Encrypt with Certbot

For automated certificate management:

```bash
# Install certbot
sudo apt-get install certbot python3-certbot-nginx

# Obtain certificate (HTTP-01 challenge)
sudo certbot --nginx -d shelly.example.com

# Auto-renewal is configured via systemd timer
sudo systemctl status certbot.timer

# Test renewal process
sudo certbot renew --dry-run
```

### Manual Certificate Installation

For custom certificates:

```bash
# Create certificate directory
sudo mkdir -p /etc/ssl/certs /etc/ssl/private

# Copy certificates
sudo cp shelly.example.com.crt /etc/ssl/certs/
sudo cp shelly.example.com.key /etc/ssl/private/
sudo cp ca-chain.crt /etc/ssl/certs/

# Set permissions
sudo chmod 644 /etc/ssl/certs/shelly.example.com.crt
sudo chmod 600 /etc/ssl/private/shelly.example.com.key

# Test nginx configuration
sudo nginx -t

# Reload nginx
sudo systemctl reload nginx
```

## Security Best Practices

### HSTS Preload List

For maximum security, submit your domain to the HSTS preload list:

1. Ensure HSTS header includes `includeSubDomains` and `preload` directives
2. Test header: `curl -I https://shelly.example.com`
3. Submit at: https://hstspreload.org/
4. Note: This is a one-way process - removal can take months

### Certificate Monitoring

Monitor certificate expiration:

```bash
# Check certificate expiration
echo | openssl s_client -connect shelly.example.com:443 2>/dev/null | openssl x509 -noout -dates

# Expected output:
# notBefore=Jan 15 00:00:00 2025 GMT
# notAfter=Apr 15 23:59:59 2025 GMT
```

### Security Testing

Test your TLS configuration:

- **SSL Labs**: https://www.ssllabs.com/ssltest/
- **Mozilla Observatory**: https://observatory.mozilla.org/
- **Security Headers**: https://securityheaders.com/

Target ratings:
- SSL Labs: A+ rating
- Mozilla Observatory: A+ rating
- Security Headers: A rating

### Firewall Configuration

Restrict access to Shelly Manager backend:

```bash
# Allow only nginx to access backend (iptables)
sudo iptables -A INPUT -p tcp --dport 8080 -s 127.0.0.1 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 8080 -j DROP

# Or using ufw
sudo ufw allow from 127.0.0.1 to any port 8080
sudo ufw deny 8080
```

### IP Allowlisting (Optional)

Restrict admin access to specific IPs in nginx:

```nginx
location /api/v1/admin/ {
    # Allow specific IPs
    allow 203.0.113.0/24;
    allow 198.51.100.5;
    deny all;

    # ... proxy configuration ...
}
```

## Monitoring & Logging

### Access Log Analysis

Monitor for suspicious activity:

```bash
# Failed admin auth attempts
sudo grep "401.*\/api\/v1\/admin" /var/log/nginx/shelly-access.log

# Rate limit hits
sudo grep "429" /var/log/nginx/shelly-access.log

# Most active IPs
sudo awk '{print $1}' /var/log/nginx/shelly-access.log | sort | uniq -c | sort -rn | head -10
```

### Prometheus Metrics

Scrape nginx metrics using nginx-prometheus-exporter:

```yaml
# docker-compose.yml addition
nginx-exporter:
  image: nginx/nginx-prometheus-exporter:latest
  command:
    - -nginx.scrape-uri=http://nginx:8080/stub_status
  ports:
    - "9113:9113"
```

## Troubleshooting

### WebSocket Connection Issues

If WebSocket connections fail:

1. Check nginx error log: `sudo tail -f /var/log/nginx/shelly-error.log`
2. Verify Upgrade headers: `curl -i -N -H "Connection: Upgrade" -H "Upgrade: websocket" https://shelly.example.com/api/v1/metrics/ws`
3. Test without proxy: `wscat -c ws://shelly-manager:8080/api/v1/metrics/ws`

### Certificate Validation Errors

If browsers show certificate errors:

1. Verify certificate chain: `openssl s_client -connect shelly.example.com:443 -showcerts`
2. Check intermediate certificates are included
3. Verify DNS resolves correctly: `dig shelly.example.com`
4. Check certificate Common Name/SAN matches domain

### Performance Issues

If experiencing slow responses through proxy:

1. Check proxy buffer settings
2. Enable connection keep-alive
3. Review timeout values
4. Monitor backend response times
5. Consider enabling caching for static assets

## References

- [Mozilla SSL Configuration Generator](https://ssl-config.mozilla.org/)
- [Nginx WebSocket Proxying](https://nginx.org/en/docs/http/websocket.html)
- [Traefik TLS Documentation](https://doc.traefik.io/traefik/https/tls/)
- [OWASP TLS Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Transport_Layer_Protection_Cheat_Sheet.html)
