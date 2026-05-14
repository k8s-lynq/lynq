---
description: "Security guide for Lynq: credential management, RBAC, audit logging, and multi-tenant isolation."
---

# Security Guide

## Security Checklist

Before going to production:

- [ ] Database password stored in a Kubernetes Secret, referenced via `passwordRef`
- [ ] Operator ClusterRole scoped to only the API groups your nodes need
- [ ] `secrets` verb limited to the namespaces where node Secrets are created
- [ ] Kubernetes audit policy captures changes to `operator.lynq.sh` resources
- [ ] Container images scanned for vulnerabilities
- [ ] Node isolation strategy chosen (namespace-per-node recommended)
- [ ] NetworkPolicies applied if nodes should not communicate with each other

## Credentials Management

Always use Kubernetes Secrets for sensitive data. Never hardcode credentials in CRDs or templates.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mysql-credentials
  namespace: lynq-system
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

**Rotating credentials:** Update the Secret — the operator detects the change and reconnects automatically. No pod restart required.

**Sensitive data in templates:** Store only references in database columns, not actual secrets.

```sql
-- Store a reference, not the value
api_key_ref = "secret-acme-api-key"
```

```yaml
# Resolve the reference in templates
env:
  - name: API_KEY
    valueFrom:
      secretKeyRef:
        name: "{{ .uid }}-secrets"
        key: api-key
```

## RBAC

### Operator Permissions

The operator service account (`lynq-controller-manager`) requires:

| Resource | Verbs | Scope |
|----------|-------|-------|
| `lynqhubs`, `lynqforms`, `lynqnodes` | `*` | Cluster |
| `lynqhubs/status`, `lynqforms/status`, `lynqnodes/status` | `get`, `update`, `patch` | Cluster |
| `namespaces` | `get`, `list`, `watch`, `create` | Cluster |
| Managed resources (Deployments, Services, etc.) | `*` | Target namespaces |
| `events` | `create`, `patch` | All namespaces |
| `leases` | `get`, `create`, `update` | `lynq-system` |
| `secrets` | `*` | Namespaces with node credentials |

### Minimal Single-Namespace Role

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: lynq-role
  namespace: production
rules:
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
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: lynq-binding
  namespace: production
subjects:
- kind: ServiceAccount
  name: lynq-controller-manager
  namespace: lynq-system
roleRef:
  kind: Role
  name: lynq-role
  apiGroup: rbac.authorization.k8s.io
```

### Read-Only Viewer Role

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: lynq-viewer
rules:
- apiGroups: ["operator.lynq.sh"]
  resources: ["lynqhubs", "lynqforms", "lynqnodes"]
  verbs: ["get", "list", "watch"]
```

### Verify Permissions

```bash
# Check a specific permission
kubectl auth can-i create deployments -n production \
  --as=system:serviceaccount:lynq-system:lynq-controller-manager

# List all permissions
kubectl auth can-i --list \
  --as=system:serviceaccount:lynq-system:lynq-controller-manager

# Debug "forbidden" errors in logs
kubectl logs -n lynq-system deployment/lynq-controller-manager | grep forbidden
```

## Audit Logging

Configure Kubernetes audit policy to capture all changes to Lynq CRDs:

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

Monitor events for lifecycle changes:

```bash
kubectl get events --all-namespaces --field-selector involvedObject.kind=LynqNode
```

## Multi-Tenant Isolation

To isolate node resources using namespace-per-node, NetworkPolicy, and ResourceQuota, see [Multi-Tenant Isolation](multi-tenant-isolation.md).

Key compliance patterns using Lynq policies:

```yaml
# Retain PVC data after node deletion (compliance/audit)
persistentVolumeClaims:
  - id: data
    deletionPolicy: Retain

# Immutable audit log ConfigMap
configMaps:
  - id: audit-log
    creationPolicy: Once
```

## Vulnerability Management

```bash
# Scan operator image
trivy image ghcr.io/k8s-lynq/lynq:latest

# Check Go dependency vulnerabilities
go list -json -m all | nancy sleuth
```

## See Also

- [Multi-Tenant Isolation](multi-tenant-isolation.md) — namespace isolation, NetworkPolicy, ResourceQuota.
- [Configuration](configuration.md) — controller flags and resource limits.
- [Installation](installation.md) — initial RBAC setup via Helm or Kustomize.
