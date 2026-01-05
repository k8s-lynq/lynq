# ExternalDNS Integration Guide

This guide shows how to integrate Lynq with ExternalDNS for automatic DNS record management.

[[toc]]

## Overview

**ExternalDNS** synchronizes exposed Kubernetes Services and Ingresses with DNS providers like AWS Route53, Google Cloud DNS, Cloudflare, and more. When integrated with Lynq, each node's DNS records are automatically created and deleted as nodes are provisioned.

```mermaid
flowchart LR
    Node["LynqNode CR<br/>(Ingress/Service templates)"]
    Resource["Kubernetes Ingress/Service"]
    ExternalDNS["ExternalDNS Controller"]
    Provider["DNS Provider<br/>(Route53, Cloudflare, ...)"]
    DNS["DNS Records<br/>(node.example.com)"]

    Node --> Resource --> ExternalDNS --> Provider --> DNS

    classDef node fill:#e3f2fd,stroke:#64b5f6,stroke-width:2px;
    class Node node;
    classDef external fill:#f3e5f5,stroke:#ba68c8,stroke-width:2px;
    class ExternalDNS external;
    classDef provider fill:#fff8e1,stroke:#ffca28,stroke-width:2px;
    class Provider,DNS provider;
```

### Use Cases

- **Multi-node SaaS**: Automatic subdomain creation per node (e.g., `node-a.example.com`, `node-b.example.com`)
- **Dynamic environments**: DNS records follow node lifecycle (created/deleted with node)
- **Multiple domains**: Different nodes on different domains or subdomains
- **SSL/TLS automation**: Combined with cert-manager for automatic certificate provisioning

## Prerequisites

::: info Requirements
- Kubernetes cluster v1.11+
- Lynq installed and reconciling
- DNS provider account (AWS Route53, Cloudflare, etc.)
- DNS zone created in your provider
:::

## Installation

### 1. Install ExternalDNS

#### Using Helm (Recommended)

```bash
# Add bitnami repo
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

# Install ExternalDNS for AWS Route53
helm install external-dns bitnami/external-dns \
  --namespace kube-system \
  --set provider=aws \
  --set aws.zoneType=public \
  --set domainFilters[0]=example.com \
  --set policy=upsert-only \
  --set txtOwnerId=my-cluster-id

# Or for Cloudflare
helm install external-dns bitnami/external-dns \
  --namespace kube-system \
  --set provider=cloudflare \
  --set cloudflare.apiToken=<your-api-token> \
  --set domainFilters[0]=example.com
```

#### Using Manifests

For detailed YAML manifests and provider-specific configurations, see:
- [ExternalDNS AWS Setup](https://github.com/kubernetes-sigs/external-dns/blob/master/docs/tutorials/aws.md)
- [ExternalDNS Cloudflare Setup](https://github.com/kubernetes-sigs/external-dns/blob/master/docs/tutorials/cloudflare.md)
- [ExternalDNS GCP Setup](https://github.com/kubernetes-sigs/external-dns/blob/master/docs/tutorials/gcp.md)

### 2. Verify Installation

```bash
# Check ExternalDNS pod
kubectl get pods -n kube-system -l app.kubernetes.io/name=external-dns

# Check logs
kubectl logs -n kube-system -l app.kubernetes.io/name=external-dns
```

## Integration with Lynq

### Basic Example: Ingress with Automatic DNS

**LynqForm with ExternalDNS annotations:**

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: web-app-with-dns
  namespace: default
spec:
  hubId: my-hub

  # Deployment
  deployments:
  - id: app
    nameTemplate: "{{ .uid }}-app"
    spec:
      apiVersion: apps/v1
      kind: Deployment
      spec:
        replicas: 2
        selector:
          matchLabels:
            app: "{{ .uid }}"
        template:
          metadata:
            labels:
              app: "{{ .uid }}"
          spec:
            containers:
            - name: app
              image: nginx:alpine
              ports:
              - containerPort: 80

  # Service
  services:
  - id: app-service
    nameTemplate: "{{ .uid }}-svc"
    spec:
      apiVersion: v1
      kind: Service
      spec:
        selector:
          app: "{{ .uid }}"
        ports:
        - port: 80
          targetPort: 80

  # Ingress with ExternalDNS annotation
  ingresses:
  - id: web-ingress
    nameTemplate: "{{ .uid }}-ingress"
    annotationsTemplate:
      external-dns.alpha.kubernetes.io/hostname: "{{ .host }}"
      external-dns.alpha.kubernetes.io/ttl: "300"
    spec:
      apiVersion: networking.k8s.io/v1
      kind: Ingress
      spec:
        ingressClassName: nginx
        rules:
        - host: "{{ .host }}"
          http:
            paths:
            - path: /
              pathType: Prefix
              backend:
                service:
                  name: "{{ .uid }}-svc"
                  port:
                    number: 80
```

**What happens:**
1. Lynq creates Ingress for each node (e.g., `acme-corp-ingress`)
2. ExternalDNS detects Ingress with `external-dns.alpha.kubernetes.io/hostname` annotation
3. ExternalDNS creates DNS A/AAAA record pointing to Ingress LoadBalancer IP
4. When node is deleted, DNS record is automatically removed

**Result:** Each node gets automatic DNS:
- `acme-corp.example.com` → 1.2.3.4
- `beta-inc.example.com` → 1.2.3.4

### LoadBalancer Service Example

For LoadBalancer Services (instead of Ingress):

```yaml
services:
- id: lb-service
  nameTemplate: "{{ .uid }}-lb"
  annotationsTemplate:
    external-dns.alpha.kubernetes.io/hostname: "{{ .host }}"
    external-dns.alpha.kubernetes.io/ttl: "300"
  spec:
    apiVersion: v1
    kind: Service
    type: LoadBalancer
    spec:
      selector:
        app: "{{ .uid }}"
      ports:
      - port: 80
        targetPort: 80
```

## How It Works

### Workflow

1. **Node Created**: LynqHub creates LynqNode CR from database
2. **Resources Applied**: LynqNode controller creates Ingress/Service with ExternalDNS annotations
3. **IP Assignment**: Kubernetes assigns LoadBalancer IP or Ingress IP
4. **DNS Sync**: ExternalDNS detects annotated resource and creates DNS record
5. **Propagation**: DNS record propagates through provider (seconds to minutes)
6. **Node Deleted**: LynqNode resources deleted → ExternalDNS removes DNS record

### DNS Record Lifecycle

```mermaid
sequenceDiagram
    participant TO as Lynq
    participant K8s as Kubernetes API
    participant ED as ExternalDNS
    participant DNS as DNS Provider

    TO->>K8s: Create Ingress with annotations
    K8s->>K8s: Assign LoadBalancer IP
    ED->>K8s: Watch Ingresses
    ED->>ED: Detect external-dns annotation
    ED->>DNS: Create DNS A record
    DNS-->>ED: Record created

    Note over TO,DNS: Node Active

    TO->>K8s: Delete Ingress
    ED->>K8s: Detect deletion
    ED->>DNS: Delete DNS record
    DNS-->>ED: Record deleted
```

## Common Annotations

### Required

| Annotation | Description | Example |
|------------|-------------|---------|
| `external-dns.alpha.kubernetes.io/hostname` | DNS hostname to create | `node.example.com` |

### Optional

| Annotation | Description | Default | Example |
|------------|-------------|---------|---------|
| `external-dns.alpha.kubernetes.io/ttl` | DNS TTL in seconds | `300` | `600` |
| `external-dns.alpha.kubernetes.io/target` | Override target IP/CNAME | Auto-detected | `1.2.3.4` |
| `external-dns.alpha.kubernetes.io/alias` | Use DNS alias (AWS Route53) | `false` | `true` |

### Provider-Specific

**AWS Route53:**
```yaml
annotations:
  external-dns.alpha.kubernetes.io/aws-weight: "100"
  external-dns.alpha.kubernetes.io/set-identifier: "primary"
```

**Cloudflare:**
```yaml
annotations:
  external-dns.alpha.kubernetes.io/cloudflare-proxied: "true"
```

## Multi-Domain Example

Support different domains per node using template variables:

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: multi-domain-template
spec:
  hubId: my-hub

  ingresses:
  - id: node-ingress
    nameTemplate: "{{ .uid }}-ingress"
    annotationsTemplate:
      # Use .host which is auto-extracted from .hostOrUrl
      external-dns.alpha.kubernetes.io/hostname: "{{ .host }}"
    spec:
      apiVersion: networking.k8s.io/v1
      kind: Ingress
      spec:
        ingressClassName: nginx
        rules:
        - host: "{{ .host }}"
          http:
            paths:
            - path: /
              pathType: Prefix
              backend:
                service:
                  name: "{{ .uid }}-svc"
                  port:
                    number: 80
```

**Database rows:**
```sql
node_id      node_url                       is_active
---------    --------------------------     ---------
acme-corp    https://acme.example.com       1
beta-inc     https://beta.example.io        1
gamma-co     https://custom.domain.net      1
```

**Result:**
- `acme.example.com` → acme-corp node
- `beta.example.io` → beta-inc node
- `custom.domain.net` → gamma-co node

## Verification Commands

After deploying, verify the integration works correctly:

```bash
# 1. Check Ingress has annotation and IP assigned
kubectl get ingress -l lynq.sh/node=acme-corp-web-app-with-dns

# Example output:
# NAME                    CLASS   HOSTS                    ADDRESS       PORTS   AGE
# acme-corp-ingress       nginx   acme.example.com         10.0.0.50     80      5m

# 2. Verify ExternalDNS annotation is present
kubectl get ingress acme-corp-ingress -o jsonpath='{.metadata.annotations.external-dns\.alpha\.kubernetes\.io/hostname}'
# Expected: acme.example.com

# 3. Check ExternalDNS logs for record creation
kubectl logs -n kube-system -l app.kubernetes.io/name=external-dns --tail=50 | grep acme.example.com

# Expected log entries:
# time="..." level=info msg="Desired change: CREATE acme.example.com A [Id: /hostedzone/Z...]"
# time="..." level=info msg="Desired change: CREATE acme.example.com TXT [Id: /hostedzone/Z...]"
# time="..." level=info msg="Record in target zone: acme.example.com. 300 IN A 10.0.0.50"

# 4. Verify DNS record with dig (global DNS servers)
dig acme.example.com @8.8.8.8 +short
# Expected: 10.0.0.50 (the LoadBalancer IP)

dig acme.example.com @1.1.1.1 +short
# Expected: 10.0.0.50

# 5. Check TXT ownership record
dig TXT acme.example.com +short
# Expected: "heritage=external-dns,external-dns/owner=my-cluster-id,..."
```

**Verify with Cloud Provider CLIs:**

::: code-group
```bash [AWS Route53]
# List hosted zones
aws route53 list-hosted-zones

# Get all records in zone
aws route53 list-resource-record-sets --hosted-zone-id Z1234567890 | jq '.ResourceRecordSets[] | select(.Name | contains("acme"))'

# Example output:
# {
#   "Name": "acme.example.com.",
#   "Type": "A",
#   "TTL": 300,
#   "ResourceRecords": [{"Value": "10.0.0.50"}]
# }

# Check specific record
aws route53 test-dns-answer --hosted-zone-id Z1234567890 --record-name acme.example.com --record-type A
# Expected: "ResponseCode": "NOERROR", "RecordData": ["10.0.0.50"]
```

```bash [Google Cloud DNS]
# List managed zones
gcloud dns managed-zones list

# List records in zone
gcloud dns record-sets list --zone=example-zone --filter="name=acme.example.com."

# Example output:
# NAME                  TYPE  TTL  DATA
# acme.example.com.     A     300  10.0.0.50
# acme.example.com.     TXT   300  "heritage=external-dns..."

# Describe specific record
gcloud dns record-sets describe acme.example.com. --zone=example-zone --type=A
```

```bash [Cloudflare]
# Get zone ID
curl -X GET "https://api.cloudflare.com/client/v4/zones?name=example.com" \
  -H "Authorization: Bearer $CF_API_TOKEN" | jq '.result[0].id'

# List DNS records
curl -X GET "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records?name=acme.example.com" \
  -H "Authorization: Bearer $CF_API_TOKEN" | jq '.result'

# Example output:
# [{
#   "name": "acme.example.com",
#   "type": "A",
#   "content": "10.0.0.50",
#   "ttl": 300,
#   "proxied": false
# }]
```
:::

**Monitor Both Systems:**

```bash
# Combined health check
echo "=== LynqNode Status ===" && \
kubectl get lynqnode acme-corp-web-app-with-dns -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' && \
echo "" && \
echo "=== Ingress IP ===" && \
kubectl get ingress acme-corp-ingress -o jsonpath='{.status.loadBalancer.ingress[0].ip}' && \
echo "" && \
echo "=== DNS Resolution ===" && \
dig +short acme.example.com @8.8.8.8

# Expected:
# === LynqNode Status ===
# True
# === Ingress IP ===
# 10.0.0.50
# === DNS Resolution ===
# 10.0.0.50
```

**Verify DNS for All Nodes:**

```bash
# Get all node hostnames and verify DNS
kubectl get ingress -l lynq.sh/template=web-app-with-dns -o jsonpath='{range .items[*]}{.spec.rules[0].host}{"\n"}{end}' | while read host; do
  ip=$(dig +short $host @8.8.8.8 | head -1)
  if [ -n "$ip" ]; then
    echo "✅ $host → $ip"
  else
    echo "❌ $host → NOT RESOLVED"
  fi
done

# Example output:
# ✅ acme.example.com → 10.0.0.50
# ✅ beta.example.com → 10.0.0.50
# ❌ gamma.example.com → NOT RESOLVED (propagating...)
```

**Check DNS Propagation Globally:**

```bash
# Online tools
# - https://dnschecker.org/#A/acme.example.com
# - https://www.whatsmydns.net/#A/acme.example.com

# Multiple DNS servers check
for ns in 8.8.8.8 1.1.1.1 208.67.222.222 9.9.9.9; do
  echo "DNS Server $ns: $(dig +short acme.example.com @$ns)"
done

# Example output:
# DNS Server 8.8.8.8: 10.0.0.50
# DNS Server 1.1.1.1: 10.0.0.50
# DNS Server 208.67.222.222: 10.0.0.50
# DNS Server 9.9.9.9: 10.0.0.50
```

## Troubleshooting

### DNS Records Not Created

**Problem:** DNS records don't appear in provider.

**Solution:**

1. **Check ExternalDNS logs:**
   ```bash
   kubectl logs -n kube-system -l app.kubernetes.io/name=external-dns
   ```

2. **Verify Ingress has IP:**
   ```bash
   kubectl get ingress <lynqnode-ingress> -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
   ```

3. **Check annotation syntax:**
   ```bash
   kubectl get ingress <lynqnode-ingress> -o yaml | grep external-dns
   ```

4. **Verify domain filter:**
   ```bash
   kubectl get deployment external-dns -n kube-system -o yaml | grep domain-filter
   ```

### DNS Records Not Deleted

**Problem:** DNS records remain after node deletion.

**Solution:**

1. **Check ExternalDNS policy:**
   - `policy=upsert-only` prevents deletion (change to `policy=sync`)
   - `policy=sync` allows ExternalDNS to delete records

2. **Check TXT records:**
   ```bash
   dig TXT <node-domain>
   ```
   TXT records track ownership - if owner doesn't match, record won't be deleted.

### DNS Propagation Delays

**Problem:** DNS changes take too long to propagate.

**Solution:**

1. **Reduce TTL:**
   ```yaml
   annotations:
     external-dns.alpha.kubernetes.io/ttl: "60"  # 1 minute
   ```

2. **Check DNS propagation:**
   ```bash
   dig <node-domain> @8.8.8.8
   dig <node-domain> @1.1.1.1
   ```

3. **Use DNS checker:**
   - https://dnschecker.org
   - https://www.whatsmydns.net

## Best Practices

### 1. Use Separate Hosted Zones

Use dedicated DNS zones for node subdomains:

```bash
# Production nodes
--domain-filter=example.com

# Staging nodes
--domain-filter=staging.example.com
```

### 2. Set Appropriate TTLs

```yaml
annotations:
  external-dns.alpha.kubernetes.io/ttl: "300"  # 5 minutes (good for production)
  # external-dns.alpha.kubernetes.io/ttl: "60"  # 1 minute (good for testing)
```

### 3. Use Policy: upsert-only for Safety

Prevent ExternalDNS from deleting existing records:

```bash
helm install external-dns bitnami/external-dns \
  --set policy=upsert-only
```

### 4. Monitor ExternalDNS Logs

```bash
kubectl logs -n kube-system -l app.kubernetes.io/name=external-dns -f
```

### 5. Combine with cert-manager

Auto-provision SSL certificates with DNS challenge:

```yaml
ingresses:
- id: secure-ingress
  nameTemplate: "{{ .uid }}-ingress"
  annotationsTemplate:
    external-dns.alpha.kubernetes.io/hostname: "{{ .host }}"
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
  spec:
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    spec:
      tls:
      - hosts:
        - "{{ .host }}"
        secretName: "{{ .uid }}-tls"
      rules:
      - host: "{{ .host }}"
        http:
          paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: "{{ .uid }}-svc"
                port:
                  number: 80
```

## See Also

- [ExternalDNS Documentation](https://github.com/kubernetes-sigs/external-dns)
- [ExternalDNS Provider List](https://github.com/kubernetes-sigs/external-dns#status-of-providers)
- [Lynq Templates Guide](templates.md)
- [cert-manager Integration](https://cert-manager.io/docs/)
- [Integration with Flux](integration-flux.md)
