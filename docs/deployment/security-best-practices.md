# Security Best Practices

This guide summarizes recommended security controls when deploying Shelly Manager in production.

## TLS

- Enforce TLS 1.2 and 1.3 only.
- Prefer modern cipher suites and disable weak ciphers.
- Terminate TLS at a reverse proxy and forward HTTP to the service.

## HTTP Security Headers

- HSTS: `Strict-Transport-Security: max-age=63072000; includeSubDomains; preload`
- Content-Security-Policy: restrict sources as appropriate for your environment.
- X-Content-Type-Options: `nosniff`
- X-Frame-Options / Frame-ancestors: deny framing where not needed.

## Rate Limiting & DoS Protection

- Configure reverse proxy rate limits for login or expensive endpoints.
- Ensure reasonable request and header size limits.

## Operational Hardening

- Run containers as non-root with read-only root filesystem (already default in K8s manifests).
- Limit exposed ports and use network policies where applicable.
- Rotate admin API keys and secrets regularly via Kubernetes Secrets or external secret stores.

## References

- Mozilla TLS Configuration Generator
- OWASP Secure Headers Project

