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
│   ├── secrets.yaml      # Secret keys (empty; fill before a deploy)
│   └── kustomization.yaml # Base kustomization
├── overlays/              # Environment-specific customizations
│   ├── development/       # Development environment
│   │   ├── kustomization.yaml
│   │   ├── configmap-dev.yaml
│   │   └── ingress-dev.yaml
│   ├── staging/          # Staging environment
│   │   ├── kustomization.yaml
│   │   ├── ingress-staging.yaml
│   │   ├── replica-patch.yaml
│   │   └── resources-patch.yaml
│   └── production/       # Production environment
│       ├── kustomization.yaml
│       ├── hpa.yaml
│       ├── ingress-production.yaml
│       ├── replica-patch.yaml
│       ├── resources-patch.yaml
│       └── servicemonitor.yaml
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
  # Default configuration values. The app reads only SHELLY_ + the config key,
  # with "." as "_": a name without the prefix has no effect.
  SHELLY_SERVER_PORT: "8080"
  SHELLY_SERVER_HOST: "0.0.0.0"
  SHELLY_LOGGING_LEVEL: "info"
  SHELLY_LOGGING_FORMAT: "json"
  SHELLY_DATABASE_PROVIDER: "sqlite"
  SHELLY_DATABASE_PATH: "/data/shelly.db"
  SHELLY_DISCOVERY_ENABLED: "true"
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
  type: ClusterIP
  ports:
  - name: http
    port: 80
    targetPort: http
    protocol: TCP
  - name: metrics
    port: 9090
    targetPort: metrics
    protocol: TCP
  selector:
    app: shelly-manager
    component: api-server
```

The file holds a second Service, `shelly-provisioner`, for the provisioner's metrics port.

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

metadata:
  name: shelly-manager-development

resources:
- ../../base

namePrefix: dev-
namespace: shelly-dev

commonLabels:
  environment: development

patchesStrategicMerge:
- configmap-dev.yaml
- ingress-dev.yaml

configMapGenerator:
- name: shelly-manager-config
  behavior: merge
  literals:
  - SHELLY_LOGGING_LEVEL=debug
  - SHELLY_DATABASE_PATH=/tmp/shelly.db
  - SHELLY_METRICS_ENABLE_DETAILED_TIMING=true
  # The provisioner's URL for the manager: this overlay's prefix, the Service's port 80.
  - SHELLY_API_URL=http://dev-shelly-manager:80

images:
- name: ghcr.io/ginsys/shelly-manager
  newTag: latest
- name: ghcr.io/ginsys/shelly-provisioner
  newTag: latest

replicas:
- name: shelly-manager
  count: 1
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

metadata:
  name: shelly-manager-staging

resources:
- ../../base

namePrefix: staging-
namespace: shelly-staging

commonLabels:
  environment: staging

patchesStrategicMerge:
- replica-patch.yaml
- resources-patch.yaml
- ingress-staging.yaml

configMapGenerator:
- name: shelly-manager-config
  behavior: merge
  literals:
  - SHELLY_LOGGING_LEVEL=info
  - SHELLY_DATABASE_PROVIDER=postgresql
  # No credentials in the DSN: the app adds SHELLY_DATABASE_USERNAME and
  # SHELLY_DATABASE_PASSWORD (the Secret) to it, with the escaping a password needs.
  - SHELLY_DATABASE_DSN=postgresql://postgresql:5432/shelly
  # Per replica: 24 connections at 2 replicas with maxSurge 1.
  - SHELLY_DATABASE_MAX_OPEN_CONNS=8
  - SHELLY_DATABASE_MAX_IDLE_CONNS=2
  # The provisioner's URL for the manager: this overlay's prefix, the Service's port 80.
  - SHELLY_API_URL=http://staging-shelly-manager:80

images:
- name: ghcr.io/ginsys/shelly-manager
  newTag: staging-latest
- name: ghcr.io/ginsys/shelly-provisioner
  newTag: staging-latest

replicas:
- name: shelly-manager
  count: 2
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

metadata:
  name: shelly-manager-production

resources:
- ../../base
- servicemonitor.yaml
- hpa.yaml

namePrefix: prod-
namespace: shelly-production

commonLabels:
  environment: production
  version: v1.0.0

patchesStrategicMerge:
- replica-patch.yaml
- resources-patch.yaml
- ingress-production.yaml

configMapGenerator:
- name: shelly-manager-config
  behavior: merge
  literals:
  - SHELLY_LOGGING_LEVEL=info
  - SHELLY_DATABASE_PROVIDER=postgresql
  # No credentials in the DSN: the app adds SHELLY_DATABASE_USERNAME and
  # SHELLY_DATABASE_PASSWORD (the Secret) to it, with the escaping a password needs.
  - SHELLY_DATABASE_DSN=postgresql://postgresql:5432/shelly
  # Per replica. 80 connections at the HPA maximum of 10 replicas, 88 during a
  # roll (maxSurge 1), against a PostgreSQL max_connections of 100 (the default).
  - SHELLY_DATABASE_MAX_OPEN_CONNS=8
  - SHELLY_DATABASE_MAX_IDLE_CONNS=2
  - SHELLY_METRICS_RETENTION_DAYS=90
  # The provisioner's URL for the manager: this overlay's prefix, the Service's port 80.
  - SHELLY_API_URL=http://prod-shelly-manager:80

images:
- name: ghcr.io/ginsys/shelly-manager
  newTag: 20250825-143022-abc1234  # Specific production version
- name: ghcr.io/ginsys/shelly-provisioner
  newTag: 20250825-143022-abc1234

replicas:
- name: shelly-manager
  count: 3
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
SHELLY_DATABASE_PROVIDER: "sqlite"
SHELLY_DATABASE_PATH: "/tmp/shelly.db"
```

**Staging and production**: PostgreSQL (persistent)
```yaml
SHELLY_DATABASE_PROVIDER: "postgresql"
SHELLY_DATABASE_DSN: "postgresql://postgresql:5432/shelly"   # no credentials
SHELLY_DATABASE_MAX_OPEN_CONNS: "8"                          # per replica
SHELLY_DATABASE_MAX_IDLE_CONNS: "2"
```

The credentials stay out of the DSN and out of every ConfigMap: `SHELLY_DATABASE_USERNAME` and
`SHELLY_DATABASE_PASSWORD` come from the `database-username` and `database-password` keys of the
`shelly-manager-secrets` Secret (`base/deployment.yaml`), and the app puts them into the DSN with the
escaping a password needs. Fill the Secret before you deploy a staging or production overlay: its keys
are empty in `base/secrets.yaml`. A DSN that carries credentials as well is refused at start.
The overlays switch the pod from the SQLite config baked into the image to PostgreSQL; the
database must exist and be reachable at `postgresql:5432` in the namespace.

Each replica opens its own pool, so replicas × `SHELLY_DATABASE_MAX_OPEN_CONNS` must stay below the
server's `max_connections` (PostgreSQL's default is 100, 97 usable; an assumption, this repository
holds no server manifest). Production: the autoscaler runs 3 to 10 replicas and a rolling update adds
one, so 8 per replica is 24 connections at rest, 80 at 10 replicas and 88 during a roll at 10.
Staging: 2 replicas and one more during a roll, 24.

Every environment name in these manifests is checked by `internal/config/deploy_env_names_test.go`.

### Secrets Management
```bash
# Create secrets manually
kubectl create secret generic shelly-manager-secrets \
  --from-literal=database-password="your-secure-password" \
  --from-literal=admin-api-key="your-admin-key" \
  --from-literal=agent-api-key="a-different-random-key"

# Or use external secret management
kubectl apply -f external-secrets.yaml
```

Two keys, two jobs. `admin-api-key` is the manager's admin key: it opens every guarded route.
`agent-api-key` is the provisioner agents' key. The manager reads it as `SHELLY_SECURITY_AGENT_API_KEY`
and accepts it on the five routes an agent calls (register, poll tasks, task status, report discovered
devices, health) and on no other route; the DaemonSet reads it as `SHELLY_API_KEY`. Both are
`secretKeyRef` entries in `base/deployment.yaml`. Use two different values: a compromised node then
holds a key that cannot create tasks or read anything else, and rotating the admin key leaves the
agents working.

Set `agent-api-key` whenever you set `admin-api-key`. While it is empty and the manager has an admin
key, the agents get 401. To rotate the agent key, change the Secret and restart the manager and the
DaemonSet; there is no API call for it, because an API rotation changes only the manager's memory and
would leave every agent with the old key. The agent key does nothing while the admin key is empty: with
no admin key the guarded routes stay open.

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