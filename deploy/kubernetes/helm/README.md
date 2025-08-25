# Helm Chart Deployments

Helm provides package management for Kubernetes with advanced templating, dependency management, and lifecycle operations.

## 📁 Directory Structure

```
helm/
├── shelly-manager/          # Main Helm chart
│   ├── Chart.yaml          # Chart metadata and dependencies
│   ├── values.yaml         # Default configuration values
│   ├── values-dev.yaml     # Development environment values
│   ├── values-staging.yaml # Staging environment values
│   ├── values-prod.yaml    # Production environment values
│   ├── templates/          # Kubernetes manifest templates
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── ingress.yaml
│   │   ├── configmap.yaml
│   │   ├── secret.yaml
│   │   ├── serviceaccount.yaml
│   │   ├── pvc.yaml
│   │   ├── hpa.yaml
│   │   ├── servicemonitor.yaml
│   │   ├── _helpers.tpl    # Template helpers
│   │   └── NOTES.txt       # Post-install notes
│   └── charts/             # Chart dependencies
└── README.md               # This documentation
```

## 🚀 Quick Start

### Prerequisites
- **Helm CLI** (v3.8+): `curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash`
- **Kubernetes cluster** (v1.20+)
- **kubectl** configured and connected

### Install Development Environment
```bash
# Install with development values
helm install shelly-dev ./deploy/kubernetes/helm/shelly-manager/ \
  --values ./deploy/kubernetes/helm/shelly-manager/values-dev.yaml \
  --namespace shelly-dev \
  --create-namespace

# Check deployment status
helm status shelly-dev -n shelly-dev
kubectl get all -n shelly-dev
```

### Install Production Environment
```bash
# Install with production values
helm install shelly-prod ./deploy/kubernetes/helm/shelly-manager/ \
  --values ./deploy/kubernetes/helm/shelly-manager/values-prod.yaml \
  --namespace shelly-prod \
  --create-namespace

# Verify deployment
helm list -n shelly-prod
kubectl get ingress -n shelly-prod
```

## 📊 Chart Configuration

### Chart.yaml
```yaml
apiVersion: v2
name: shelly-manager
description: Smart home device manager for Shelly devices
type: application
version: 0.1.0
appVersion: "0.5.0-alpha"
home: https://github.com/ginsys/shelly-manager
sources:
- https://github.com/ginsys/shelly-manager
maintainers:
- name: Ginsys Team
  email: team@ginsys.com
keywords:
- smart-home
- iot
- shelly
- device-management
dependencies:
- name: postgresql
  version: "12.x.x"
  repository: "https://charts.bitnami.com/bitnami"
  condition: postgresql.enabled
  tags:
  - database
```

### Default Values (values.yaml)
```yaml
# Global configuration
global:
  imageRegistry: ghcr.io
  imagePullSecrets: []

# Application images
image:
  registry: ghcr.io
  repository: ginsys/shelly-manager
  tag: "latest"
  pullPolicy: IfNotPresent

provisioner:
  enabled: true
  image:
    registry: ghcr.io
    repository: ginsys/shelly-provisioner
    tag: "latest"
    pullPolicy: IfNotPresent

# Deployment configuration
replicaCount: 1
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 1
    maxUnavailable: 0

# Service configuration
service:
  type: ClusterIP
  port: 80
  targetPort: 8080
  metricsPort: 9090

# Ingress configuration
ingress:
  enabled: false
  className: "nginx"
  annotations: {}
  hosts:
  - host: shelly-manager.local
    paths:
    - path: /
      pathType: Prefix
  tls: []

# Resource configuration
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "256Mi"
    cpu: "250m"

# Autoscaling
autoscaling:
  enabled: false
  minReplicas: 1
  maxReplicas: 10
  targetCPUUtilizationPercentage: 80

# Database configuration
database:
  provider: sqlite  # sqlite, postgresql, mysql
  path: /data/shelly.db
  
# PostgreSQL dependency
postgresql:
  enabled: false
  auth:
    username: shelly
    database: shelly

# Application configuration
config:
  server:
    port: 8080
    host: "0.0.0.0"
  logging:
    level: info
    format: json
    output: stdout
  discovery:
    enabled: true
    networks: ["192.168.1.0/24"]
    interval: 300
  provisioning:
    auto: false
    interval: 600

# Security configuration
securityContext:
  runAsNonRoot: true
  runAsUser: 10001
  fsGroup: 10001

# Persistence
persistence:
  enabled: true
  storageClass: ""
  accessMode: ReadWriteOnce
  size: 1Gi

# Monitoring
serviceMonitor:
  enabled: false
  namespace: monitoring
  interval: 30s

# Pod configuration
podSecurityContext:
  fsGroup: 10001

nodeSelector: {}
tolerations: []
affinity: {}
```

## 🎯 Environment-Specific Values

### Development (values-dev.yaml)
```yaml
# Development overrides
replicaCount: 1

image:
  tag: "latest"
  pullPolicy: Always

config:
  logging:
    level: debug
    format: text
  server:
    ginMode: debug

resources:
  requests:
    memory: "64Mi"
    cpu: "50m"
  limits:
    memory: "128Mi"
    cpu: "100m"

ingress:
  enabled: true
  hosts:
  - host: shelly-dev.local
    paths:
    - path: /
      pathType: Prefix

persistence:
  enabled: false  # Use emptyDir for development

database:
  provider: sqlite
  path: /tmp/shelly.db
```

### Staging (values-staging.yaml)
```yaml
# Staging overrides  
replicaCount: 2

image:
  tag: "staging-latest"

config:
  logging:
    level: info
    format: json

resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "256Mi"
    cpu: "250m"

ingress:
  enabled: true
  hosts:
  - host: shelly-staging.example.com
    paths:
    - path: /
      pathType: Prefix
  tls:
  - secretName: shelly-staging-tls
    hosts:
    - shelly-staging.example.com

# Enable PostgreSQL for staging
postgresql:
  enabled: true
  auth:
    username: shelly
    database: shelly
    existingSecret: postgresql-secret

database:
  provider: postgresql
  dsn: postgresql://shelly:$(POSTGRES_PASSWORD)@shelly-manager-postgresql:5432/shelly
```

### Production (values-prod.yaml)
```yaml
# Production overrides
replicaCount: 3

image:
  tag: "20250825-143022-abc1234"  # Specific version
  pullPolicy: IfNotPresent

strategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 1
    maxUnavailable: 0

config:
  logging:
    level: info
    format: json
  server:
    ginMode: release

resources:
  requests:
    memory: "256Mi"
    cpu: "250m"
  limits:
    memory: "512Mi"
    cpu: "500m"

# Enable autoscaling
autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70

# Production ingress with TLS
ingress:
  enabled: true
  className: "nginx"
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    nginx.ingress.kubernetes.io/rate-limit: "100"
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
  hosts:
  - host: shelly.example.com
    paths:
    - path: /
      pathType: Prefix
  tls:
  - secretName: shelly-prod-tls
    hosts:
    - shelly.example.com

# Production database
postgresql:
  enabled: true
  auth:
    username: shelly
    database: shelly
    existingSecret: postgresql-secret
  primary:
    persistence:
      enabled: true
      size: 20Gi
  resources:
    requests:
      memory: "256Mi"
      cpu: "100m"
    limits:
      memory: "512Mi"  
      cpu: "500m"

database:
  provider: postgresql
  dsn: postgresql://shelly:$(POSTGRES_PASSWORD)@shelly-manager-postgresql:5432/shelly

# Enable monitoring
serviceMonitor:
  enabled: true
  namespace: monitoring

# Production persistence
persistence:
  enabled: true
  storageClass: "fast-ssd"
  size: 10Gi

# Node affinity for production
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchExpressions:
          - key: app.kubernetes.io/name
            operator: In
            values:
            - shelly-manager
        topologyKey: kubernetes.io/hostname
```

## 🔧 Template Examples

### Deployment Template (templates/deployment.yaml)
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "shelly-manager.fullname" . }}
  labels:
    {{- include "shelly-manager.labels" . | nindent 4 }}
spec:
  {{- if not .Values.autoscaling.enabled }}
  replicas: {{ .Values.replicaCount }}
  {{- end }}
  strategy:
    {{- toYaml .Values.strategy | nindent 4 }}
  selector:
    matchLabels:
      {{- include "shelly-manager.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      annotations:
        checksum/config: {{ include (print $.Template.BasePath "/configmap.yaml") . | sha256sum }}
      labels:
        {{- include "shelly-manager.selectorLabels" . | nindent 8 }}
    spec:
      {{- with .Values.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      serviceAccountName: {{ include "shelly-manager.serviceAccountName" . }}
      securityContext:
        {{- toYaml .Values.podSecurityContext | nindent 8 }}
      containers:
      - name: {{ .Chart.Name }}
        image: "{{ .Values.image.registry }}/{{ .Values.image.repository }}:{{ .Values.image.tag }}"
        imagePullPolicy: {{ .Values.image.pullPolicy }}
        ports:
        - name: http
          containerPort: {{ .Values.config.server.port }}
          protocol: TCP
        - name: metrics  
          containerPort: 9090
          protocol: TCP
        envFrom:
        - configMapRef:
            name: {{ include "shelly-manager.fullname" . }}-config
        {{- if .Values.database.existingSecret }}
        - secretRef:
            name: {{ .Values.database.existingSecret }}
        {{- end }}
        livenessProbe:
          httpGet:
            path: /api/v1/health
            port: http
          initialDelaySeconds: 30
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /api/v1/ready  
            port: http
          initialDelaySeconds: 10
          periodSeconds: 10
        resources:
          {{- toYaml .Values.resources | nindent 12 }}
        volumeMounts:
        {{- if .Values.persistence.enabled }}
        - name: data
          mountPath: /data
        {{- end }}
      volumes:
      {{- if .Values.persistence.enabled }}
      - name: data
        persistentVolumeClaim:
          claimName: {{ include "shelly-manager.fullname" . }}-data
      {{- else }}
      - name: data
        emptyDir: {}
      {{- end }}
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.affinity }}
      affinity:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.tolerations }}
      tolerations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
```

## 🚀 Helm Operations

### Installation
```bash
# Install with default values
helm install my-shelly ./deploy/kubernetes/helm/shelly-manager/

# Install with custom values file
helm install my-shelly ./deploy/kubernetes/helm/shelly-manager/ \
  --values ./deploy/kubernetes/helm/shelly-manager/values-prod.yaml

# Install with inline value overrides
helm install my-shelly ./deploy/kubernetes/helm/shelly-manager/ \
  --set replicaCount=3 \
  --set ingress.enabled=true \
  --set database.provider=postgresql
```

### Upgrades
```bash
# Upgrade with new image tag
helm upgrade my-shelly ./deploy/kubernetes/helm/shelly-manager/ \
  --set image.tag=20250825-143022-abc1234 \
  --reuse-values

# Upgrade with new values file
helm upgrade my-shelly ./deploy/kubernetes/helm/shelly-manager/ \
  --values ./deploy/kubernetes/helm/shelly-manager/values-prod.yaml
```

### Rollbacks
```bash
# View release history
helm history my-shelly

# Rollback to previous version
helm rollback my-shelly

# Rollback to specific revision
helm rollback my-shelly 3
```

### Management
```bash
# List releases
helm list --all-namespaces

# Get release status
helm status my-shelly

# Get release values
helm get values my-shelly

# Uninstall release
helm uninstall my-shelly
```

## 🔒 Security Best Practices

### RBAC Configuration
```yaml
# templates/serviceaccount.yaml
{{- if .Values.serviceAccount.create -}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "shelly-manager.serviceAccountName" . }}
  labels:
    {{- include "shelly-manager.labels" . | nindent 4 }}
  {{- with .Values.serviceAccount.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
{{- end }}
```

### Network Policies
```yaml
# templates/networkpolicy.yaml
{{- if .Values.networkPolicy.enabled }}
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ include "shelly-manager.fullname" . }}
  labels:
    {{- include "shelly-manager.labels" . | nindent 4 }}
spec:
  podSelector:
    matchLabels:
      {{- include "shelly-manager.selectorLabels" . | nindent 6 }}
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: {{ .Values.networkPolicy.ingressNamespace }}
    ports:
    - protocol: TCP
      port: {{ .Values.config.server.port }}
{{- end }}
```

## 📊 Monitoring Integration

### ServiceMonitor Template
```yaml
{{- if and .Values.serviceMonitor.enabled }}
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: {{ include "shelly-manager.fullname" . }}
  namespace: {{ .Values.serviceMonitor.namespace | default .Release.Namespace }}
  labels:
    {{- include "shelly-manager.labels" . | nindent 4 }}
spec:
  selector:
    matchLabels:
      {{- include "shelly-manager.selectorLabels" . | nindent 6 }}
  endpoints:
  - port: metrics
    interval: {{ .Values.serviceMonitor.interval }}
    path: /metrics
{{- end }}
```

## 🧪 Testing

### Chart Testing
```bash
# Lint chart
helm lint ./deploy/kubernetes/helm/shelly-manager/

# Template generation test
helm template test-release ./deploy/kubernetes/helm/shelly-manager/ \
  --values ./deploy/kubernetes/helm/shelly-manager/values-dev.yaml

# Install in test mode
helm install test-release ./deploy/kubernetes/helm/shelly-manager/ \
  --dry-run --debug

# Test with different values
helm install test-release ./deploy/kubernetes/helm/shelly-manager/ \
  --values ./deploy/kubernetes/helm/shelly-manager/values-prod.yaml \
  --dry-run
```

### Automated Testing
```yaml
# .github/workflows/helm-test.yml
name: Helm Chart Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v5
    - uses: azure/setup-helm@v3
    - name: Lint chart
      run: helm lint deploy/kubernetes/helm/shelly-manager/
    - name: Test template generation
      run: |
        helm template test deploy/kubernetes/helm/shelly-manager/ \
          --values deploy/kubernetes/helm/shelly-manager/values-dev.yaml
```

## 📦 Chart Dependencies

### PostgreSQL Integration
```bash
# Update dependencies
helm dependency update ./deploy/kubernetes/helm/shelly-manager/

# Install with PostgreSQL
helm install my-shelly ./deploy/kubernetes/helm/shelly-manager/ \
  --set postgresql.enabled=true \
  --set database.provider=postgresql
```

## 🛠️ Troubleshooting

### Common Issues

1. **Template Rendering Errors**
   ```bash
   # Debug template rendering
   helm template debug-release ./deploy/kubernetes/helm/shelly-manager/ \
     --debug --values ./values-debug.yaml
   ```

2. **Dependency Issues**
   ```bash
   # Update dependencies
   helm dependency update ./deploy/kubernetes/helm/shelly-manager/
   
   # Check dependency status
   helm dependency list ./deploy/kubernetes/helm/shelly-manager/
   ```

3. **Installation Failures**
   ```bash
   # Get installation logs
   helm status my-shelly --show-desc
   
   # Check Kubernetes events
   kubectl get events --sort-by=.metadata.creationTimestamp
   ```

## 📚 Best Practices

1. **Versioning**: Use semantic versioning for chart versions
2. **Values**: Provide sensible defaults and document all options
3. **Templates**: Use helper templates for common patterns
4. **Testing**: Test charts with multiple value configurations
5. **Documentation**: Document all configuration options
6. **Security**: Follow Kubernetes security best practices
7. **Dependencies**: Pin dependency versions for stability

## 🔗 Related Documentation

- [Helm Official Documentation](https://helm.sh/docs/)
- [Chart Best Practices](https://helm.sh/docs/chart_best_practices/)
- [Kustomize Alternative](../kustomize/README.md)
- [Base Kubernetes Documentation](../README.md)