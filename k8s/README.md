# Shelly Manager Kubernetes Deployment

This directory contains Kubernetes manifests for deploying Shelly Manager in a production-ready configuration.

## Files

- `deployment.yaml` - Main application deployment with PersistentVolume and Service
- `configmap.yaml` - Configuration data and secrets
- `ingress.yaml` - Ingress controller configuration with TLS
- `kustomization.yaml` - Kustomize configuration for environment management
- `README.md` - This deployment guide

## Prerequisites

- Kubernetes cluster (v1.19+)
- NGINX Ingress Controller
- StorageClass for persistent volumes
- kubectl configured for your cluster

## Quick Deployment

```bash
# Deploy with kubectl
kubectl apply -k .

# Or deploy individual files
kubectl apply -f configmap.yaml
kubectl apply -f deployment.yaml
kubectl apply -f ingress.yaml
```

## Configuration

### Storage

The deployment uses a PersistentVolume for SQLite database persistence:
- **Size**: 2Gi
- **Access Mode**: ReadWriteOnce
- **Path**: `/var/lib/shelly-manager` (hostPath)
- **StorageClass**: `shelly-storage`

For production, consider using a proper StorageClass (e.g., SSD-backed) instead of hostPath.

### Networking

- **Internal Service**: `shelly-manager-service:80`
- **Metrics Endpoint**: `shelly-manager-service:9090`
- **Ingress Hosts**: 
  - `shelly-manager.local` (development)
  - `shelly.example.com` (replace with your domain)

### Security

- Runs as non-root user (UID: 10001)
- Read-only root filesystem
- Dropped ALL capabilities
- Security context with seccomp profile
- Resource limits enforced

## Customization

### Environment-Specific Configuration

Use Kustomize overlays for different environments:

```bash
# Create overlays directory
mkdir -p overlays/production overlays/staging

# Create production kustomization
cat > overlays/production/kustomization.yaml << EOF
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../../

patchesStrategicMerge:
- replica-count.yaml
- resource-limits.yaml

images:
- name: shelly-manager
  newTag: v0.4.2-alpha
EOF

# Deploy to production
kubectl apply -k overlays/production/
```

### TLS Configuration

1. Generate TLS certificate:
```bash
# Self-signed (development)
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout tls.key -out tls.crt \
  -subj "/CN=shelly-manager.local"

# Create TLS secret
kubectl create secret tls shelly-manager-tls \
  --cert=tls.crt --key=tls.key
```

2. For production, use cert-manager:
```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: shelly-manager-tls
spec:
  secretName: shelly-manager-tls
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
  dnsNames:
  - shelly.example.com
```

### Database Backup

Set up periodic backups of the SQLite database:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: shelly-manager-backup
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: alpine:latest
            command:
            - sh
            - -c
            - |
              apk add --no-cache sqlite
              sqlite3 /data/shelly.db ".backup /backup/shelly-$(date +%Y%m%d).db"
            volumeMounts:
            - name: data-volume
              mountPath: /data
            - name: backup-volume
              mountPath: /backup
          restartPolicy: OnFailure
          volumes:
          - name: data-volume
            persistentVolumeClaim:
              claimName: shelly-manager-data
          - name: backup-volume
            # Configure your backup storage
```

## Monitoring

The application exposes metrics on port 9090:
- `/metrics` - Prometheus metrics endpoint

Example Prometheus configuration:
```yaml
- job_name: 'shelly-manager'
  static_configs:
  - targets: ['shelly-manager-service:9090']
  scrape_interval: 30s
  metrics_path: /metrics
```

## Troubleshooting

### Check Pod Status
```bash
kubectl get pods -l app=shelly-manager
kubectl describe pod <pod-name>
kubectl logs <pod-name>
```

### Check Storage
```bash
kubectl get pv,pvc -l app=shelly-manager
kubectl describe pvc shelly-manager-data
```

### Check Service and Ingress
```bash
kubectl get svc,ingress -l app=shelly-manager
kubectl describe ingress shelly-manager-ingress
```

### Access Application
```bash
# Port forward for testing
kubectl port-forward svc/shelly-manager-service 8080:80

# Check health
curl http://localhost:8080/health
```

## Scaling

For horizontal scaling, consider:
1. Use external database (PostgreSQL) instead of SQLite
2. Implement session affinity or stateless sessions
3. Configure shared storage for device discovery coordination

## Security Considerations

1. **Network Policies**: Restrict pod-to-pod communication
2. **RBAC**: Create service account with minimal permissions  
3. **Pod Security Standards**: Enforce restricted pod security
4. **Secrets Management**: Use external secret management (Vault, etc.)
5. **Image Security**: Scan container images for vulnerabilities