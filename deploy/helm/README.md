# Helm Chart for Shelly Manager

This directory will contain a Helm chart for easy deployment and management of Shelly Manager in Kubernetes clusters.

## Planned Features

### Chart Structure
```
helm/
├── shelly-manager/
│   ├── Chart.yaml          # Chart metadata
│   ├── values.yaml         # Default values
│   ├── values-dev.yaml     # Development values
│   ├── values-prod.yaml    # Production values
│   ├── templates/          # Kubernetes templates
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── ingress.yaml
│   │   ├── configmap.yaml
│   │   ├── secret.yaml
│   │   └── serviceaccount.yaml
│   └── charts/             # Dependency charts
└── README.md               # This file
```

### Installation (Future)

```bash
# Add Helm repository (when available)
helm repo add shelly-manager https://ginsys.github.io/shelly-manager

# Install with default values
helm install my-shelly-manager shelly-manager/shelly-manager

# Install with custom values
helm install my-shelly-manager shelly-manager/shelly-manager \
  -f values-prod.yaml \
  --set image.tag=20250825-143022-abc1234
```

### Key Features (Planned)

- **Multiple Environments**: Development, staging, production value files
- **Database Options**: SQLite, PostgreSQL, MySQL support
- **Ingress Options**: Multiple ingress controller support
- **Security**: RBAC, security contexts, network policies
- **Monitoring**: Prometheus ServiceMonitor integration
- **Scaling**: HorizontalPodAutoscaler support
- **Storage**: Persistent volume management
- **Configuration**: Flexible configuration via values

### Chart Dependencies (Planned)

- **PostgreSQL**: Optional database dependency
- **Ingress-NGINX**: Optional ingress controller
- **Cert-Manager**: Optional TLS certificate management

## Status

This Helm chart is planned for future development. Current priority is:

1. ✅ Docker Compose functionality
2. ⏳ Basic Kubernetes manifests
3. 📋 Helm chart development
4. 📋 Chart repository setup

## Contributing

Interested in contributing to the Helm chart development? Please:

1. Check existing Kubernetes manifests in `../kubernetes/base/`
2. Review Docker Compose configuration for reference
3. Follow Helm best practices for chart structure
4. Ensure compatibility with multiple environments

## Timeline

- **Phase 1**: Basic chart structure and templates
- **Phase 2**: Values customization and environments  
- **Phase 3**: Advanced features (HPA, monitoring, security)
- **Phase 4**: Chart repository and CI/CD integration