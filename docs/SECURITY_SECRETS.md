# Secrets Management and Secure Configuration

This guide shows how to configure sensitive settings with environment variables and Kubernetes Secrets, and how to enable safe export downloads.

## Environment Variables (12-factor)

The app supports environment overrides via `SHELLY_` prefix and dot→underscore mapping. Key variables:

- SHELLY_SECURITY_ADMIN_API_KEY: Admin key protecting export/import endpoints.
- SHELLY_EXPORT_OUTPUT_DIRECTORY: Base directory restriction for export downloads.

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
```

Mounting more secrets (SMTP, OPNSense) follows the same pattern. Use `SHELLY_` keys that map to config fields (e.g., `SHELLY_NOTIFICATIONS_EMAIL_SMTP_PASSWORD`).

## Config File (configs/shelly-manager.yaml)

Relevant keys for reference (can be overridden by env):

```
security:
  admin_api_key: ""
export:
  output_directory: ""
```

## Security Tips

- Never commit secrets to git. Use `.env` locally and K8s Secrets in production.
- Rotate admin keys periodically and restrict who can access them.
- Prefer a dedicated exports directory (e.g., `/data/exports`) and ensure container permissions are appropriate.

