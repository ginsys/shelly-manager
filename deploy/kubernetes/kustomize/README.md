# Kustomize Deployments

Kustomize provides declarative YAML-based Kubernetes deployments with environment-specific customizations through overlays.

## 📁 Directory Structure

```
kustomize/
├── base/                   # Base Kubernetes manifests
│   ├── configmap.yaml     # Application configuration
│   ├── deployment.yaml    # Application deployments  
│   ├── service.yaml       # Service definitions
│   ├── ingress.yaml       # Ingress configuration
│   ├── pvc.yaml          # Persistent volume claims
│   └── kustomization.yaml # Base kustomization
├── overlays/              # Environment-specific customizations
│   ├── development/       # Development environment
│   │   ├── kustomization.yaml
│   │   ├── configmap-dev.yaml
│   │   └── ingress-dev.yaml
│   ├── staging/          # Staging environment
│   │   ├── kustomization.yaml
│   │   ├── configmap-staging.yaml
│   │   └── replica-patch.yaml
│   └── production/       # Production environment
│       ├── kustomization.yaml
│       ├── configmap-prod.yaml
│       ├── replica-patch.yaml
│       ├── resources-patch.yaml
│       └── ingress-prod.yaml
└── README.md             # This documentation
```

## 🚀 Quick Start

### Prerequisites
- Kubernetes cluster (v1.20+)
- kubectl with kustomize support (v1.14+)
- Container images available at ghcr.io registry

### Deploy Development Environment
```bash
# Deploy development configuration
kubectl apply -k deploy/kubernetes/kustomize/overlays/development/

# Check deployment status
kubectl get pods -l app=shelly-manager

# View services
kubectl get services -l app=shelly-manager
```

### Deploy Production Environment
```bash
# Deploy production configuration
kubectl apply -k deploy/kubernetes/kustomize/overlays/production/

# Check deployment status
kubectl get all -l app=shelly-manager

# View ingress
kubectl get ingress shelly-manager
```

## 📋 Base Manifests

### ConfigMap (base/configmap.yaml)
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: shelly-manager-config
data:
  # Default configuration values
  SERVER_PORT: "8080"
  SERVER_HOST: "0.0.0.0"
  LOG_LEVEL: "info"
  LOG_FORMAT: "json"
  DATABASE_PROVIDER: "sqlite"
  DISCOVERY_ENABLED: "true"
  # ... additional config
```

### Deployment (base/deployment.yaml)
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: shelly-manager
spec:
  replicas: 1
  selector:
    matchLabels:
      app: shelly-manager
  template:
    spec:
      containers:
      - name: manager
        image: ghcr.io/ginsys/shelly-manager:latest
        ports:
        - containerPort: 8080
        envFrom:
        - configMapRef:
            name: shelly-manager-config
        # Health checks, resources, etc.
```

### Service (base/service.yaml)
```yaml
apiVersion: v1
kind: Service
metadata:
  name: shelly-manager
spec:
  selector:
    app: shelly-manager
  ports:
  - port: 80
    targetPort: 8080
    name: http
  - port: 9090
    targetPort: 9090
    name: metrics
```

## 🎯 Environment Overlays

### Development Overlay
**Features:**
- Single replica
- Debug logging
- Local storage
- Ingress with basic auth
- Resource limits relaxed

```yaml
# overlays/development/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../../base

patchesStrategicMerge:
- configmap-dev.yaml

configMapGenerator:
- name: shelly-manager-config
  behavior: merge
  literals:
  - LOG_LEVEL=debug
  - GIN_MODE=debug

images:
- name: ghcr.io/ginsys/shelly-manager
  newTag: latest
```

### Staging Overlay
**Features:**
- 2 replicas for availability testing
- Production-like configuration
- Resource limits enforced
- Staging domain ingress

```yaml
# overlays/staging/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../../base

patchesStrategicMerge:
- replica-patch.yaml
- configmap-staging.yaml

images:
- name: ghcr.io/ginsys/shelly-manager
  newTag: staging-latest
```

### Production Overlay
**Features:**
- Multiple replicas (3+)
- Production configuration
- Resource limits enforced
- TLS-enabled ingress
- Monitoring labels

```yaml
# overlays/production/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../../base

patchesStrategicMerge:
- replica-patch.yaml
- resources-patch.yaml
- configmap-prod.yaml
- ingress-prod.yaml

configMapGenerator:
- name: shelly-manager-config
  behavior: merge
  literals:
  - LOG_LEVEL=info
  - GIN_MODE=release
  - DATABASE_PROVIDER=postgresql

images:
- name: ghcr.io/ginsys/shelly-manager
  newTag: v1.0.0  # Specific version tag

commonLabels:
  environment: production
  version: v1.0.0
```

## 🔧 Configuration Management

### Environment Variables
Configuration priority (highest to lowest):
1. **Overlay-specific ConfigMaps**
2. **Base ConfigMap**
3. **Container defaults**

### Database Configuration

**Development**: SQLite (ephemeral)
```yaml
DATABASE_PROVIDER: "sqlite"
DATABASE_PATH: "/tmp/shelly.db"
```

**Production**: PostgreSQL (persistent)
```yaml
DATABASE_PROVIDER: "postgresql"
DATABASE_DSN: "postgresql://user:pass@postgres:5432/shelly?sslmode=require"
```

### Secrets Management
```bash
# Create secrets manually
kubectl create secret generic shelly-manager-secrets \
  --from-literal=database-password="your-secure-password" \
  --from-literal=api-key="your-api-key"

# Or use external secret management
kubectl apply -f external-secrets.yaml
```

## 🔀 Customization Patterns

### Image Tag Management
```yaml
# Development (latest builds)
images:
- name: ghcr.io/ginsys/shelly-manager
  newTag: latest

# Production (specific versions)  
images:
- name: ghcr.io/ginsys/shelly-manager
  newTag: 20250825-143022-abc1234
```

### Resource Customization
```yaml
# resources-patch.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: shelly-manager
spec:
  template:
    spec:
      containers:
      - name: manager
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi" 
            cpu: "500m"
```

### Replica Scaling
```yaml
# replica-patch.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: shelly-manager
spec:
  replicas: 3
```

## 🛡️ Security Configuration

### Security Context
```yaml
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 10001
    fsGroup: 10001
  containers:
  - name: manager
    securityContext:
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities:
        drop: ["ALL"]
```

### Network Policies
```yaml
# network-policy.yaml (can be added as resource)
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: shelly-manager-netpol
spec:
  podSelector:
    matchLabels:
      app: shelly-manager
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-system
    ports:
    - protocol: TCP
      port: 8080
```

## 📊 Monitoring Integration

### ServiceMonitor for Prometheus
```yaml
# monitoring/servicemonitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: shelly-manager
spec:
  selector:
    matchLabels:
      app: shelly-manager
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

## 🚀 Deployment Commands

### Deploy Specific Environment
```bash
# Development
kubectl apply -k deploy/kubernetes/kustomize/overlays/development/

# Staging
kubectl apply -k deploy/kubernetes/kustomize/overlays/staging/

# Production
kubectl apply -k deploy/kubernetes/kustomize/overlays/production/
```

### Preview Configuration
```bash
# See generated YAML without applying
kubectl kustomize deploy/kubernetes/kustomize/overlays/production/

# Save to file for review
kubectl kustomize deploy/kubernetes/kustomize/overlays/production/ > production-manifest.yaml
```

### Update Deployments
```bash
# Update image tag
cd deploy/kubernetes/kustomize/overlays/production/
kustomize edit set image ghcr.io/ginsys/shelly-manager:20250825-143022-abc1234
kubectl apply -k ./
```

### Clean Up
```bash
# Remove deployment
kubectl delete -k deploy/kubernetes/kustomize/overlays/production/

# Or delete by label
kubectl delete all -l app=shelly-manager
```

## 🔧 Development Workflow

### 1. Local Development
```bash
# Test configuration locally
kubectl kustomize deploy/kubernetes/kustomize/overlays/development/ | kubectl apply --dry-run=client -f -

# Deploy to development namespace
kubectl create namespace shelly-dev
kubectl apply -k deploy/kubernetes/kustomize/overlays/development/ -n shelly-dev
```

### 2. GitOps Integration
```yaml
# .github/workflows/deploy.yml
name: Deploy to Kubernetes
on:
  push:
    branches: [main]
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v5
    - name: Deploy to staging
      run: |
        kubectl apply -k deploy/kubernetes/kustomize/overlays/staging/
```

## 🛠️ Troubleshooting

### Common Issues

1. **Configuration Not Applied**
   ```bash
   # Check configmap
   kubectl describe configmap shelly-manager-config
   
   # Verify environment variables in pod
   kubectl exec -it deployment/shelly-manager -- env | grep SHELLY_
   ```

2. **Image Pull Issues**
   ```bash
   # Check if image exists
   kubectl describe pod <pod-name>
   
   # Test image pull manually
   kubectl run test --image=ghcr.io/ginsys/shelly-manager:latest --rm -it
   ```

3. **Kustomize Build Errors**
   ```bash
   # Validate kustomization.yaml
   kubectl kustomize deploy/kubernetes/kustomize/overlays/production/
   
   # Check for YAML syntax errors
   yamllint deploy/kubernetes/kustomize/overlays/production/kustomization.yaml
   ```

## 📚 Best Practices

1. **Environment Isolation**: Use namespaces for different environments
2. **Version Control**: Tag base images with specific versions in production
3. **Resource Limits**: Always set resource requests and limits
4. **Security**: Use non-root containers and read-only filesystems
5. **Monitoring**: Include ServiceMonitor for Prometheus integration
6. **Documentation**: Document all customizations in overlay READMEs

## 🔗 Related Documentation

- [Base Kubernetes Documentation](../README.md)
- [Helm Alternative](../helm/README.md) 
- [Docker Compose for Local Development](../../docker-compose/README.md)