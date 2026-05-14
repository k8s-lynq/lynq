---
description: "Production full-stack Crossplane + Lynq walkthrough: RDS database, S3 bucket, CloudFront CDN, and Kubernetes workloads provisioned per node."
---

# Crossplane Full-Stack Production Walkthrough

This guide deploys a complete production application per node:

- **PostgreSQL Database** — isolated database on a shared RDS instance
- **S3 Bucket** — static asset storage
- **CloudFront CDN** — global CDN (provisioned async; does not block node readiness)
- **Backend API** — Deployment with database and S3 access
- **Frontend** — Nginx with CDN acceleration

::: tip Time to working
Node becomes ready in ~5 minutes. CloudFront takes 15-30 minutes and provisions in the background.
:::

For the minimal setup, see [Crossplane Integration](integration-crossplane.md).

## Prerequisites

- Crossplane installed with AWS providers (RDS, S3, CloudFront, SQL). See [Crossplane Integration](integration-crossplane.md#installation).
- Shared RDS instance already running (see Step 1 below).

## Step 1: Shared RDS Instance

Create a single shared RDS instance once. Each node gets its own database within it.

```yaml
apiVersion: rds.aws.upbound.io/v1beta1
kind: Instance
metadata:
  name: shared-postgres
  namespace: default
spec:
  forProvider:
    region: us-east-1
    allocatedStorage: 100
    engine: postgres
    engineVersion: "15.4"
    instanceClass: db.t3.medium
    dbName: postgres
    username: postgres
    masterUserPasswordSecretRef:
      key: password
      name: rds-master-password
      namespace: default
    publiclyAccessible: false
    storageEncrypted: true
    multiAZ: true
    backupRetentionPeriod: 7
    deletionProtection: true
  writeConnectionSecretToRef:
    name: shared-postgres-connection
    namespace: default
  providerConfigRef:
    name: default
```

## Step 2: Full-Stack LynqForm

The LynqForm provisions all resources in dependency order. CloudFront is non-blocking (`waitForReady: false`) so nodes become ready in ~5 minutes instead of 30+.

::: v-pre
```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: production-app
spec:
  hubId: customer-hub

  manifests:
    # 1. Isolated PostgreSQL database
    - id: postgres-db
      nameTemplate: "{{ .uid }}-db"
      waitForReady: true
      timeoutSeconds: 300
      spec:
        apiVersion: postgresql.sql.crossplane.io/v1alpha1
        kind: Database
        metadata:
          annotations:
            crossplane.io/external-name: "node_{{ .uid | replace \"-\" \"_\" }}"
        spec:
          forProvider:
            connectHost: shared-postgres-connection
          providerConfigRef:
            name: postgres-provider

    # 2. Database user with isolated credentials
    - id: postgres-role
      nameTemplate: "{{ .uid }}-role"
      dependIds: ["postgres-db"]
      waitForReady: true
      spec:
        apiVersion: postgresql.sql.crossplane.io/v1alpha1
        kind: Role
        spec:
          forProvider:
            privileges:
              - LOGIN
              - NOSUPERUSER
            passwordSecretRef:
              key: password
              name: "{{ .uid }}-db-password"
              namespace: default
          writeConnectionSecretToRef:
            name: "{{ .uid }}-db-creds"
            namespace: default
          providerConfigRef:
            name: postgres-provider

    # 3. Grant database access
    - id: postgres-grant
      nameTemplate: "{{ .uid }}-grant"
      dependIds: ["postgres-role"]
      waitForReady: true
      spec:
        apiVersion: postgresql.sql.crossplane.io/v1alpha1
        kind: Grant
        spec:
          forProvider:
            privileges: ["ALL"]
            database: "node_{{ .uid | replace \"-\" \"_\" }}"
            role: "{{ .uid | replace \"-\" \"_\" }}_user"
          providerConfigRef:
            name: postgres-provider

    # 4. S3 bucket for static assets
    - id: s3-bucket
      nameTemplate: "{{ .uid }}-assets"
      waitForReady: true
      timeoutSeconds: 300
      spec:
        apiVersion: s3.aws.upbound.io/v1beta1
        kind: Bucket
        spec:
          forProvider:
            region: us-east-1
            tags:
              node-id: "{{ .uid }}"
              managed-by: lynq
          providerConfigRef:
            name: default

    # 5. CloudFront OAI for secure S3 access
    - id: cloudfront-oai
      nameTemplate: "{{ .uid }}-oai"
      dependIds: ["s3-bucket"]
      waitForReady: true
      spec:
        apiVersion: cloudfront.aws.upbound.io/v1beta1
        kind: OriginAccessIdentity
        spec:
          forProvider:
            comment: "OAI for {{ .uid }}"
          providerConfigRef:
            name: default

    # 6. CloudFront distribution (non-blocking)
    - id: cloudfront-distribution
      nameTemplate: "{{ .uid }}-cdn"
      dependIds: ["cloudfront-oai"]
      waitForReady: false   # Does NOT block node readiness
      deletionPolicy: Retain
      spec:
        apiVersion: cloudfront.aws.upbound.io/v1beta1
        kind: Distribution
        spec:
          forProvider:
            enabled: true
            origins:
              - domainName: "{{ .uid }}-assets.s3.amazonaws.com"
                originId: "s3-origin"
                s3OriginConfig:
                  originAccessIdentity: "origin-access-identity/cloudfront/{{ .uid }}-oai"
            defaultCacheBehavior:
              allowedMethods: [GET, HEAD]
              cachedMethods: [GET, HEAD]
              targetOriginId: s3-origin
              viewerProtocolPolicy: redirect-to-https
          providerConfigRef:
            name: default

  deployments:
    # 7. Backend API
    - id: backend
      nameTemplate: "{{ .uid }}-backend"
      dependIds: ["postgres-grant", "s3-bucket"]
      waitForReady: true
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 2
          template:
            spec:
              containers:
                - name: api
                  image: "{{ default \"myorg/api:latest\" .deployImage }}"
                  env:
                    - name: DB_HOST
                      valueFrom:
                        secretKeyRef:
                          name: shared-postgres-connection
                          key: endpoint
                    - name: DB_NAME
                      value: "node_{{ .uid | replace \"-\" \"_\" }}"
                    - name: DB_USER
                      value: "{{ .uid | replace \"-\" \"_\" }}_user"
                    - name: DB_PASSWORD
                      valueFrom:
                        secretKeyRef:
                          name: "{{ .uid }}-db-creds"
                          key: password
                    - name: S3_BUCKET
                      value: "{{ .uid }}-assets"

    # 8. Frontend
    - id: frontend
      nameTemplate: "{{ .uid }}-frontend"
      dependIds: ["backend"]
      waitForReady: true
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 2
          template:
            spec:
              containers:
                - name: nginx
                  image: nginx:alpine
                  env:
                    - name: S3_FALLBACK_URL
                      value: "https://{{ .uid }}-assets.s3.amazonaws.com"

  services:
    - id: backend-svc
      nameTemplate: "{{ .uid }}-backend"
      dependIds: ["backend"]
      spec:
        apiVersion: v1
        kind: Service
        spec:
          selector:
            app: "{{ .uid }}-backend"
          ports:
            - port: 80
              targetPort: 8080

    - id: frontend-svc
      nameTemplate: "{{ .uid }}-frontend"
      dependIds: ["frontend"]
      spec:
        apiVersion: v1
        kind: Service
        spec:
          selector:
            app: "{{ .uid }}-frontend"
          ports:
            - port: 80

  ingresses:
    - id: ingress
      nameTemplate: "{{ .uid }}-ingress"
      dependIds: ["backend-svc", "frontend-svc"]
      spec:
        apiVersion: networking.k8s.io/v1
        kind: Ingress
        spec:
          rules:
            - host: "{{ .uid }}.example.com"
              http:
                paths:
                  - path: /api
                    pathType: Prefix
                    backend:
                      service:
                        name: "{{ .uid }}-backend"
                        port:
                          number: 80
                  - path: /
                    pathType: Prefix
                    backend:
                      service:
                        name: "{{ .uid }}-frontend"
                        port:
                          number: 80
```
:::

## What Gets Provisioned

| # | Resource | Approx Time | Blocks Node? |
|---|----------|-------------|--------------|
| 1 | PostgreSQL Database | 30s | Yes |
| 2 | PostgreSQL Role | 15s | Yes |
| 3 | Database Grant | 10s | Yes |
| 4 | S3 Bucket | 60s | Yes |
| 5 | CloudFront OAI | 30s | Yes |
| 6 | CloudFront Distribution | 15–30 min | **No** (`waitForReady: false`) |
| 7 | Backend Deployment | 2 min | Yes |
| 8 | Frontend Deployment | 1 min | Yes |
| 9 | Services + Ingress | 30s | Yes |

**Node ready time: ~5 minutes.** CloudFront finishes in the background.

## Advanced Examples

### Schema-Based Multi-Node (Cost-Effective)

Create isolated schemas within a single database — fastest provisioning (seconds), lowest cost:

::: v-pre
```yaml
manifests:
  - id: postgres-schema
    nameTemplate: "{{ .uid }}-schema"
    spec:
      apiVersion: postgresql.sql.crossplane.io/v1alpha1
      kind: Schema
      metadata:
        annotations:
          crossplane.io/external-name: "node_{{ .uid | replace \"-\" \"_\" }}"
      spec:
        forProvider:
          database: shared_db
        providerConfigRef:
          name: postgres-provider
    waitForReady: true

  - id: schema-grant
    nameTemplate: "{{ .uid }}-schema-grant"
    dependIds: ["postgres-schema"]
    spec:
      apiVersion: postgresql.sql.crossplane.io/v1alpha1
      kind: Grant
      spec:
        forProvider:
          privileges: ["ALL"]
          schema: "node_{{ .uid | replace \"-\" \"_\" }}"
          database: shared_db
          role: "{{ .uid }}_user"
        providerConfigRef:
          name: postgres-provider
```
:::

### Dedicated RDS Instance (Premium Tier)

Full database isolation for high-value nodes:

::: v-pre
```yaml
manifests:
  - id: dedicated-rds
    nameTemplate: "{{ .uid }}-rds"
    deletionPolicy: Retain
    waitForReady: true
    timeoutSeconds: 1200
    spec:
      apiVersion: rds.aws.upbound.io/v1beta1
      kind: Instance
      spec:
        forProvider:
          region: us-east-1
          allocatedStorage: 50
          engine: postgres
          engineVersion: "15.4"
          instanceClass: db.t3.small
          dbName: "{{ .uid | replace \"-\" \"_\" }}"
          masterUserPasswordSecretRef:
            key: password
            name: "{{ .uid }}-rds-password"
            namespace: default
          storageEncrypted: true
          backupRetentionPeriod: 30
        writeConnectionSecretToRef:
          name: "{{ .uid }}-rds-connection"
          namespace: default
        providerConfigRef:
          name: default
```
:::

## Best Practices

| Practice | Why |
|----------|-----|
| `waitForReady: false` on CloudFront | 5-minute node readiness instead of 30+ minutes |
| `deletionPolicy: Retain` on databases | Prevent data loss when node is deactivated |
| `writeConnectionSecretToRef` for credentials | Avoid hardcoding in templates |
| Tag all cloud resources with `lynq.sh/uid` | Cost allocation and orphan detection |

## Verify It Works

```bash
# Node should be Ready in ~5 minutes
kubectl get lynqnode -l lynq.sh/hub=customer-hub

# Check all Crossplane resources for a specific node
kubectl get database,role,grant,bucket,distribution -l lynq.sh/node=acme-production-app

# Monitor CloudFront (async, takes 15-30 min)
kubectl get distribution acme-cdn -o jsonpath='{.status.atProvider.status}'
# InProgress → Deployed

# Verify database credentials were written
kubectl get secret acme-db-creds

# Test application endpoint
curl https://acme.example.com/api/health
```

## Troubleshooting

**CloudFront stuck in InProgress > 45 minutes**

This is abnormal — 15-30 minutes is expected. Check the AWS CloudFront console. The application should still be working via the S3 fallback URL.

**Database connection error**

```bash
kubectl get database acme-db -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
# If False: kubectl describe database acme-db
kubectl get secret acme-db-creds  # Verify credentials were written
```

**S3 access denied**

Verify the CloudFront OAI is referenced correctly in the bucket policy. Check:
```bash
kubectl get bucket acme-assets -o jsonpath='{.status.conditions[?(@.type=="Ready")]}'
```

**Crossplane provider unhealthy**

```bash
kubectl get providers
kubectl logs -n crossplane-system -l pkg.crossplane.io/provider=provider-aws-rds
kubectl delete pod -n crossplane-system -l pkg.crossplane.io/provider=provider-aws-rds
```

## See Also

- [Crossplane Integration](integration-crossplane.md) — Overview, minimal example, prerequisites.
- [Dependencies](dependencies.md) — `waitForReady` and `dependIds` for ordered provisioning.
- [Policies](policies.md) — `deletionPolicy: Retain` for stateful cloud resources.
- [Crossplane Docs](https://docs.crossplane.io/) — Provider-specific CRD reference.
