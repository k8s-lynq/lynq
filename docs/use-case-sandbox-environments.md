---
description: "Isolated API sandbox environments per developer account — provisioned automatically when an account is created, updated when the plan changes, torn down when the account closes."
---

# Developer Sandbox Environments

Stripe gives every account a test-mode environment that's completely isolated from production. Twilio does the same. The challenge isn't building the first one — it's keeping up as you scale to thousands of accounts, handling plan upgrades that change resource limits, and cleaning up without leaving orphaned resources.

With Lynq, your accounts table drives the sandboxes. A new row provisions a namespace with an API server and mock webhook sink. Upgrading a plan updates the replica count and rate limits automatically. Closing an account removes everything.

::: tip Time to working
~5 minutes to configure. Each account gets a fully isolated sandbox within ~60 seconds of INSERT.
:::

## How It Works

- Each account row maps to one sandbox: an isolated namespace, API server Deployment, mock webhook sink, and per-account config.
- When `plan_type` changes, the hub re-reads the pre-computed resource columns and Lynq reconciles the Deployment on the next sync.
- Setting `is_active = FALSE` or deleting the row removes the namespace and all resources inside it.

## Database Schema

```sql
CREATE TABLE developer_accounts (
  account_id     VARCHAR(63)   PRIMARY KEY,   -- e.g. 'acct_1234abc'
  plan_type      VARCHAR(20)   NOT NULL DEFAULT 'free',  -- free | starter | growth | enterprise
  region         VARCHAR(20)   NOT NULL DEFAULT 'us-east-1',
  is_active      BOOLEAN       DEFAULT TRUE,

  -- Pre-computed resource limits; set by application layer when plan_type changes.
  -- Avoids multi-tier conditionals in templates.
  api_replicas   INT           DEFAULT 1,
  rate_limit_rps INT           DEFAULT 100,
  storage_gb     INT           DEFAULT 5,

  owner_email    VARCHAR(255),
  created_at     TIMESTAMP     DEFAULT CURRENT_TIMESTAMP,
  closed_at      TIMESTAMP     NULL
);
```

A database view pre-computes limits from `plan_type` so templates stay simple:

```sql
CREATE VIEW developer_accounts_with_limits AS
SELECT *,
  CASE plan_type
    WHEN 'enterprise' THEN 4
    WHEN 'growth'     THEN 2
    ELSE 1
  END AS api_replicas,
  CASE plan_type
    WHEN 'enterprise' THEN 5000
    WHEN 'growth'     THEN 1000
    WHEN 'starter'    THEN 300
    ELSE 100
  END AS rate_limit_rps
FROM developer_accounts
WHERE is_active = TRUE AND closed_at IS NULL;
```

## LynqHub

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: sandbox-hub
  namespace: lynq-system
spec:
  source:
    type: mysql
    syncInterval: 1m
    mysql:
      host: mysql.internal.svc.cluster.local
      port: 3306
      database: accounts_db
      table: developer_accounts_with_limits
      username: lynq_reader
      passwordRef:
        name: mysql-credentials
        key: password
  valueMappings:
    uid: account_id
    activate: is_active
  extraValueMappings:
    planType: plan_type
    region: region
    apiReplicas: api_replicas
    rateLimitRps: rate_limit_rps
    storageGb: storage_gb
    ownerEmail: owner_email
```

## LynqForm

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: sandbox-stack
  namespace: lynq-system
spec:
  hubId: sandbox-hub

  namespaces:
    - id: ns
      nameTemplate: "sandbox-{{ .uid }}"
      spec:
        apiVersion: v1
        kind: Namespace
        metadata:
          labels:
            sandbox-account: "{{ .uid }}"
            plan-type: "{{ .planType }}"

  configMaps:
    - id: config
      nameTemplate: "{{ .uid }}-config"
      targetNamespace: "sandbox-{{ .uid }}"
      dependIds: ["ns"]
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          ACCOUNT_ID: "{{ .uid }}"
          PLAN_TYPE: "{{ .planType }}"
          REGION: "{{ .region }}"
          RATE_LIMIT_RPS: "{{ .rateLimitRps }}"
          OWNER_EMAIL: "{{ .ownerEmail }}"
          SANDBOX_MODE: "true"

  deployments:
    - id: api
      nameTemplate: "{{ .uid }}-api"
      targetNamespace: "sandbox-{{ .uid }}"
      dependIds: ["config"]
      deletionPolicy: Delete
      waitForReady: true
      timeoutSeconds: 120
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
                  image: registry.example.com/sandbox-api:latest
                  ports:
                    - containerPort: 8080
                  envFrom:
                    - configMapRef:
                        name: "{{ .uid }}-config"
                  resources:
                    requests:
                      cpu: 200m
                      memory: 256Mi
                    limits:
                      cpu: 500m
                      memory: 512Mi
                  readinessProbe:
                    httpGet:
                      path: /healthz
                      port: 8080
                    initialDelaySeconds: 5
                    periodSeconds: 10

    - id: webhook-sink
      nameTemplate: "{{ .uid }}-webhook-sink"
      targetNamespace: "sandbox-{{ .uid }}"
      dependIds: ["ns"]
      deletionPolicy: Delete
      spec:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          labels:
            app: "{{ .uid }}-webhook-sink"
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: "{{ .uid }}-webhook-sink"
          template:
            metadata:
              labels:
                app: "{{ .uid }}-webhook-sink"
            spec:
              containers:
                - name: sink
                  image: registry.example.com/webhook-sink:latest
                  ports:
                    - containerPort: 9000
                  env:
                    - name: ACCOUNT_ID
                      value: "{{ .uid }}"
                  resources:
                    requests:
                      cpu: 50m
                      memory: 64Mi

  services:
    - id: api-svc
      nameTemplate: "{{ .uid }}-api"
      targetNamespace: "sandbox-{{ .uid }}"
      dependIds: ["api"]
      deletionPolicy: Delete
      spec:
        apiVersion: v1
        kind: Service
        spec:
          selector:
            app: "{{ .uid }}-api"
          ports:
            - port: 80
              targetPort: 8080

    - id: webhook-svc
      nameTemplate: "{{ .uid }}-webhook-sink"
      targetNamespace: "sandbox-{{ .uid }}"
      dependIds: ["webhook-sink"]
      deletionPolicy: Delete
      spec:
        apiVersion: v1
        kind: Service
        spec:
          selector:
            app: "{{ .uid }}-webhook-sink"
          ports:
            - port: 80
              targetPort: 9000

  ingresses:
    - id: ingress
      nameTemplate: "{{ .uid }}-ingress"
      targetNamespace: "sandbox-{{ .uid }}"
      dependIds: ["api-svc"]
      deletionPolicy: Delete
      spec:
        apiVersion: networking.k8s.io/v1
        kind: Ingress
        metadata:
          annotations:
            cert-manager.io/cluster-issuer: letsencrypt-prod
            nginx.ingress.kubernetes.io/proxy-read-timeout: "30"
        spec:
          ingressClassName: nginx
          tls:
            - hosts:
                - "{{ .uid }}.sandbox.example.com"
              secretName: "{{ .uid }}-tls"
          rules:
            - host: "{{ .uid }}.sandbox.example.com"
              http:
                paths:
                  - path: /
                    pathType: Prefix
                    backend:
                      service:
                        name: "{{ .uid }}-api"
                        port:
                          number: 80
```

## Account Lifecycle

```sql
-- New signup → sandbox provisioned automatically
INSERT INTO developer_accounts (account_id, plan_type, region, owner_email)
VALUES ('acct_1234abc', 'starter', 'us-east-1', 'dev@example.com');

-- Plan upgrade → api_replicas and rate_limit_rps update on next sync
UPDATE developer_accounts
SET plan_type = 'growth'
WHERE account_id = 'acct_1234abc';

-- Account closes → namespace and all resources deleted
UPDATE developer_accounts
SET is_active = FALSE, closed_at = NOW()
WHERE account_id = 'acct_1234abc';
```

When `plan_type` changes, the view re-computes `api_replicas` and `rate_limit_rps`. On the next hub sync, Lynq updates the Deployment and ConfigMap. Kubernetes rolls out new pods automatically.

## Verify It Works

```bash
# Sandbox provisioned for a new account
kubectl get lynqnodes -n lynq-system -l lynq.sh/hub=sandbox-hub
# NAME                       READY   DESIRED   AGE
# acct_1234abc-sandbox-stack True    5/5       2m

# All resources in the account's namespace
kubectl get all -n sandbox-acct_1234abc
# pod/acct_1234abc-api-...           Running
# pod/acct_1234abc-webhook-sink-...  Running
# deployment/acct_1234abc-api
# deployment/acct_1234abc-webhook-sink
# service/acct_1234abc-api
# service/acct_1234abc-webhook-sink
# ingress/acct_1234abc-ingress  → acct_1234abc.sandbox.example.com

# After plan upgrade: replica count updated
kubectl get deployment acct_1234abc-api -n sandbox-acct_1234abc \
  -o jsonpath='{.spec.replicas}'
# 2  ← growth plan = 2 replicas
```

## Caveats

- **The database view defines who has a sandbox.** If `closed_at IS NOT NULL` or `is_active = FALSE`, the account is excluded from the view and its sandbox is deleted. Make sure your application writes these fields consistently.
- **Plan upgrades have a ~1-minute lag** (hub sync interval) before the Deployment reflects new replica counts. This is usually acceptable for plan tier changes, but reduce `syncInterval` if you need faster propagation.
- **cert-manager must be installed** for TLS to work. Without it, ingresses are created without valid certificates. See [Custom Domain Provisioning](./use-case-custom-domains.md) for cert-manager setup.

## See Also

- [Per-PR Preview Environments](./use-case-preview-environments.md) — short-lived environments driven by CI, same isolation pattern
- [Feature Flags](./use-case-feature-flags.md) — add per-account feature toggles on top of this pattern
- [Policies](./policies.md) — `deletionPolicy`, `creationPolicy`, `conflictPolicy`
- [Datasource Configuration](./datasource.md) — using a database view as the hub source
