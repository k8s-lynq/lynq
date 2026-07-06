---
url: 'https://lynq.sh/use-case-custom-domains.md'
description: >-
  Per-node custom domains with automatic DNS records (ExternalDNS) and TLS
  certificates (cert-manager), driven by a database column. No manual DNS ops.
---

# Custom Domain Provisioning

Your users want `app.theirdomain.com` instead of `acme-corp.yourplatform.com`. The challenge: issuing TLS certificates, creating DNS records, and tearing it all down when a customer churns — multiplied by every customer.

Set `domain_verified = TRUE` in your database and Lynq creates the Ingress. ExternalDNS creates the DNS record. cert-manager issues the certificate. All three happen automatically.

::: tip Time to working
\~10 minutes to configure (includes cert-manager and ExternalDNS setup). Custom domain goes live within 2–5 minutes of `domain_verified = TRUE`.
:::

## How It Works

* Each node has a default subdomain (`acme-corp.yourplatform.com`) provisioned unconditionally.
* When `domain_verified = TRUE`, Lynq also creates a second Ingress for the custom domain — ExternalDNS and cert-manager pick it up automatically via annotations.
* Setting `domain_verified = FALSE` or removing the row deletes the custom-domain Ingress and its certificate.

## Prerequisites

cert-manager and ExternalDNS must be installed in your cluster. See [ExternalDNS Integration](./integration-external-dns.md) for a complete setup guide.

```bash
# Verify both are running before proceeding
kubectl get pods -n cert-manager
kubectl get pods -l app.kubernetes.io/name=external-dns
```

## Database Schema

```sql
CREATE TABLE nodes (
  node_id         VARCHAR(63)   PRIMARY KEY,
  is_active       BOOLEAN       DEFAULT TRUE,
  subdomain       VARCHAR(255)  NOT NULL,           -- default: <node_id>.yourplatform.com
  custom_domain   VARCHAR(255),                     -- e.g. 'app.acme.com'
  domain_verified BOOLEAN       DEFAULT FALSE,
  cname_target    VARCHAR(255),                     -- shown to the customer for CNAME setup

  -- Pre-computed resource limits; set by application layer when plan changes.
  cpu_request     VARCHAR(10)   DEFAULT '200m',
  memory_request  VARCHAR(10)   DEFAULT '512Mi',
  cpu_limit       VARCHAR(10)   DEFAULT '400m',
  memory_limit    VARCHAR(10)   DEFAULT '1Gi'
);
```

## Minimal Setup

Default subdomain for every node — no prerequisites beyond nginx-ingress.

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: domain-nodes
  namespace: lynq-system
spec:
  source:
    type: mysql
    syncInterval: 1m
    mysql:
      host: mysql.internal.svc.cluster.local
      port: 3306
      database: nodes_db
      table: nodes
      username: lynq_reader
      passwordRef:
        name: mysql-credentials
        key: password
  valueMappings:
    uid: node_id
    activate: is_active
  extraValueMappings:
    customDomain: custom_domain
    domainVerified: domain_verified
```

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: domain-stack
  namespace: lynq-system
spec:
  hubId: domain-nodes

  deployments:
    - id: app
      nameTemplate: "{{ .uid }}-web"
      waitForReady: true
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
                  image: registry.example.com/app:latest
                  ports:
                    - containerPort: 8080

  services:
    - id: svc
      nameTemplate: "{{ .uid }}-web"
      dependIds: ["app"]
      spec:
        apiVersion: v1
        kind: Service
        spec:
          selector:
            app: "{{ .uid }}"
          ports:
            - port: 80
              targetPort: 8080

  ingresses:
    - id: default-ingress
      nameTemplate: "{{ .uid }}-default"
      dependIds: ["svc"]
      spec:
        apiVersion: networking.k8s.io/v1
        kind: Ingress
        metadata:
          annotations:
            cert-manager.io/cluster-issuer: letsencrypt-prod
        spec:
          ingressClassName: nginx
          tls:
            - hosts:
                - "{{ .uid }}.yourplatform.com"
              secretName: "{{ .uid }}-default-tls"
          rules:
            - host: "{{ .uid }}.yourplatform.com"
              http:
                paths:
                  - path: /
                    pathType: Prefix
                    backend:
                      service:
                        name: "{{ .uid }}-web"
                        port:
                          number: 80
```

## Full Example

Full production setup: default subdomain + custom domain (when verified), with ExternalDNS annotations, resource limits from DB, and per-node namespace.

### LynqHub

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: domain-nodes
  namespace: lynq-system
spec:
  source:
    type: mysql
    syncInterval: 1m
    mysql:
      host: mysql.internal.svc.cluster.local
      port: 3306
      database: nodes_db
      table: nodes
      username: lynq_reader
      passwordRef:
        name: mysql-credentials
        key: password
  valueMappings:
    uid: node_id
    activate: is_active
  extraValueMappings:
    customDomain: custom_domain
    domainVerified: domain_verified
    cnameTarget: cname_target
    cpuRequest: cpu_request
    memoryRequest: memory_request
    cpuLimit: cpu_limit
    memoryLimit: memory_limit
```

### LynqForm

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: domain-stack
  namespace: lynq-system
spec:
  hubId: domain-nodes

  namespaces:
    - id: ns
      nameTemplate: "node-{{ .uid }}"
      spec:
        apiVersion: v1
        kind: Namespace
        metadata:
          labels:
            node-id: "{{ .uid }}"

  deployments:
    - id: app
      nameTemplate: "{{ .uid }}-web"
      targetNamespace: "node-{{ .uid }}"
      dependIds: ["ns"]
      waitForReady: true
      timeoutSeconds: 300
      spec:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          labels:
            app: "{{ .uid }}-web"
        spec:
          replicas: 2
          selector:
            matchLabels:
              app: "{{ .uid }}-web"
          template:
            metadata:
              labels:
                app: "{{ .uid }}-web"
            spec:
              containers:
                - name: app
                  image: registry.example.com/app:latest
                  env:
                    - name: NODE_ID
                      value: "{{ .uid }}"
                    - name: NODE_DOMAIN
                      value: "{{ if and .customDomain (eq .domainVerified \"true\") }}{{ .customDomain }}{{ else }}{{ .uid }}.yourplatform.com{{ end }}"
                  ports:
                    - containerPort: 8080
                      name: http
                  resources:
                    requests:
                      cpu: "{{ .cpuRequest | default \"200m\" }}"
                      memory: "{{ .memoryRequest | default \"512Mi\" }}"
                    limits:
                      cpu: "{{ .cpuLimit | default \"400m\" }}"
                      memory: "{{ .memoryLimit | default \"1Gi\" }}"
                  readinessProbe:
                    httpGet:
                      path: /ready
                      port: http
                    initialDelaySeconds: 5
                    periodSeconds: 5

  services:
    - id: svc
      nameTemplate: "{{ .uid }}-web"
      targetNamespace: "node-{{ .uid }}"
      dependIds: ["app"]
      spec:
        apiVersion: v1
        kind: Service
        spec:
          selector:
            app: "{{ .uid }}-web"
          ports:
            - port: 80
              targetPort: http

  ingresses:
    - id: default-ingress
      nameTemplate: "{{ .uid }}-default"
      targetNamespace: "node-{{ .uid }}"
      dependIds: ["svc"]
      spec:
        apiVersion: networking.k8s.io/v1
        kind: Ingress
        metadata:
          annotations:
            cert-manager.io/cluster-issuer: letsencrypt-prod
            external-dns.alpha.kubernetes.io/hostname: "{{ .uid }}.yourplatform.com"
            external-dns.alpha.kubernetes.io/ttl: "300"
        spec:
          ingressClassName: nginx
          tls:
            - hosts:
                - "{{ .uid }}.yourplatform.com"
              secretName: "{{ .uid }}-default-tls"
          rules:
            - host: "{{ .uid }}.yourplatform.com"
              http:
                paths:
                  - path: /
                    pathType: Prefix
                    backend:
                      service:
                        name: "{{ .uid }}-web"
                        port:
                          number: 80

    - id: custom-ingress
      nameTemplate: "{{ .uid }}-custom"
      targetNamespace: "node-{{ .uid }}"
      dependIds: ["svc"]
      spec:
        apiVersion: networking.k8s.io/v1
        kind: Ingress
        metadata:
          annotations:
            cert-manager.io/cluster-issuer: letsencrypt-prod
            external-dns.alpha.kubernetes.io/hostname: "{{ .customDomain }}"
            external-dns.alpha.kubernetes.io/ttl: "300"
        spec:
          ingressClassName: nginx
          tls:
            - hosts:
                - "{{ .customDomain }}"
              secretName: "{{ .uid }}-custom-tls"
          rules:
            - host: "{{ .customDomain }}"
              http:
                paths:
                  - path: /
                    pathType: Prefix
                    backend:
                      service:
                        name: "{{ .uid }}-web"
                        port:
                          number: 80
```

::: tip Filtering unverified domains
The custom-ingress above is always created. To avoid creating Ingress for nodes with no `customDomain`, use a database view as the hub's table:

```sql
CREATE VIEW nodes_with_verified_domains AS
SELECT * FROM nodes
WHERE custom_domain IS NOT NULL AND domain_verified = TRUE AND is_active = TRUE;
```

Create a separate LynqHub pointing to this view, and reference it from the custom-ingress LynqForm.
:::

## Domain Verification Workflow

1. User enters `app.acme.com` in your portal → `custom_domain = 'app.acme.com'`, `domain_verified = FALSE`
2. Show user: "Point CNAME for `app.acme.com` to `acme-corp.yourplatform.com`"
3. Your background job checks DNS periodically
4. CNAME confirmed → `domain_verified = TRUE`
5. Lynq creates the custom-domain Ingress on the next sync
6. ExternalDNS creates the Route53/Cloudflare record
7. cert-manager issues the Let's Encrypt certificate

## Verify It Works

```bash
# Default subdomain has Ingress and certificate
kubectl get ingress -n node-acme-corp
# NAME                CLASS   HOSTS                         ADDRESS      PORTS     AGE
# acme-corp-default   nginx   acme-corp.yourplatform.com    10.0.0.50    80, 443   5m

kubectl get certificate -n node-acme-corp
# NAME                    READY   SECRET                    AGE
# acme-corp-default-tls   True    acme-corp-default-tls     5m

# After setting domain_verified = TRUE:
kubectl get ingress acme-corp-custom -n node-acme-corp
# NAME               CLASS   HOSTS         ADDRESS      PORTS     AGE
# acme-corp-custom   nginx   app.acme.com  10.0.0.50    80, 443   2m

# DNS resolves correctly
dig app.acme.com CNAME +short
# acme-corp.yourplatform.com.
```

## Caveats

* **DNS propagation takes time** even after the Ingress is created. TTL `300` (5 minutes) is a good starting point; lower values help during initial setup.
* **HTTP-01 challenge requires the domain to resolve to your ingress** before the certificate can be issued. If the CNAME isn't set correctly, cert-manager will retry but the certificate will remain in `pending` state.
* **Wildcard domains** (`*.acme.com`) require DNS-01 challenge — configure your DNS provider plugin for cert-manager before attempting this.

## See Also

* [ExternalDNS Integration](./integration-external-dns.md) — full DNS provider setup (Route53, Cloudflare, etc.)
* [Preview Environments](./use-case-preview-environments.md) — same Ingress + TLS pattern applied to per-PR environments
* [Policies](./policies.md) — `deletionPolicy: Retain` if you need to keep certificates during node transitions
