# Secrets Management and Secure Configuration

This guide shows how to configure sensitive settings with environment variables and Kubernetes Secrets (or mounted secret files), and how to enable safe export downloads.

## Environment Variables (12-factor)

The app supports environment overrides via `SHELLY_` prefix and dot→underscore mapping. Sensitive fields also support the `_FILE` convention (value read from file).

Supported secret keys (value or `_FILE`):
- SHELLY_SECURITY_ADMIN_API_KEY
- SHELLY_SECURITY_AGENT_API_KEY (the manager's agent key)
- SHELLY_NOTIFICATIONS_EMAIL_SMTP_PASSWORD
- SHELLY_OPNSENSE_API_KEY
- SHELLY_OPNSENSE_API_SECRET
- SHELLY_API_KEY (provisioner agent: send it the agent key, never the admin key)
- SHELLY_PROVISIONING_AGENT_SEED (provisioner agent: the seed that makes its signing key; 64 hexadecimal characters. `shelly-provisioner seed` prints a new one, so its output belongs in the secret store and nowhere else. The agent never logs, prints or sends it, and it has no flag. The manager reads the variable too, because both programs share one table, and uses it nowhere.)

`SHELLY_SECURITY_AGENT_API_KEY` also takes the `_FILE` form. Kubernetes can keep using `secretKeyRef`.

A plain variable that has a value wins over its `_FILE` form. An empty value counts as not set, for the variable and for the file content: an empty variable falls through to the `_FILE` form, and with neither a value nor a readable, non-empty file the key from the config file stays. So a Compose file that always sets `SHELLY_SECURITY_ADMIN_API_KEY=${ADMIN_API_KEY:-}` or `SHELLY_SECURITY_AGENT_API_KEY=${AGENT_API_KEY:-}` (the shipped one does) can carry the `_FILE` line beside the plain line. A `_FILE` variable that is set to a path the process cannot read (a typo in a mount path, a missing Docker secret, a directory) stops the start-up with an error that names the variable and the path, because an empty admin key would leave every guarded route open. A plain variable with a value is used before the file is looked at, so it hides an unreadable file, and an unset or empty `_FILE` variable is not an error.

Two keys, two jobs. The admin key opens every guarded route. The agent key opens only the five routes the provisioner agent calls (register, poll tasks, update task status, report discovered devices, health), so a leaked agent key cannot reach the admin-only configuration, device or admin routes, and cannot create tasks. On its own it opens register and health only: each agent gets its own token when it registers, and poll, task status and report need that token, so one agent cannot read another agent's tasks or mark them done. In the default `open` enrolment mode (see below), a holder of the agent key can still register a new agent id and claim tasks nobody was assigned, and can take over an id that has been silent for 5 minutes, so treat the agent key as a secret too: a task carries its configuration (any Wi-Fi settings or device credentials in it). Use two different values. The agent key has no effect while the admin key is empty.

**Strict enrolment closes what the agent key still allows.** The agent key can register a new keyless agent id, claim tasks nobody was assigned and take over an id silent for 5 minutes. Setting `security.agent_enrolment` (`SHELLY_SECURITY_AGENT_ENROLMENT`) to `strict` ends all three for a keyless agent: the manager refuses a register without a public key with `403`, and the token of any agent that is not approved by its key authorizes nothing. In `strict` the agent key alone creates no keyless id, issues no token and opens no claim, so a leaked agent key lets nobody in; only the admin key holder can approve an agent. A holder of the agent key can still ask to enrol by key, which leaves a `pending` row (a soft limit of 20 at a time; concurrent registrations can exceed it) that works only once the admin key holder approves its fingerprint. `strict` needs an admin key and refuses to start without one. The default is `open`, which behaves as above. See the API overview, "Strict enrolment".

Other relevant config keys:
- SHELLY_EXPORT_OUTPUT_DIRECTORY (safe download base directory)

Example (Docker Compose `.env`):

```
ADMIN_API_KEY=use-a-strong-long-random-secret
AGENT_API_KEY=use-a-different-strong-random-secret
EXPORT_OUTPUT_DIR=/data/exports
```

Compose files pass these through as:

```
environment:
  - SHELLY_SECURITY_ADMIN_API_KEY=${ADMIN_API_KEY}
  - SHELLY_SECURITY_AGENT_API_KEY=${AGENT_API_KEY:-}
  - SHELLY_EXPORT_OUTPUT_DIRECTORY=${EXPORT_OUTPUT_DIR}
  # SMTP (prefer using *_FILE mounted from Docker secrets)
  - SHELLY_NOTIFICATIONS_EMAIL_SMTP_PASSWORD=${SMTP_PASSWORD:-}
  # OPNSense
  - SHELLY_OPNSENSE_API_KEY=${OPNSENSE_API_KEY:-}
  - SHELLY_OPNSENSE_API_SECRET=${OPNSENSE_API_SECRET:-}
  # Provisioner agent: sends the agent key (the admin key if AGENT_API_KEY is unset)
  - SHELLY_API_KEY=${AGENT_API_KEY:-${ADMIN_API_KEY:-}}
```

### Docker secrets / mounted files (`*_FILE`)

For Docker/Compose or Kubernetes mounts, point `*_FILE` env vars to file paths:

```
environment:
  - SHELLY_NOTIFICATIONS_EMAIL_SMTP_PASSWORD_FILE=/run/secrets/smtp_password
  - SHELLY_OPNSENSE_API_KEY_FILE=/run/secrets/opnsense_key
  - SHELLY_OPNSENSE_API_SECRET_FILE=/run/secrets/opnsense_secret
  - SHELLY_SECURITY_ADMIN_API_KEY_FILE=/run/secrets/admin_key
  - SHELLY_SECURITY_AGENT_API_KEY_FILE=/run/secrets/agent_api_key
volumes:
  - ./secrets:/run/secrets:ro
```

## Kubernetes Secrets

Create a Secret:

```
kubectl create secret generic shelly-manager-secrets \
  --from-literal=ADMIN_API_KEY="use-a-strong-long-random-secret" \
  --from-literal=AGENT_API_KEY="use-a-different-strong-random-secret"
```

Reference it in a Deployment (Kustomize/Helm overlays):

```
env:
  - name: SHELLY_SECURITY_ADMIN_API_KEY
    valueFrom:
      secretKeyRef:
        name: shelly-manager-secrets
        key: ADMIN_API_KEY
  - name: SHELLY_SECURITY_AGENT_API_KEY
    valueFrom:
      secretKeyRef:
        name: shelly-manager-secrets
        key: AGENT_API_KEY
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
  agent_api_key: ""
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
- Rotates key for protected HTTP routes and the metrics WebSocket. All of them share one key, so the switch is a single step: from the moment the call returns, the old key is refused everywhere and the new one is accepted everywhere. Each check reads the key once, so it sees either the old key or the new one. A few provisioner routes check more than once in one request (the agent-or-admin guard, then whether the caller may act for any agent). A rotation that lands between those checks can change the outcome of the later one, so such a request may be refused, or may skip an action that needs the admin key, and a retry gives the answer for the new key.
- For persistence across restarts, update your secret source (K8s Secret, Docker secret, `.env`).
- The agent key is not touched by this call, so agents keep working. There is no API to rotate it: change the secret source and restart the manager and the agents.
