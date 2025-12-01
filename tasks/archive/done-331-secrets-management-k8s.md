# Secrets Management (K8s)

**Priority**: MEDIUM - Important Feature
**Status**: completed
**Effort**: 4-6 hours
**Completed**: 2025-12-01

## Context

Docker Compose `.env.example` exists (238 lines) and K8s ConfigMaps are complete, but sensitive values (SMTP credentials, OPNSense API keys) should be in Kubernetes Secrets instead of ConfigMaps.

## Success Criteria

- [x] K8s Secrets manifest created for sensitive config
- [x] SMTP credentials stored in Secrets
- [x] OPNSense credentials stored in Secrets
- [x] Documentation updated with secrets usage
- [x] Existing ConfigMaps reference Secrets properly

## Implementation

### Step 1: Create Secrets Manifest

**File**: `deployments/k8s/secrets.yaml`

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: shelly-manager-secrets
  namespace: shelly-manager
type: Opaque
stringData:
  # Email/SMTP credentials
  smtp-password: ""  # Set via kubectl or sealed-secrets

  # OPNSense API credentials
  opnsense-api-key: ""
  opnsense-api-secret: ""

  # Admin API key (for protected endpoints)
  admin-api-key: ""

  # Database credentials (if using PostgreSQL/MySQL)
  database-password: ""
```

### Step 2: Update Deployment to Reference Secrets

**File**: `deployments/k8s/deployment.yaml`

```yaml
env:
  - name: SHELLY_NOTIFICATIONS_EMAIL_SMTP_PASSWORD
    valueFrom:
      secretKeyRef:
        name: shelly-manager-secrets
        key: smtp-password
  - name: SHELLY_OPNSENSE_API_KEY
    valueFrom:
      secretKeyRef:
        name: shelly-manager-secrets
        key: opnsense-api-key
  - name: SHELLY_OPNSENSE_API_SECRET
    valueFrom:
      secretKeyRef:
        name: shelly-manager-secrets
        key: opnsense-api-secret
  - name: SHELLY_SECURITY_ADMIN_API_KEY
    valueFrom:
      secretKeyRef:
        name: shelly-manager-secrets
        key: admin-api-key
```

### Step 3: Create Sealed Secrets (Optional)

For GitOps workflows, use Bitnami Sealed Secrets:

```bash
# Install kubeseal
brew install kubeseal

# Seal the secrets
kubeseal --format yaml < secrets.yaml > sealed-secrets.yaml
```

### Step 4: Update Documentation

**File**: `docs/deployment/kubernetes.md`

Add section on secrets management.

## Completed Work

- [x] Docker Compose `.env.example` exists (238 lines)
- [x] K8s ConfigMaps exist with full configuration

## Remaining Work

- [ ] Create Secrets manifest
- [ ] Update Deployment to use Secrets
- [ ] Add sealed-secrets support (optional)
- [ ] Update documentation

## Validation

```bash
# Verify secrets created
kubectl get secrets -n shelly-manager

# Verify deployment uses secrets
kubectl describe deployment shelly-manager -n shelly-manager | grep -A5 "Environment:"

# Test that application starts with secrets
kubectl logs -f deployment/shelly-manager -n shelly-manager
```

## Dependencies

None

## Risk

Low - Standard K8s pattern
