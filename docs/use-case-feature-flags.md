---
description: "Database boolean columns as infrastructure switches — flip a column to provision or remove Kubernetes resources per node."
---

# Feature Flags

Enabling a feature for 50 customers the traditional way means 50 Helm upgrades or 50 kubectl edits. With Lynq, a feature flag is a database column. Update the column and the infrastructure follows — resources appear for nodes where the flag is on, disappear when it's off.

::: tip Time to working
~5 minutes to configure. Flag changes propagate within the hub's `syncInterval` (default: 30 seconds).
:::

## How It Works

- **Application-level flags**: feature values are passed as environment variables. The Deployment is the same for all nodes; the application enables or disables features based on env vars.
- **Infrastructure-level flags**: a separate LynqHub reads from a filtered database view. Nodes where the flag is on get additional Kubernetes resources (GPU workload, background worker, etc.). Nodes where it's off have no such resources at all.

::: tip Which pattern should I use?
- **Application-level** — the feature is code-only (toggle a UI element, enable an API endpoint, change a rate limit). No dedicated infrastructure needed.
- **Infrastructure-level** — the feature requires dedicated compute (GPU pod, webhook worker, cache cluster). Cost and resource usage should be zero when the feature is off.
:::

## Database Schema

```sql
CREATE TABLE nodes (
  node_id  VARCHAR(63) PRIMARY KEY,
  is_active BOOLEAN DEFAULT TRUE,

  -- Application-level flags (passed as env vars)
  feature_sso              BOOLEAN DEFAULT FALSE,
  feature_analytics        BOOLEAN DEFAULT FALSE,
  feature_advanced_reports BOOLEAN DEFAULT FALSE,
  feature_audit_logs       BOOLEAN DEFAULT TRUE,
  feature_webhooks         BOOLEAN DEFAULT FALSE,

  -- Infrastructure-level flag (controls whether a separate worker is provisioned)
  feature_ai_assistant     BOOLEAN DEFAULT FALSE,

  -- JSON for complex configuration values
  feature_config JSON
);
```

## Pattern 1: Application-Level Flags

One LynqForm, one Deployment per node. Feature flags arrive as environment variables.

### LynqHub

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: production-nodes
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
    featureSso: feature_sso
    featureAnalytics: feature_analytics
    featureAdvancedReports: feature_advanced_reports
    featureAuditLogs: feature_audit_logs
    featureWebhooks: feature_webhooks
    featureConfig: feature_config
```

### LynqForm

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: base-app
  namespace: lynq-system
spec:
  hubId: production-nodes

  deployments:
    - id: app
      nameTemplate: "{{ .uid }}-app"
      waitForReady: true
      spec:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          labels:
            app: "{{ .uid }}"
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
                  image: registry.example.com/node-app:v1.5.0
                  env:
                    - name: NODE_ID
                      value: "{{ .uid }}"
                    - name: FEATURE_SSO
                      value: "{{ .featureSso }}"
                    - name: FEATURE_ANALYTICS
                      value: "{{ .featureAnalytics }}"
                    - name: FEATURE_ADVANCED_REPORTS
                      value: "{{ .featureAdvancedReports }}"
                    - name: FEATURE_AUDIT_LOGS
                      value: "{{ .featureAuditLogs }}"
                    - name: FEATURE_WEBHOOKS
                      value: "{{ .featureWebhooks }}"
                    - name: FEATURE_CONFIG
                      value: "{{ .featureConfig | toJson }}"
                  ports:
                    - containerPort: 8080
                  resources:
                    requests:
                      cpu: 500m
                      memory: 1Gi

  services:
    - id: svc
      nameTemplate: "{{ .uid }}-app"
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
```

### Enabling a flag

```sql
-- Enable SSO for one node
UPDATE nodes SET feature_sso = TRUE WHERE node_id = 'acme-corp';

-- Gradual rollout: enable advanced reports for all pro+ customers
UPDATE nodes SET feature_advanced_reports = TRUE
WHERE plan_type IN ('pro', 'enterprise');
```

Lynq updates the Deployment env vars on the next sync. Kubernetes rolls out new pods.

## Pattern 2: Infrastructure-Level Flags

A separate LynqHub reads from a filtered view. Nodes where the flag is on have dedicated infrastructure; nodes where it's off have none.

### Database View

```sql
-- Only includes nodes where the AI assistant is enabled
CREATE VIEW nodes_with_ai AS
SELECT * FROM nodes
WHERE is_active = TRUE AND feature_ai_assistant = TRUE;
```

### LynqHub for AI Workloads

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: nodes-with-ai
  namespace: lynq-system
spec:
  source:
    type: mysql
    syncInterval: 1m
    mysql:
      host: mysql.internal.svc.cluster.local
      port: 3306
      database: nodes_db
      table: nodes_with_ai   # filtered view
      username: lynq_reader
      passwordRef:
        name: mysql-credentials
        key: password
  valueMappings:
    uid: node_id
    activate: is_active
  extraValueMappings:
    featureConfig: feature_config
```

### LynqForm for AI Workloads

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: ai-assistant
  namespace: lynq-system
spec:
  hubId: nodes-with-ai

  deployments:
    - id: ai-assistant
      nameTemplate: "{{ .uid }}-ai"
      waitForReady: true
      timeoutSeconds: 300
      spec:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          labels:
            app: "{{ .uid }}-ai"
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: "{{ .uid }}-ai"
          template:
            metadata:
              labels:
                app: "{{ .uid }}-ai"
            spec:
              containers:
                - name: ai-assistant
                  image: registry.example.com/ai-assistant:v2.0.0
                  env:
                    - name: NODE_ID
                      value: "{{ .uid }}"
                    - name: OPENAI_API_KEY
                      valueFrom:
                        secretKeyRef:
                          name: openai-credentials
                          key: api-key
                  ports:
                    - containerPort: 8080
                  resources:
                    requests:
                      cpu: 1000m
                      memory: 2Gi
                      nvidia.com/gpu: "1"
                    limits:
                      cpu: 2000m
                      memory: 4Gi
                      nvidia.com/gpu: "1"

  services:
    - id: svc
      nameTemplate: "{{ .uid }}-ai"
      dependIds: ["ai-assistant"]
      spec:
        apiVersion: v1
        kind: Service
        spec:
          selector:
            app: "{{ .uid }}-ai"
          ports:
            - port: 80
              targetPort: 8080
```

### Enabling and disabling

```sql
-- Enable: node appears in the view → Lynq creates the AI Deployment
UPDATE nodes SET feature_ai_assistant = TRUE WHERE node_id = 'acme-corp';

-- Disable: node leaves the view → Lynq deletes the AI Deployment and frees the GPU
UPDATE nodes SET feature_ai_assistant = FALSE WHERE node_id = 'acme-corp';
```

## Verify It Works

```bash
# Pattern 1: env var reflected in running pod
kubectl exec deployment/acme-corp-app -- env | grep FEATURE_SSO
# FEATURE_SSO=true

# Pattern 2: AI LynqNode exists only for enabled nodes
kubectl get lynqnodes -n lynq-system -l lynq.sh/hub=nodes-with-ai
# NAME                       READY   DESIRED   AGE
# acme-corp-ai-assistant     True    2/2       5m
# (other nodes not listed — they have no AI resources)

# After disabling:
kubectl get lynqnode acme-corp-ai-assistant -n lynq-system
# Error from server (NotFound): ...
kubectl get deployment acme-corp-ai
# Error from server (NotFound): ...  ← GPU freed
```

## Caveats

- **Pattern 1 causes a pod restart** when a flag changes — Kubernetes rolls the Deployment when env vars change. If your app can't tolerate a brief rollout, use a ConfigMap or remote config system for runtime flags.
- **Pattern 2 has a 1-minute lag** (hub sync interval) before infrastructure appears or disappears. For immediate provisioning, reduce `syncInterval` — but watch your database query load.
- **Database views must stay in sync** with the flag column names. If you rename a flag column, update the view and the `extraValueMappings` together.

## See Also

- [Blue-Green Deployments](./use-case-blue-green.md) — column-as-switch pattern applied to traffic routing
- [Datasource Configuration](./datasource.md) — using database views as hub sources
- [Templates](./templates.md) — `toJson` and other template functions for complex config values
