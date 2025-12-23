# Dashboard Installation Guide

Lynq Dashboard is a web UI for visualizing and monitoring Lynq Operator status. You can view the hierarchical structure of Hub, Form, and Node in a topology view, and quickly find problematic resources.

[[toc]]

## Choose Installation Method

| Method | Use Case | Difficulty |
|--------|----------|------------|
| [Docker (Local)](#docker-local) | Quick local testing | Easy |
| [Kubernetes Deployment](#kubernetes-deployment) | Running in cluster | Medium |
| [Helm Chart](#helm-chart) | Production deployment | Medium |

## Docker (Local)

Run the Dashboard on your local machine and connect to a cluster using your existing kubeconfig.

### Basic Usage

```bash
docker run -d \
  --name lynq-dashboard \
  -p 8080:8080 \
  -v ~/.kube/config:/root/.kube/config:ro \
  -e APP_MODE=local \
  ghcr.io/k8s-lynq/lynq-dashboard:latest
```

Open `http://localhost:8080` in your browser

### Using a Specific Context

To use a specific context from a kubeconfig with multiple clusters:

```bash
docker run -d \
  --name lynq-dashboard \
  -p 8080:8080 \
  -v ~/.kube/config:/root/.kube/config:ro \
  -e APP_MODE=local \
  ghcr.io/k8s-lynq/lynq-dashboard:latest \
  /app/bff-server -mode local -context my-cluster-context
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_MODE` | `cluster` | Run mode. `local` or `cluster` |
| `STATIC_DIR` | `/app/public` | Path to UI static files |

### Stop and Cleanup

```bash
docker stop lynq-dashboard
docker rm lynq-dashboard
```

## Kubernetes Deployment

Run the Dashboard as a Deployment within the cluster. It uses in-cluster authentication, so no separate kubeconfig is needed.

### 1. Create Namespace (Optional)

If Lynq Operator is already installed, the `lynq-system` namespace exists:

```bash
kubectl create namespace lynq-system --dry-run=client -o yaml | kubectl apply -f -
```

### 2. RBAC Setup

Grant permissions for the Dashboard to read Lynq CRDs and related resources:

```yaml
# dashboard-rbac.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: lynq-dashboard
  namespace: lynq-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: lynq-dashboard
rules:
  # Lynq CRDs
  - apiGroups: ["lynq.sh"]
    resources: ["lynqhubs", "lynqforms", "lynqnodes"]
    verbs: ["get", "list", "watch"]
  # Managed resources (read-only)
  - apiGroups: [""]
    resources: ["services", "configmaps", "secrets", "persistentvolumeclaims", "serviceaccounts", "events", "namespaces"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["deployments", "statefulsets", "daemonsets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["batch"]
    resources: ["jobs", "cronjobs"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["networking.k8s.io"]
    resources: ["ingresses"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["policy"]
    resources: ["poddisruptionbudgets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["autoscaling"]
    resources: ["horizontalpodautoscalers"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: lynq-dashboard
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: lynq-dashboard
subjects:
  - kind: ServiceAccount
    name: lynq-dashboard
    namespace: lynq-system
```

```bash
kubectl apply -f dashboard-rbac.yaml
```

### 3. Deployment and Service

```yaml
# dashboard-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lynq-dashboard
  namespace: lynq-system
  labels:
    app.kubernetes.io/name: lynq-dashboard
    app.kubernetes.io/component: dashboard
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: lynq-dashboard
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lynq-dashboard
    spec:
      serviceAccountName: lynq-dashboard
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
        runAsGroup: 65532
        fsGroup: 65532
      containers:
        - name: dashboard
          image: ghcr.io/k8s-lynq/lynq-dashboard:latest
          args:
            - "-mode"
            - "cluster"
            - "-addr"
            - ":8080"
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /healthz
              port: http
            initialDelaySeconds: 10
            periodSeconds: 30
          readinessProbe:
            httpGet:
              path: /readyz
              port: http
            initialDelaySeconds: 5
            periodSeconds: 10
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 256Mi
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop:
                - ALL
---
apiVersion: v1
kind: Service
metadata:
  name: lynq-dashboard
  namespace: lynq-system
  labels:
    app.kubernetes.io/name: lynq-dashboard
spec:
  type: ClusterIP
  ports:
    - port: 80
      targetPort: http
      protocol: TCP
      name: http
  selector:
    app.kubernetes.io/name: lynq-dashboard
```

```bash
kubectl apply -f dashboard-deployment.yaml
```

### 4. Access Methods

#### Port Forward (Development/Testing)

```bash
kubectl port-forward -n lynq-system svc/lynq-dashboard 8080:80
# Open http://localhost:8080 in browser
```

#### Ingress (Production)

```yaml
# dashboard-ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: lynq-dashboard
  namespace: lynq-system
  annotations:
    # Modify according to your Ingress Controller
    kubernetes.io/ingress.class: nginx
spec:
  rules:
    - host: lynq.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: lynq-dashboard
                port:
                  number: 80
  tls:
    - hosts:
        - lynq.example.com
      secretName: lynq-dashboard-tls
```

```bash
kubectl apply -f dashboard-ingress.yaml
```

## Helm Chart

The Dashboard is included in the Lynq Helm Chart.

### Installation

```bash
# Add Helm repo
helm repo add lynq https://k8s-lynq.github.io/lynq
helm repo update

# Install with Dashboard
helm install lynq lynq/lynq \
  --namespace lynq-system \
  --create-namespace \
  --set dashboard.enabled=true
```

### Key Configuration Options

```yaml
# values.yaml
dashboard:
  enabled: true
  replicaCount: 1
  image:
    repository: ghcr.io/k8s-lynq/lynq-dashboard
    tag: latest
    pullPolicy: IfNotPresent
  resources:
    requests:
      cpu: 50m
      memory: 64Mi
    limits:
      cpu: 200m
      memory: 256Mi
  service:
    type: ClusterIP
    port: 80
  ingress:
    enabled: false
    className: nginx
    hosts:
      - host: lynq.example.com
        paths:
          - path: /
            pathType: Prefix
    tls: []
```

### Enable Ingress

```bash
helm upgrade lynq lynq/lynq \
  --namespace lynq-system \
  --set dashboard.enabled=true \
  --set dashboard.ingress.enabled=true \
  --set dashboard.ingress.hosts[0].host=lynq.example.com
```

## Feature Customization

### Hide External Links

To hide the Documentation and GitHub links at the bottom of the Dashboard:

```yaml
# Add environment variables to Deployment
env:
  - name: VITE_HIDE_DOCS_LINK
    value: "true"
  - name: VITE_HIDE_GITHUB_LINK
    value: "true"
```

### Custom Links

Change external links to custom URLs:

```yaml
env:
  - name: VITE_DOCS_URL
    value: "https://internal-docs.company.com/lynq"
  - name: VITE_GITHUB_URL
    value: "https://github.internal.company.com/platform/lynq"
```

## Troubleshooting

### CRDs Not Found

If the Dashboard shows "No resources found":

1. Verify Lynq Operator CRDs are installed:
```bash
kubectl get crd | grep lynq
```

2. Verify RBAC permissions are correct:
```bash
kubectl auth can-i list lynqhubs --as=system:serviceaccount:lynq-system:lynq-dashboard
```

### Permission Errors

```bash
# Check if ServiceAccount has correct permissions
kubectl describe clusterrolebinding lynq-dashboard
```

### Connection Errors

If Docker local mode cannot connect to the cluster:

1. Verify the kubeconfig file is mounted correctly
2. Verify the current context points to an accessible cluster:
```bash
kubectl config current-context
kubectl cluster-info
```

## Next Steps

- [Quick Start](/quickstart) - Getting started with Lynq Operator
- [Architecture](/architecture) - Understanding the architecture
- [Monitoring](/monitoring) - Prometheus/Grafana integration
