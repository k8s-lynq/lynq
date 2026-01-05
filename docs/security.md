# Security Guide

Security best practices for Lynq.

[[toc]]

## Credentials Management

### Database Credentials

Always use Kubernetes Secrets for sensitive data:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mysql-credentials
  namespace: default
type: Opaque
stringData:
  password: your-secure-password
```

Reference in LynqHub:

```yaml
spec:
  source:
    mysql:
      passwordRef:
        name: mysql-credentials
        key: password
```

::: danger Credential safety
Never hardcode credentials in CRDs or templates. Always reference Kubernetes Secrets.
:::

### Rotating Credentials

1. Update Secret:
```bash
kubectl create secret generic mysql-credentials \
  --from-literal=password=new-password \
  --dry-run=client -o yaml | kubectl apply -f -
```

2. Operator automatically detects change and reconnects.

## RBAC

### Operator Permissions

The operator requires:

**CRD Management:**
- `lynqhubs`, `lynqforms`, `lynqnodes`: All verbs

**Resource Management:**
- Managed resources (Deployments, Services, etc.): All verbs in target namespaces
- `namespaces`: Create, list, watch, get (cluster-scoped)

**Supporting Resources:**
- `events`: Create, patch
- `leases`: Get, create, update (for leader election)
- `secrets`: Get, list, watch (for credentials, namespace-scoped)

### Least Privilege

Scope RBAC to specific namespaces when possible:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role  # Not ClusterRole
metadata:
  name: lynq-role
  namespace: production  # Specific namespace
rules:
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["*"]
```

### Scenario-Based RBAC Examples

**Scenario 1: Single Namespace (Basic)**

For operators managing resources in a single namespace:

```yaml
# Role for managing node resources in 'production' namespace only
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: lynq-production-role
  namespace: production
rules:
# Core resources
- apiGroups: [""]
  resources: ["services", "configmaps", "secrets", "serviceaccounts", "persistentvolumeclaims"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
# Workloads
- apiGroups: ["apps"]
  resources: ["deployments", "statefulsets", "daemonsets"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
# Networking
- apiGroups: ["networking.k8s.io"]
  resources: ["ingresses", "networkpolicies"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
# Batch
- apiGroups: ["batch"]
  resources: ["jobs", "cronjobs"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
# Events (for status reporting)
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: lynq-production-binding
  namespace: production
subjects:
- kind: ServiceAccount
  name: lynq-controller-manager
  namespace: lynq-system
roleRef:
  kind: Role
  name: lynq-production-role
  apiGroup: rbac.authorization.k8s.io
```

**Scenario 2: Multi-Namespace with Cross-Namespace Resources**

For operators managing resources across multiple namespaces:

```yaml
# ClusterRole for cross-namespace resource management
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: lynq-cluster-role
rules:
# Lynq CRDs (cluster-scoped)
- apiGroups: ["operator.lynq.sh"]
  resources: ["lynqhubs", "lynqforms", "lynqnodes"]
  verbs: ["*"]
- apiGroups: ["operator.lynq.sh"]
  resources: ["lynqhubs/status", "lynqforms/status", "lynqnodes/status"]
  verbs: ["get", "update", "patch"]
# Namespace management (for dynamic namespace creation)
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list", "watch", "create"]
# All managed resources (cluster-wide)
- apiGroups: [""]
  resources: ["services", "configmaps", "secrets", "serviceaccounts", "persistentvolumeclaims"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["apps"]
  resources: ["deployments", "statefulsets", "daemonsets"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["networking.k8s.io"]
  resources: ["ingresses", "networkpolicies"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["batch"]
  resources: ["jobs", "cronjobs"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["autoscaling"]
  resources: ["horizontalpodautoscalers"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["policy"]
  resources: ["poddisruptionbudgets"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
# Events
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
# Leader election
- apiGroups: ["coordination.k8s.io"]
  resources: ["leases"]
  verbs: ["get", "create", "update"]
```

**Scenario 3: Read-Only Access for Monitoring**

For users who only need to view Lynq resources:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: lynq-viewer
rules:
- apiGroups: ["operator.lynq.sh"]
  resources: ["lynqhubs", "lynqforms", "lynqnodes"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: lynq-viewer-binding
subjects:
- kind: User
  name: monitoring-user
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: lynq-viewer
  apiGroup: rbac.authorization.k8s.io
```

### RBAC Verification Commands

Verify permissions before deploying:

```bash
# Check if operator can create deployments in production namespace
kubectl auth can-i create deployments -n production \
  --as=system:serviceaccount:lynq-system:lynq-controller-manager
# Expected: yes

# Check if operator can create namespaces (cluster-scoped)
kubectl auth can-i create namespaces \
  --as=system:serviceaccount:lynq-system:lynq-controller-manager
# Expected: yes (if cross-namespace feature enabled)

# Check if operator can read secrets (for DB credentials)
kubectl auth can-i get secrets -n production \
  --as=system:serviceaccount:lynq-system:lynq-controller-manager
# Expected: yes

# List all permissions for the service account
kubectl auth can-i --list \
  --as=system:serviceaccount:lynq-system:lynq-controller-manager

# Check specific resource in specific namespace
kubectl auth can-i delete ingresses -n staging \
  --as=system:serviceaccount:lynq-system:lynq-controller-manager
```

**Troubleshooting RBAC Issues:**

```bash
# If operator logs show "forbidden" errors:
$ kubectl logs -n lynq-system deployment/lynq-controller-manager | grep forbidden

# Example error:
# "deployments.apps is forbidden: User ... cannot create resource ... in API group"

# Solution: Check and update Role/ClusterRole
kubectl get clusterrole lynq-cluster-role -o yaml | grep -A5 "deployments"

# Verify binding exists
kubectl get clusterrolebindings | grep lynq
kubectl describe clusterrolebinding lynq-cluster-binding
```

### Service Account

Default service account: `lynq-controller-manager`

Custom service account:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: custom-sa
  namespace: lynq-system
---
apiVersion: v1
kind: Pod
spec:
  serviceAccountName: custom-sa
```

## Multi-Tenancy Isolation

Lynq supports multiple isolation models for multi-tenant environments.

### Namespace Isolation Model

**Model 1: Namespace-per-Node (Recommended)**

Each node gets its own namespace for complete isolation:

```yaml
# In LynqForm: Create namespace per node
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: isolated-tenant
spec:
  hubId: customer-hub
  namespaces:
    - id: tenant-ns
      nameTemplate: "{{ .uid }}"  # Namespace named after node UID
      spec:
        labels:
          lynq.sh/node: "{{ .uid }}"
          environment: production
  deployments:
    - id: app
      targetNamespace: "{{ .uid }}"  # Deploy to node's namespace
      nameTemplate: "{{ .uid }}-app"
      dependIds: ["tenant-ns"]
      spec:
        # ... deployment spec
```

**Model 2: Shared Namespace with Labels**

Multiple nodes share namespaces, isolated by labels:

```yaml
# All nodes in 'production' namespace, isolated by labels
deployments:
  - id: app
    nameTemplate: "{{ .uid }}-app"
    labelsTemplate:
      lynq.sh/node: "{{ .uid }}"
      tenant: "{{ .uid }}"
    spec:
      template:
        metadata:
          labels:
            lynq.sh/node: "{{ .uid }}"
```

### Network Isolation with NetworkPolicy

**1. Default Deny All (Recommended Baseline)**

Apply to each node namespace:

```yaml
# Deny all ingress/egress by default
networkPolicies:
  - id: default-deny
    nameTemplate: "{{ .uid }}-default-deny"
    targetNamespace: "{{ .uid }}"
    dependIds: ["tenant-ns"]
    spec:
      podSelector: {}  # Applies to all pods in namespace
      policyTypes:
        - Ingress
        - Egress
```

**2. Allow Internal Communication Only**

Pods within the same node can communicate:

```yaml
networkPolicies:
  - id: allow-internal
    nameTemplate: "{{ .uid }}-allow-internal"
    targetNamespace: "{{ .uid }}"
    dependIds: ["default-deny"]
    spec:
      podSelector:
        matchLabels:
          lynq.sh/node: "{{ .uid }}"
      policyTypes:
        - Ingress
        - Egress
      ingress:
        - from:
            - podSelector:
                matchLabels:
                  lynq.sh/node: "{{ .uid }}"
      egress:
        - to:
            - podSelector:
                matchLabels:
                  lynq.sh/node: "{{ .uid }}"
```

**3. Allow External Ingress (via Ingress Controller)**

```yaml
networkPolicies:
  - id: allow-ingress
    nameTemplate: "{{ .uid }}-allow-ingress"
    targetNamespace: "{{ .uid }}"
    dependIds: ["default-deny"]
    spec:
      podSelector:
        matchLabels:
          lynq.sh/node: "{{ .uid }}"
      policyTypes:
        - Ingress
      ingress:
        - from:
            - namespaceSelector:
                matchLabels:
                  kubernetes.io/metadata.name: ingress-nginx
              podSelector:
                matchLabels:
                  app.kubernetes.io/name: ingress-nginx
          ports:
            - protocol: TCP
              port: 8080
```

**4. Allow DNS Resolution**

```yaml
networkPolicies:
  - id: allow-dns
    nameTemplate: "{{ .uid }}-allow-dns"
    targetNamespace: "{{ .uid }}"
    dependIds: ["default-deny"]
    spec:
      podSelector: {}
      policyTypes:
        - Egress
      egress:
        - to:
            - namespaceSelector:
                matchLabels:
                  kubernetes.io/metadata.name: kube-system
              podSelector:
                matchLabels:
                  k8s-app: kube-dns
          ports:
            - protocol: UDP
              port: 53
            - protocol: TCP
              port: 53
```

**5. Complete Isolation Template**

Combine all policies for a fully isolated node:

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: secure-tenant
spec:
  hubId: customer-hub
  # 1. Create isolated namespace
  namespaces:
    - id: tenant-ns
      nameTemplate: "{{ .uid }}"
      spec:
        labels:
          lynq.sh/node: "{{ .uid }}"

  # 2. Apply network policies
  networkPolicies:
    # Default deny
    - id: default-deny
      nameTemplate: "default-deny"
      targetNamespace: "{{ .uid }}"
      dependIds: ["tenant-ns"]
      spec:
        podSelector: {}
        policyTypes: [Ingress, Egress]

    # Allow DNS
    - id: allow-dns
      nameTemplate: "allow-dns"
      targetNamespace: "{{ .uid }}"
      dependIds: ["default-deny"]
      spec:
        podSelector: {}
        policyTypes: [Egress]
        egress:
          - to:
              - namespaceSelector:
                  matchLabels:
                    kubernetes.io/metadata.name: kube-system
            ports:
              - protocol: UDP
                port: 53

    # Allow internal
    - id: allow-internal
      nameTemplate: "allow-internal"
      targetNamespace: "{{ .uid }}"
      dependIds: ["default-deny"]
      spec:
        podSelector:
          matchLabels:
            lynq.sh/node: "{{ .uid }}"
        policyTypes: [Ingress, Egress]
        ingress:
          - from:
              - podSelector:
                  matchLabels:
                    lynq.sh/node: "{{ .uid }}"
        egress:
          - to:
              - podSelector:
                  matchLabels:
                    lynq.sh/node: "{{ .uid }}"

  # 3. Deploy workload
  deployments:
    - id: app
      nameTemplate: "app"
      targetNamespace: "{{ .uid }}"
      dependIds: ["allow-internal", "allow-dns"]
      spec:
        # ... deployment spec
```

### Verify Network Isolation

```bash
# List NetworkPolicies for a node
kubectl get networkpolicies -n acme-corp

# Test connectivity (should fail if properly isolated)
kubectl run test-pod -n acme-corp --rm -it --image=busybox -- wget -T 5 http://other-tenant.other-ns.svc.cluster.local
# Expected: wget: download timed out

# Test internal connectivity (should succeed)
kubectl run test-pod -n acme-corp --rm -it --image=busybox -- wget -T 5 http://app.acme-corp.svc.cluster.local
# Expected: Connected, downloading...
```

### Resource Quotas per Node

Prevent resource exhaustion attacks:

```yaml
# In LynqForm: Create ResourceQuota per node namespace
manifests:
  - id: quota
    nameTemplate: "resource-quota"
    targetNamespace: "{{ .uid }}"
    dependIds: ["tenant-ns"]
    spec:
      apiVersion: v1
      kind: ResourceQuota
      metadata:
        name: node-quota
      spec:
        hard:
          requests.cpu: "2"
          requests.memory: "4Gi"
          limits.cpu: "4"
          limits.memory: "8Gi"
          pods: "20"
          services: "10"
          persistentvolumeclaims: "5"
```

## Data Security

### Sensitive Data in Templates

Avoid storing sensitive data in database columns. Instead:

1. Store only references:
```sql
-- Good
api_key_ref = "secret-acme-api-key"

-- Bad
api_key = "sk-abc123..."
```

2. Reference Secrets in templates:
```yaml
env:
- name: API_KEY
  valueFrom:
    secretKeyRef:
      name: "{{ .uid }}-secrets"
      key: api-key
```

## Audit Logging

### Enable Audit Logs

Configure Kubernetes audit policy:

```yaml
# audit-policy.yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: RequestResponse
  resources:
  - group: "operator.lynq.sh"
    resources: ["lynqhubs", "lynqforms", "lynqnodes"]
```

### Track Changes

Monitor events:

```bash
kubectl get events --all-namespaces | grep LynqNode
```

## Compliance

### Data Retention

Configure deletion policies for compliance:

```yaml
persistentVolumeClaims:
  - id: data
    deletionPolicy: Retain  # Keep data after node deletion
```

### Immutable Resources

Use `CreationPolicy: Once` for audit resources:

```yaml
configMaps:
  - id: audit-log
    creationPolicy: Once  # Never update
```

## Vulnerability Management

### Container Scanning

Scan operator images:

```bash
# Using Trivy
trivy image ghcr.io/k8s-lynq/lynq:latest

# Using Snyk
snyk container test ghcr.io/k8s-lynq/lynq:latest
```

### Dependency Updates

Keep dependencies updated:

```bash
# Update Go dependencies
go get -u ./...
go mod tidy

# Check for vulnerabilities
go list -json -m all | nancy sleuth
```

## Best Practices

1. **Never hardcode credentials** - Use Secrets with SecretRef
2. **Enforce least privilege** - Scope RBAC to specific namespaces
3. **Apply security contexts** - Run as non-root, drop capabilities
4. **Enable audit logging** - Track all CRD changes
5. **Scan container images** - Regular vulnerability scanning
6. **Rotate credentials** - Regular password rotation
7. **Apply network policies** - Isolate node traffic
8. **Enforce resource quotas** - Prevent resource exhaustion

## See Also

- [Configuration Guide](configuration.md)
- [Installation Guide](installation.md)
- [RBAC Documentation](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
