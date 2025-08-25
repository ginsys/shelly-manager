# Kubernetes Deployment Options

This directory contains multiple Kubernetes deployment options for Shelly Manager, each optimized for different use cases and operational preferences.

## 📁 Directory Structure

```
kubernetes/
├── kustomize/              # Kustomize-based deployments
│   ├── base/              # Base manifests and resources
│   └── overlays/          # Environment-specific customizations
├── helm/                  # Helm chart deployments  
│   └── shelly-manager/    # Helm chart for flexible deployments
├── kurel/                 # Future: Kurel-based deployments
└── README.md              # This overview
```

## 🚀 Deployment Options Overview

| Method | Best For | Complexity | Customization | Enterprise |
|--------|----------|------------|---------------|------------|
| **Kustomize** | GitOps, simple customization | Low | Medium | ✅ |
| **Helm** | Complex deployments, templating | Medium | High | ✅ |
| **Kurel** | Future: Simplified operations | Low | Low | 📋 |

## 1. 🔧 Kustomize Deployments

**Best for**: GitOps workflows, infrastructure-as-code, simple environment variations

```bash
# Quick deployment
kubectl apply -k deploy/kubernetes/kustomize/overlays/production/

# Development environment
kubectl apply -k deploy/kubernetes/kustomize/overlays/development/
```

**Features**:
- ✅ Simple YAML-based configuration
- ✅ Environment-specific overlays
- ✅ Built-in Kubernetes tooling
- ✅ GitOps-friendly
- ✅ No additional dependencies

[📖 **Detailed Kustomize Documentation →**](./kustomize/README.md)

---

## 2. ⎈ Helm Chart Deployments

**Best for**: Complex deployments, templating, package management, multiple environments

```bash
# Install from local chart
helm install my-shelly-manager ./deploy/kubernetes/helm/shelly-manager/

# Install with custom values
helm install my-shelly-manager ./deploy/kubernetes/helm/shelly-manager/ \
  --values ./deploy/kubernetes/helm/shelly-manager/values-production.yaml
```

**Features**:
- ✅ Advanced templating with Go templates
- ✅ Flexible configuration via values.yaml
- ✅ Package management and versioning
- ✅ Dependency management
- ✅ Rollback and upgrade support

[📖 **Detailed Helm Documentation →**](./helm/README.md)

---

## 3. 🔮 Kurel Deployments (Future)

**Best for**: Future simplified Kubernetes operations, beginner-friendly deployments

```bash
# Future usage example
kurel deploy shelly-manager --env production
kurel scale shelly-manager --replicas 3
```

**Planned Features**:
- 📋 Simplified deployment commands
- 📋 Automatic best practices
- 📋 Built-in monitoring setup
- 📋 Easy scaling operations
- 📋 Integrated troubleshooting

[📖 **Kurel Planning Documentation →**](./kurel/README.md)

---

## 🎯 Choosing the Right Deployment Method

### Use **Kustomize** when:
- You prefer declarative YAML configuration
- You're implementing GitOps workflows
- You need simple environment customizations
- You want to use built-in Kubernetes tooling
- You're working with infrastructure-as-code

### Use **Helm** when:
- You need complex templating and logic
- You're managing multiple environments with significant differences
- You want package management and versioning
- You need dependency management
- You're building reusable deployment packages

### Use **Kurel** when (future):
- You want simplified Kubernetes operations
- You're new to Kubernetes deployments
- You need guided deployment workflows
- You want automated best practices

---

## 🔧 Common Prerequisites

All deployment methods require:

- **Kubernetes cluster** (v1.20+)
- **kubectl** configured and connected to cluster
- **Container images** available at `ghcr.io/ginsys/shelly-manager:latest`

Additional requirements per method:
- **Kustomize**: Built into kubectl 1.14+
- **Helm**: Helm CLI installed (v3.0+)
- **Kurel**: TBA (future)

---

## 📊 Resource Requirements

### Minimum Resources (Development)
```yaml
shelly-manager:
  requests: { memory: "128Mi", cpu: "100m" }
  limits: { memory: "256Mi", cpu: "250m" }

shelly-provisioner:
  requests: { memory: "64Mi", cpu: "50m" }
  limits: { memory: "128Mi", cpu: "100m" }
```

### Recommended Resources (Production)
```yaml
shelly-manager:
  requests: { memory: "256Mi", cpu: "250m" }
  limits: { memory: "512Mi", cpu: "500m" }

shelly-provisioner:
  requests: { memory: "128Mi", cpu: "100m" }
  limits: { memory: "256Mi", cpu: "250m" }
```

---

## 🔐 Security Considerations

### Network Security
- **shelly-manager**: Runs in cluster network (standard security)
- **shelly-provisioner**: Requires `hostNetwork: true` for WiFi access

### RBAC
- Minimal RBAC permissions for service accounts
- No cluster-admin privileges required
- Network policies for traffic isolation

### Container Security
- Non-root containers (UID 10001)
- Read-only root filesystems
- Dropped Linux capabilities
- Security contexts enforced

---

## 📈 Monitoring & Observability

All deployment methods include:

- **Health Checks**: Liveness and readiness probes
- **Metrics**: Prometheus metrics on port 9090
- **Logging**: Structured JSON logs to stdout
- **Service Monitors**: Automatic Prometheus discovery

---

## 🆘 Troubleshooting

### Common Issues

1. **WiFi Provisioner Not Working**
   ```bash
   # Check if running on node with WiFi
   kubectl describe node <node-name>
   
   # Verify privileged security context
   kubectl get pod <provisioner-pod> -o yaml | grep -A5 securityContext
   ```

2. **Database Persistence Issues**
   ```bash
   # Check PVC status
   kubectl get pvc
   
   # Verify storage class
   kubectl get storageclass
   ```

3. **Image Pull Issues**
   ```bash
   # Check image availability
   docker pull ghcr.io/ginsys/shelly-manager:latest
   
   # Verify registry access from cluster
   kubectl run test --image=ghcr.io/ginsys/shelly-manager:latest --rm -it --restart=Never -- /bin/sh
   ```

### Getting Help

1. **Check pod logs**: `kubectl logs -f deployment/shelly-manager`
2. **Check events**: `kubectl get events --sort-by=.metadata.creationTimestamp`
3. **Describe resources**: `kubectl describe deployment shelly-manager`

---

## 🤝 Contributing

To contribute to Kubernetes deployments:

1. Test changes with local cluster (minikube, kind, k3s)
2. Validate with different Kubernetes versions
3. Ensure security best practices
4. Update documentation for new features
5. Test with both development and production configurations

---

## 🗺️ Migration Path

```
Docker Compose → Kustomize → Helm → Kurel (future)
```

Each method builds upon the previous, allowing gradual migration as operational requirements grow in complexity.