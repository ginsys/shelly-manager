# Secrets Management and Secure Configuration

This guide shows how to configure sensitive settings with environment variables and Kubernetes Secrets (or mounted secret files), and how to enable safe export downloads.

## Environment Variables (12-factor)

The app supports environment overrides via `SHELLY_` prefix and dot→underscore mapping. Sensitive fields also support the `_FILE` convention (value read from file).

Supported secret keys (value or `_FILE`):
- SHELLY_SECURITY_ADMIN_API_KEY
- SHELLY_NOTIFICATIONS_EMAIL_SMTP_PASSWORD
- SHELLY_OPNSENSE_API_KEY
- SHELLY_OPNSENSE_API_SECRET
- SHELLY_API_KEY (provisioner agent)

Other relevant config keys:
- SHELLY_EXPORT_OUTPUT_DIRECTORY (safe download base directory)

Example (Docker Compose `.env`):

```
ADMIN_API_KEY=use-a-strong-long-random-secret
EXPORT_OUTPUT_DIR=/data/exports
```

Compose files pass these through as:

```
environment:
  - SHELLY_SECURITY_ADMIN_API_KEY=${ADMIN_API_KEY}
  - SHELLY_EXPORT_OUTPUT_DIRECTORY=${EXPORT_OUTPUT_DIR}
  # SMTP (prefer using *_FILE mounted from Docker secrets)
  - SHELLY_NOTIFICATIONS_EMAIL_SMTP_PASSWORD=${SMTP_PASSWORD:-}
  # OPNSense
  - SHELLY_OPNSENSE_API_KEY=${OPNSENSE_API_KEY:-}
  - SHELLY_OPNSENSE_API_SECRET=${OPNSENSE_API_SECRET:-}
  # Provisioner agent
  - SHELLY_API_KEY=${API_KEY:-}
```

### Docker secrets / mounted files (`*_FILE`)

For Docker/Compose or Kubernetes mounts, point `*_FILE` env vars to file paths:

```
environment:
  - SHELLY_NOTIFICATIONS_EMAIL_SMTP_PASSWORD_FILE=/run/secrets/smtp_password
  - SHELLY_OPNSENSE_API_KEY_FILE=/run/secrets/opnsense_key
  - SHELLY_OPNSENSE_API_SECRET_FILE=/run/secrets/opnsense_secret
  - SHELLY_SECURITY_ADMIN_API_KEY_FILE=/run/secrets/admin_key
volumes:
  - ./secrets:/run/secrets:ro
```

## Kubernetes Secrets

Create a Secret:

```
kubectl create secret generic shelly-manager-secrets \
  --from-literal=ADMIN_API_KEY="use-a-strong-long-random-secret"
```

Reference it in a Deployment (Kustomize/Helm overlays):

```
env:
  - name: SHELLY_SECURITY_ADMIN_API_KEY
    valueFrom:
      secretKeyRef:
        name: shelly-manager-secrets
        key: ADMIN_API_KEY
  - name: SHELLY_EXPORT_OUTPUT_DIRECTORY
    value: /data/exports
  - name: SHELLY_NOTIFICATIONS_EMAIL_SMTP_PASSWORD
    valueFrom:
      secretKeyRef:
        name: shelly-manager-secrets
        key: SMTP_PASSWORD
  - name: SHELLY_OPNSENSE_API_KEY
    valueFrom:
      secretKeyRef:
        name: shelly-manager-secrets
        key: OPNSENSE_API_KEY
  - name: SHELLY_OPNSENSE_API_SECRET
    valueFrom:
      secretKeyRef:
        name: shelly-manager-secrets
        key: OPNSENSE_API_SECRET
```

Note: You can also mount secrets as files and use the `*_FILE` variant, but `secretKeyRef` to env is simplest in Kubernetes.

## Config File (configs/shelly-manager.yaml)

Relevant keys for reference (can be overridden by env):

```
security:
  admin_api_key: ""
export:
  output_directory: ""

notifications:
  email:
    smtp_password: ""

opnsense:
  api_key: ""
  api_secret: ""
```

## Security Tips

- Never commit secrets to git. Use `.env` (local), K8s Secrets/Secret Store in production.
- Rotate admin keys periodically and restrict who can access them.
- Prefer a dedicated exports directory (e.g., `/data/exports`) and ensure container permissions are appropriate.

### Admin Key Rotation API

Rotate the in-memory admin key without restarts:

```
curl -X POST http://localhost:8080/api/v1/admin/rotate-admin-key \
  -H "Authorization: Bearer $OLD_ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"new_key": "REDACTED_NEW_KEY"}'
```

Notes:
- Requires current admin key via `Authorization: Bearer ...` or `X-API-Key`.
- Rotates key for protected HTTP routes and the metrics WebSocket.
- For persistence across restarts, update your secret source (K8s Secret, Docker secret, `.env`).
