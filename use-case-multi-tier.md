---
url: 'https://lynq.sh/use-case-multi-tier.md'
description: >-
  Full-stack application stacks (web, API, worker, database) provisioned per
  node from a single database row, with dependency-ordered startup.
---

# Multi-Tier Application Stack

Provisioning an isolated full-stack environment per customer — web frontend, API backend, background worker, and database — used to mean writing orchestration scripts or Helm umbrella charts that someone has to maintain. With Lynq, each tier is a separate LynqForm. One database row activates all of them.

Add a row to your `nodes` table and four LynqNodes come to life: data tier first (PostgreSQL + Redis), then API and worker (both depend on data), then web frontend (depends on API). Remove the row and all four are cleaned up.

::: tip Time to working
\~10 minutes to configure. Full stack provisions in ~2–3 minutes per node.
:::

## How It Works

* One LynqHub, four LynqForms. Each form manages one tier independently — different resource configs, policies, and scaling rules.
* `dependIds` within each LynqForm enforce resource ordering inside a tier. Cross-tier ordering is handled by Kubernetes-native service discovery (the API tier connects to PostgreSQL via DNS once it's running).
* The data tier uses `creationPolicy: Once` and `deletionPolicy: Retain` on the PostgreSQL StatefulSet — the database survives node deletion.

## Database Schema

```sql
CREATE TABLE nodes (
  node_id          VARCHAR(63)  PRIMARY KEY,
  is_active        BOOLEAN      DEFAULT TRUE,

  -- Per-tier replica counts
  web_replicas     INT          DEFAULT 2,
  api_replicas     INT          DEFAULT 3,
  worker_replicas  INT          DEFAULT 2,

  -- Database tier sizing (pre-computed by application layer or DB view)
  db_cpu_request   VARCHAR(10)  DEFAULT '500m',   -- '500m' | '1000m' | '2000m'
  db_memory_request VARCHAR(10) DEFAULT '1Gi',    -- '1Gi'  | '2Gi'   | '4Gi'
  db_storage_size  VARCHAR(10)  DEFAULT '20Gi',   -- '20Gi' | '50Gi'  | '100Gi'

  -- Feature flags consumed by API tier
  enable_analytics      BOOLEAN DEFAULT FALSE,
  enable_notifications  BOOLEAN DEFAULT TRUE
);
```

## LynqHub

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: multi-tier-nodes
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
    webReplicas: web_replicas
    apiReplicas: api_replicas
    workerReplicas: worker_replicas
    dbCpu: db_cpu_request
    dbMemory: db_memory_request
    dbStorage: db_storage_size
    enableAnalytics: enable_analytics
    enableNotifications: enable_notifications
```

## Minimal Setup

Two tiers (data + API) to demonstrate the cross-tier dependency pattern before adding the full stack.

```yaml
# LynqForm: data-tier — PostgreSQL + headless Service
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: data-tier
  namespace: lynq-system
spec:
  hubId: multi-tier-nodes

  namespaces:
    - id: ns
      nameTemplate: "node-{{ .uid }}"
      spec:
        apiVersion: v1
        kind: Namespace
        metadata:
          labels:
            node-id: "{{ .uid }}"

  statefulSets:
    - id: postgres
      nameTemplate: "{{ .uid }}-postgres"
      targetNamespace: "node-{{ .uid }}"
      dependIds: ["ns"]
      creationPolicy: Once
      deletionPolicy: Retain
      waitForReady: true
      timeoutSeconds: 600
      spec:
        apiVersion: apps/v1
        kind: StatefulSet
        spec:
          serviceName: "{{ .uid }}-postgres"
          replicas: 1
          selector:
            matchLabels:
              app: "{{ .uid }}-postgres"
          template:
            metadata:
              labels:
                app: "{{ .uid }}-postgres"
            spec:
              containers:
                - name: postgres
                  image: postgres:15-alpine
                  env:
                    - name: POSTGRES_DB
                      value: "{{ .uid }}"
                    - name: POSTGRES_USER
                      value: "{{ .uid }}"
                    - name: POSTGRES_PASSWORD
                      valueFrom:
                        secretKeyRef:
                          name: "{{ .uid }}-db-credentials"
                          key: password
                  ports:
                    - containerPort: 5432
                      name: postgres

  services:
    - id: postgres-svc
      nameTemplate: "{{ .uid }}-postgres"
      targetNamespace: "node-{{ .uid }}"
      dependIds: ["postgres"]
      spec:
        apiVersion: v1
        kind: Service
        spec:
          clusterIP: None  # headless for StatefulSet
          selector:
            app: "{{ .uid }}-postgres"
          ports:
            - port: 5432
              targetPort: postgres
```

```yaml
# LynqForm: api-tier — connects to PostgreSQL via service discovery
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: api-tier
  namespace: lynq-system
spec:
  hubId: multi-tier-nodes

  deployments:
    - id: api
      nameTemplate: "{{ .uid }}-api"
      targetNamespace: "node-{{ .uid }}"
      waitForReady: true
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: {{ .apiReplicas | int }}
          selector:
            matchLabels:
              app: "{{ .uid }}-api"
          template:
            metadata:
              labels:
                app: "{{ .uid }}-api"
            spec:
              containers:
                - name: api
                  image: registry.example.com/node-api:v2.0.0
                  env:
                    - name: DATABASE_URL
                      valueFrom:
                        secretKeyRef:
                          name: "{{ .uid }}-db-credentials"
                          key: connection-string
                  ports:
                    - containerPort: 8080
                      name: http
```

## Full Example

All four tiers with complete resource configurations.

### Data Tier

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: data-tier
  namespace: lynq-system
spec:
  hubId: multi-tier-nodes

  namespaces:
    - id: ns
      nameTemplate: "node-{{ .uid }}"
      spec:
        apiVersion: v1
        kind: Namespace
        metadata:
          labels:
            node-id: "{{ .uid }}"

  secrets:
    - id: db-credentials
      nameTemplate: "{{ .uid }}-db-credentials"
      targetNamespace: "node-{{ .uid }}"
      dependIds: ["ns"]
      creationPolicy: Once
      spec:
        apiVersion: v1
        kind: Secret
        stringData:
          password: "{{ randAlphaNum 32 }}"
          connection-string: "postgresql://{{ .uid }}:REPLACE_WITH_PASSWORD@{{ .uid }}-postgres:5432/{{ .uid }}"

  statefulSets:
    - id: postgres
      nameTemplate: "{{ .uid }}-postgres"
      targetNamespace: "node-{{ .uid }}"
      dependIds: ["ns", "db-credentials"]
      creationPolicy: Once
      deletionPolicy: Retain
      waitForReady: true
      timeoutSeconds: 600
      spec:
        apiVersion: apps/v1
        kind: StatefulSet
        metadata:
          labels:
            app: "{{ .uid }}-postgres"
        spec:
          serviceName: "{{ .uid }}-postgres"
          replicas: 1
          selector:
            matchLabels:
              app: "{{ .uid }}-postgres"
          template:
            metadata:
              labels:
                app: "{{ .uid }}-postgres"
            spec:
              containers:
                - name: postgres
                  image: postgres:15-alpine
                  env:
                    - name: POSTGRES_DB
                      value: "{{ .uid }}"
                    - name: POSTGRES_USER
                      value: "{{ .uid }}"
                    - name: POSTGRES_PASSWORD
                      valueFrom:
                        secretKeyRef:
                          name: "{{ .uid }}-db-credentials"
                          key: password
                    - name: PGDATA
                      value: /var/lib/postgresql/data/pgdata
                  ports:
                    - containerPort: 5432
                      name: postgres
                  resources:
                    requests:
                      cpu: "{{ .dbCpu | default \"500m\" }}"
                      memory: "{{ .dbMemory | default \"1Gi\" }}"
                  volumeMounts:
                    - name: data
                      mountPath: /var/lib/postgresql/data
          volumeClaimTemplates:
            - metadata:
                name: data
              spec:
                accessModes: ["ReadWriteOnce"]
                resources:
                  requests:
                    storage: "{{ .dbStorage | default \"20Gi\" }}"

  deployments:
    - id: redis
      nameTemplate: "{{ .uid }}-redis"
      targetNamespace: "node-{{ .uid }}"
      dependIds: ["ns"]
      waitForReady: true
      spec:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          labels:
            app: "{{ .uid }}-redis"
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: "{{ .uid }}-redis"
          template:
            metadata:
              labels:
                app: "{{ .uid }}-redis"
            spec:
              containers:
                - name: redis
                  image: redis:7-alpine
                  ports:
                    - containerPort: 6379
                      name: redis
                  resources:
                    requests:
                      cpu: 200m
                      memory: 512Mi
                    limits:
                      cpu: 400m
                      memory: 1Gi

  services:
    - id: postgres-svc
      nameTemplate: "{{ .uid }}-postgres"
      targetNamespace: "node-{{ .uid }}"
      dependIds: ["postgres"]
      spec:
        apiVersion: v1
        kind: Service
        spec:
          clusterIP: None
          selector:
            app: "{{ .uid }}-postgres"
          ports:
            - port: 5432
              targetPort: postgres

    - id: redis-svc
      nameTemplate: "{{ .uid }}-redis"
      targetNamespace: "node-{{ .uid }}"
      dependIds: ["redis"]
      spec:
        apiVersion: v1
        kind: Service
        spec:
          selector:
            app: "{{ .uid }}-redis"
          ports:
            - port: 6379
              targetPort: redis
```

::: tip Secret generation
`randAlphaNum 32` generates a random password at first creation. Use `creationPolicy: Once` so it's generated once and never overwritten. In production, consider [External Secrets Operator](https://external-secrets.io) to pull credentials from a vault.
:::

### API Tier

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: api-tier
  namespace: lynq-system
spec:
  hubId: multi-tier-nodes

  deployments:
    - id: api
      nameTemplate: "{{ .uid }}-api"
      targetNamespace: "node-{{ .uid }}"
      waitForReady: true
      timeoutSeconds: 300
      spec:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          labels:
            app: "{{ .uid }}-api"
        spec:
          replicas: {{ .apiReplicas | int }}
          selector:
            matchLabels:
              app: "{{ .uid }}-api"
          template:
            metadata:
              labels:
                app: "{{ .uid }}-api"
            spec:
              containers:
                - name: api
                  image: registry.example.com/node-api:v2.0.0
                  env:
                    - name: NODE_ID
                      value: "{{ .uid }}"
                    - name: DATABASE_URL
                      valueFrom:
                        secretKeyRef:
                          name: "{{ .uid }}-db-credentials"
                          key: connection-string
                    - name: REDIS_URL
                      value: "redis://{{ .uid }}-redis:6379"
                    - name: ENABLE_ANALYTICS
                      value: "{{ .enableAnalytics }}"
                  ports:
                    - containerPort: 8080
                      name: http
                  resources:
                    requests:
                      cpu: 500m
                      memory: 1Gi
                    limits:
                      cpu: 1000m
                      memory: 2Gi
                  readinessProbe:
                    httpGet:
                      path: /ready
                      port: http
                    initialDelaySeconds: 10
                    periodSeconds: 5

  services:
    - id: api-svc
      nameTemplate: "{{ .uid }}-api"
      targetNamespace: "node-{{ .uid }}"
      dependIds: ["api"]
      spec:
        apiVersion: v1
        kind: Service
        spec:
          selector:
            app: "{{ .uid }}-api"
          ports:
            - port: 8080
              targetPort: http
```

### Web Tier

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: web-tier
  namespace: lynq-system
spec:
  hubId: multi-tier-nodes

  deployments:
    - id: web
      nameTemplate: "{{ .uid }}-web"
      targetNamespace: "node-{{ .uid }}"
      waitForReady: true
      spec:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          labels:
            app: "{{ .uid }}-web"
        spec:
          replicas: {{ .webReplicas | int }}
          selector:
            matchLabels:
              app: "{{ .uid }}-web"
          template:
            metadata:
              labels:
                app: "{{ .uid }}-web"
            spec:
              containers:
                - name: web
                  image: registry.example.com/node-web:v2.0.0
                  env:
                    - name: NODE_ID
                      value: "{{ .uid }}"
                    - name: API_URL
                      value: "http://{{ .uid }}-api:8080"
                  ports:
                    - containerPort: 3000
                      name: http
                  resources:
                    requests:
                      cpu: 200m
                      memory: 512Mi

  services:
    - id: web-svc
      nameTemplate: "{{ .uid }}-web"
      targetNamespace: "node-{{ .uid }}"
      dependIds: ["web"]
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
    - id: ingress
      nameTemplate: "{{ .uid }}-ingress"
      targetNamespace: "node-{{ .uid }}"
      dependIds: ["web-svc"]
      spec:
        apiVersion: networking.k8s.io/v1
        kind: Ingress
        spec:
          ingressClassName: nginx
          rules:
            - host: "{{ .uid }}.example.com"
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

### Worker Tier

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: worker-tier
  namespace: lynq-system
spec:
  hubId: multi-tier-nodes

  deployments:
    - id: worker
      nameTemplate: "{{ .uid }}-worker"
      targetNamespace: "node-{{ .uid }}"
      waitForReady: true
      spec:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          labels:
            app: "{{ .uid }}-worker"
        spec:
          replicas: {{ .workerReplicas | int }}
          selector:
            matchLabels:
              app: "{{ .uid }}-worker"
          template:
            metadata:
              labels:
                app: "{{ .uid }}-worker"
            spec:
              containers:
                - name: worker
                  image: registry.example.com/node-worker:v2.0.0
                  env:
                    - name: NODE_ID
                      value: "{{ .uid }}"
                    - name: DATABASE_URL
                      valueFrom:
                        secretKeyRef:
                          name: "{{ .uid }}-db-credentials"
                          key: connection-string
                    - name: REDIS_URL
                      value: "redis://{{ .uid }}-redis:6379"
                    - name: ENABLE_NOTIFICATIONS
                      value: "{{ .enableNotifications }}"
                  resources:
                    requests:
                      cpu: 300m
                      memory: 768Mi
```

## Startup Timeline

| Time | Data Tier | API + Worker | Web |
|------|-----------|--------------|-----|
| T+0 | Creating namespace, secrets... | Waiting | Waiting |
| T+30s | PostgreSQL starting, Redis starting... | Waiting | Waiting |
| T+60s | All ready | Starting... | Waiting |
| T+90s | Ready | Ready | Starting... |
| T+120s | Ready | Ready | Ready + Ingress live |

**Total: ~2–3 minutes per node.** Cross-tier ordering is managed by Kubernetes service discovery — the API Deployment starts when it's created and will retry the database connection until PostgreSQL is ready.

## Verify It Works

```bash
# All 4 LynqNodes ready
kubectl get lynqnodes -n lynq-system | grep acme-corp
# acme-corp-data-tier      True    5/5   0   10m
# acme-corp-api-tier       True    2/2   0   10m
# acme-corp-web-tier       True    3/3   0   10m
# acme-corp-worker-tier    True    2/2   0   10m

# All resources in node namespace
kubectl get all -n node-acme-corp
# Deployments: acme-corp-redis, acme-corp-api, acme-corp-web, acme-corp-worker
# StatefulSets: acme-corp-postgres
# Services: acme-corp-postgres, acme-corp-redis, acme-corp-api, acme-corp-web
# Ingress: acme-corp-ingress → acme-corp.example.com
```

## Caveats

* **Cross-tier dependencies aren't expressed in Lynq** — each LynqForm is independent. The API tier Deployment will start and retry connecting to PostgreSQL. If your app has no retry logic and crashes on startup, add an `initContainer` that waits for the DB port.
* **Scaling a specific tier** is a database column update: `UPDATE nodes SET api_replicas = 5 WHERE node_id = 'acme-corp'`. Lynq updates the Deployment replicas on the next sync.
* **The PostgreSQL Secret uses `randAlphaNum`**, which means the `connection-string` value stores a placeholder — not the actual password. For production, use External Secrets Operator or manage credentials separately.

## See Also

* [Database per Node with Crossplane](./use-case-database-per-tenant.md) — managed RDS instead of in-cluster PostgreSQL
* [Feature Flags](./use-case-feature-flags.md) — use the `enable_analytics` / `enable_notifications` columns as infrastructure switches for optional tiers
* [Dependencies](./dependencies.md) — `dependIds` and `waitForReady` within a single LynqForm
