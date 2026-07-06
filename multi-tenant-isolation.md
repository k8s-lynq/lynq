---
url: 'https://lynq.sh/multi-tenant-isolation.md'
description: >-
  Isolate node resources using namespace-per-node, NetworkPolicy, and
  ResourceQuota. Complete LynqForm templates for multi-tenant Kubernetes
  clusters.
---

# Multi-Tenant Isolation

Each node gets its own namespace, network boundary, and resource quota — enforced by Lynq automatically from database rows. Add a customer; isolation follows.

::: tip Time to working
\~10 minutes to apply the complete isolation template. New nodes are isolated automatically from the first sync.
:::

## How It Works

::: v-pre

* **Namespace-per-node** — `targetNamespace: "{{ .uid }}"` creates a dedicated namespace for every node.
* **NetworkPolicy** — default-deny plus allow-internal rules confine traffic to the node's own namespace.
* **ResourceQuota** — CPU/memory/pod limits prevent any single node from exhausting cluster resources.
  :::

## Prerequisites

* A CNI that enforces NetworkPolicy (Calico, Cilium, Weave Net, or similar). Flannel does not enforce policies by default.
* Operator RBAC must include `namespaces` create/delete and `networkpolicies` all verbs.

## Isolation Models

### Namespace-per-Node (Recommended)

Each node gets its own namespace for complete isolation:

::: v-pre

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: isolated-node
spec:
  hubId: customer-hub
  namespaces:
    - id: node-ns
      nameTemplate: "{{ .uid }}"
      spec:
        labels:
          lynq.sh/node: "{{ .uid }}"
          environment: production
  deployments:
    - id: app
      targetNamespace: "{{ .uid }}"
      nameTemplate: "{{ .uid }}-app"
      dependIds: ["node-ns"]
      spec:
        # ... deployment spec
```

:::

### Shared Namespace with Labels

Multiple nodes share namespaces, isolated by label selectors. Simpler but provides weaker security boundaries:

::: v-pre

```yaml
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

:::

## Network Isolation with NetworkPolicy

### Default Deny All

Apply as the first policy in every node namespace:

::: v-pre

```yaml
networkPolicies:
  - id: default-deny
    nameTemplate: "default-deny"
    targetNamespace: "{{ .uid }}"
    dependIds: ["node-ns"]
    spec:
      podSelector: {}
      policyTypes:
        - Ingress
        - Egress
```

:::

### Allow Internal Communication

Pods within the same node namespace can communicate:

::: v-pre

```yaml
networkPolicies:
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
```

:::

### Allow DNS Resolution

Required for any pod that resolves service names:

::: v-pre

```yaml
networkPolicies:
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
              podSelector:
                matchLabels:
                  k8s-app: kube-dns
          ports:
            - protocol: UDP
              port: 53
            - protocol: TCP
              port: 53
```

:::

### Allow External Ingress

Permit traffic from your ingress controller:

::: v-pre

```yaml
networkPolicies:
  - id: allow-ingress
    nameTemplate: "allow-ingress"
    targetNamespace: "{{ .uid }}"
    dependIds: ["default-deny"]
    spec:
      podSelector:
        matchLabels:
          lynq.sh/node: "{{ .uid }}"
      policyTypes: [Ingress]
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

:::

## Resource Quotas per Node

Prevent resource exhaustion by scoping CPU, memory, and object counts per namespace:

::: v-pre

```yaml
manifests:
  - id: quota
    nameTemplate: "resource-quota"
    targetNamespace: "{{ .uid }}"
    dependIds: ["node-ns"]
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

:::

## Complete Isolation Template

Combine namespace, network policies, and quota into a single LynqForm:

::: v-pre

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: secure-node
spec:
  hubId: customer-hub

  namespaces:
    - id: node-ns
      nameTemplate: "{{ .uid }}"
      spec:
        labels:
          lynq.sh/node: "{{ .uid }}"

  networkPolicies:
    - id: default-deny
      nameTemplate: "default-deny"
      targetNamespace: "{{ .uid }}"
      dependIds: ["node-ns"]
      spec:
        podSelector: {}
        policyTypes: [Ingress, Egress]

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

  manifests:
    - id: quota
      nameTemplate: "resource-quota"
      targetNamespace: "{{ .uid }}"
      dependIds: ["node-ns"]
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

  deployments:
    - id: app
      nameTemplate: "app"
      targetNamespace: "{{ .uid }}"
      dependIds: ["allow-internal", "allow-dns", "quota"]
      spec:
        # ... deployment spec
```

:::

## Verify It Works

```bash
# List namespaces created by Lynq
kubectl get namespaces -l lynq.sh/node

# List NetworkPolicies for a specific node
kubectl get networkpolicies -n acme-corp

# Test cross-node traffic is blocked
kubectl run test --rm -it -n acme-corp --image=busybox -- \
  wget -T 5 http://app.other-node.svc.cluster.local
# Expected: wget: download timed out

# Test internal traffic succeeds
kubectl run test --rm -it -n acme-corp --image=busybox -- \
  wget -T 5 http://app.acme-corp.svc.cluster.local
# Expected: connected

# Verify resource quota
kubectl describe resourcequota node-quota -n acme-corp
```

## Caveats

* **CNI requirement**: NetworkPolicy enforcement requires a compatible CNI plugin. Flannel does not enforce policies without an additional plugin.
* **Namespace deletion lag**: When a node is deactivated, Lynq deletes the namespace. Kubernetes namespace deletion is asynchronous — the namespace enters `Terminating` state while child resources are cleaned up. This can take 30–60 seconds for namespaces with many resources.
* **ResourceQuota is not a hard wall**: CPU limits only apply when pods specify `resources.limits`. Pods without limits can still exhaust node CPU.
* **NetworkPolicy scope**: Policies only apply within the cluster. They do not control traffic to/from external services unless you add explicit egress rules for those CIDRs.

## See Also

* [Security Guide](security.md) — RBAC, credentials, and audit logging.
* [Dependencies](dependencies.md) — Order namespace creation before deploying workloads into it.
* [Policies](policies.md) — DeletionPolicy Retain for keeping namespaces after node deactivation.
* [Templates](templates.md) — Dynamic namespace naming and targetNamespace.
