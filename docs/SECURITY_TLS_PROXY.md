# TLS and Proxy Hardening Guide

This guide shows how to deploy Shelly Manager behind a TLS‑terminating reverse proxy with strict security headers. It provides production‑ready examples for NGINX Ingress and Traefik.

Important: The application already sets security headers at the app layer. Use your proxy to enforce HTTPS (redirects, HSTS) and optionally add/override headers. Avoid duplicating CSP unless you need a stricter policy.

## Goals
- Enforce HTTPS with automatic HTTP→HTTPS redirects
- Enable HSTS with a safe max‑age (and optional preload)
- Apply common hardening headers (X‑Frame‑Options, X‑Content‑Type‑Options, Referrer‑Policy, Permissions‑Policy, CSP)
- Keep large upload limits and timeouts sane for API use

## NGINX Ingress (Kubernetes)

Minimal secure Ingress example:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: shelly-manager
  annotations:
    kubernetes.io/ingress.class: nginx
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
    nginx.ingress.kubernetes.io/proxy-body-size: "10m"
    # Security headers (avoid conflicting CSP if app sets it to stricter policy)
    nginx.ingress.kubernetes.io/server-snippet: |
      add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
      add_header X-Frame-Options "DENY" always;
      add_header X-Content-Type-Options "nosniff" always;
      add_header Referrer-Policy "strict-origin-when-cross-origin" always;
      add_header Permissions-Policy "geolocation=(), camera=(), microphone=(), payment=()" always;
      add_header Content-Security-Policy "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self';" always;
spec:
  tls:
  - hosts: ["manager.example.com"]
    secretName: shelly-production-tls
  rules:
  - host: manager.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: shelly-manager
            port:
              number: 8080
```

Notes:
- HSTS `preload` is optional and carries operational risk. Only enable once you’re confident in permanent HTTPS across all subdomains.
- Use cert-manager to manage `secretName` certificates automatically.
- The app’s WebSocket endpoint `/metrics/ws` works through NGINX; ensure `nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"` for long‑lived connections if needed.

## Traefik (Kubernetes)

Define a security headers middleware and attach it to an IngressRoute:

```yaml
apiVersion: traefik.containo.us/v1alpha1
kind: Middleware
metadata:
  name: shelly-security-headers
spec:
  headers:
    sslRedirect: true
    stsSeconds: 31536000
    stsIncludeSubdomains: true
    # stsPreload: true   # Optional; see HSTS preload cautions
    contentTypeNosniff: true
    frameDeny: true
    referrerPolicy: "strict-origin-when-cross-origin"
    customResponseHeaders:
      Permissions-Policy: "geolocation=(), camera=(), microphone=(), payment=()"
      Content-Security-Policy: "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self';"
---
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: shelly-manager
spec:
  entryPoints:
    - websecure
  routes:
    - match: Host(`manager.example.com`)
      kind: Rule
      services:
        - name: shelly-manager
          port: 8080
      middlewares:
        - name: shelly-security-headers
  tls:
    secretName: shelly-production-tls
```

To redirect HTTP→HTTPS add a `Middleware` with `redirectScheme: https` and attach it on the `web` entrypoint.

## Edge / Cloud TLS

If terminating TLS at a cloud/load‑balancer layer (Cloudflare, ALB, etc.):
- Enforce HTTPS redirection and HSTS there.
- Preserve client IP headers (`X-Forwarded-For`, `X-Real-IP`) and configure `security.use_proxy_headers` and `security.trusted_proxies` in the app.
- Keep TLS 1.2+ and modern ciphers; disable legacy protocols.

## Validation & Testing

Quick checks:
- `curl -I https://manager.example.com` to verify headers (HSTS, X‑Frame‑Options, etc.).
- Browser DevTools → Network → Response headers.
- External scanners: securityheaders.com, observatory.mozilla.org.

## Operational Guidance

- Start with HSTS `max-age=31536000` and no preload; add `preload` only after subdomains are ready.
- Keep CSP aligned with the app; if you customize CSP at the proxy, ensure it matches your frontend (Vue migration may tighten CSP).
- Ensure large enough `proxy-body-size` for export/import payloads when needed.

