# Dynamic Feature Flags

::: info Multi-Tenancy Example
This guide uses **Multi-Tenancy** (SaaS application with multiple customers) as an example, which is the most common use case for Lynq. The pattern shown here can be adapted for any database-driven infrastructure automation scenario.
:::

## Overview

Enable/disable features per node using environment variables and separate templates for optional components.

This pattern enables:
- **A/B Testing**: Test new features with subset of users
- **Gradual Rollout**: Enable features progressively
- **Plan-Based Features**: Different features for different subscription tiers
- **Cost Control**: Expensive features (GPU, etc.) only for paying customers
- **Rapid Iteration**: Enable/disable features without redeploying

## Patterns

There are two main patterns for implementing feature flags:

### Pattern 1: Application-Level Feature Flags
Pass flags as environment variables, application handles the logic. Best for lightweight features.

### Pattern 2: Infrastructure-Level Feature Flags
Use database views and separate templates for expensive features (GPU, workers, etc.).

## Database Schema

```sql
CREATE TABLE nodes (
  node_id VARCHAR(63) PRIMARY KEY,
  domain VARCHAR(255) NOT NULL,
  is_active BOOLEAN DEFAULT TRUE,

  -- Feature flags (boolean features)
  feature_analytics BOOLEAN DEFAULT FALSE,
  feature_ai_assistant BOOLEAN DEFAULT FALSE,
  feature_advanced_reports BOOLEAN DEFAULT FALSE,
  feature_sso BOOLEAN DEFAULT FALSE,
  feature_audit_logs BOOLEAN DEFAULT TRUE,
  feature_webhooks BOOLEAN DEFAULT FALSE,

  -- Feature configuration (JSON for complex settings)
  feature_config JSON,

  plan_type VARCHAR(20) DEFAULT 'basic'
);

-- Example feature_config JSON:
-- {
--   "rate_limits": {"requests_per_minute": 100},
--   "storage_quota_gb": 50,
--   "max_users": 25,
--   "custom_branding": {"logo_url": "https://...", "primary_color": "#ff6600"}
-- }
```

## Pattern 1: Application-Level Feature Flags

Pass feature flags as environment variables and let the application enable/disable features:

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: base-app
  namespace: lynq-system
spec:
  hubId: production-nodes

  deployments:
    - id: main-app
      nameTemplate: "{{ .uid }}-app"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          labels:
            app: "{{ .uid }}"
            node-id: "{{ .uid }}"
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

                    # Feature flags as environment variables
                    - name: FEATURE_ANALYTICS
                      value: "{{ .featureAnalytics }}"
                    - name: FEATURE_SSO
                      value: "{{ .featureSso }}"
                    - name: FEATURE_AUDIT_LOGS
                      value: "{{ .featureAuditLogs }}"
                    - name: FEATURE_ADVANCED_REPORTS
                      value: "{{ .featureAdvancedReports }}"

                    # Complex feature config as JSON
                    - name: FEATURE_CONFIG
                      value: "{{ .featureConfig | toJson }}"
                  ports:
                    - containerPort: 8080
                  resources:
                    requests:
                      cpu: 500m
                      memory: 1Gi

  services:
    - id: app-svc
      nameTemplate: "{{ .uid }}-app"
      dependIds: ["main-app"]
      spec:
        apiVersion: v1
        kind: Service
        metadata:
          labels:
            app: "{{ .uid }}"
            node-id: "{{ .uid }}"
        spec:
          selector:
            app: "{{ .uid }}"
          ports:
            - port: 80
              targetPort: 8080
```

**Benefits:**
- Simple and efficient
- All nodes get same deployment
- Features enabled/disabled at application level
- No infrastructure changes needed

**Use for:** Lightweight features that don't require additional infrastructure

## Pattern 2: Separate Templates for Optional Features

For expensive features (like AI assistants with GPU), use database views and separate templates:

### Database Views

```sql
-- Base view for all active nodes
CREATE OR REPLACE VIEW nodes_active AS
SELECT * FROM nodes WHERE is_active = TRUE;

-- View for nodes with AI assistant enabled
CREATE OR REPLACE VIEW nodes_with_ai AS
SELECT * FROM nodes WHERE is_active = TRUE AND feature_ai_assistant = TRUE;

-- View for nodes with webhook workers
CREATE OR REPLACE VIEW nodes_with_webhooks AS
SELECT * FROM nodes WHERE is_active = TRUE AND feature_webhooks = TRUE;
```

### LynqHub for AI Assistant

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
      host: mysql.database.svc.cluster.local
      port: 3306
      database: nodes_db
      username: node_reader
      passwordRef:
        name: mysql-credentials
        key: password
      table: nodes_with_ai  # View that filters AI-enabled nodes

  valueMappings:
    uid: node_id
    # DEPRECATED v1.1.11+: Use extraValueMappings instead
    #     hostOrUrl: domain
    activate: is_active

  extraValueMappings:
    featureConfig: feature_config
```

### LynqForm for AI Assistant

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: ai-assistant
  namespace: lynq-system
spec:
  hubId: nodes-with-ai  # References filtered registry

  deployments:
    - id: ai-assistant
      nameTemplate: "{{ .uid }}-ai"
      waitForReady: true
      spec:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          labels:
            app: "{{ .uid }}-ai"
            component: ai-assistant
            node-id: "{{ .uid }}"
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: "{{ .uid }}-ai"
              component: ai-assistant
          template:
            metadata:
              labels:
                app: "{{ .uid }}-ai"
                component: ai-assistant
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
                      name: http
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
    - id: ai-service
      nameTemplate: "{{ .uid }}-ai"
      dependIds: ["ai-assistant"]
      spec:
        apiVersion: v1
        kind: Service
        metadata:
          labels:
            app: "{{ .uid }}-ai"
            component: ai-assistant
            node-id: "{{ .uid }}"
        spec:
          selector:
            app: "{{ .uid }}-ai"
          ports:
            - port: 80
              targetPort: http
```

**Benefits:**
- Resource efficiency: Only nodes with enabled features consume resources
- Cost optimization: GPU/expensive resources only allocated when needed
- Automatic cleanup: Disabling feature in DB automatically removes infrastructure
- Independent scaling: Feature-specific resources scaled separately

**Use for:** Expensive features requiring dedicated infrastructure (GPU, workers, etc.)

## Feature Rollout Workflow

### Enable Application-Level Feature (Pattern 1)

```sql
-- Enable SSO for premium customer
UPDATE nodes
SET feature_sso = TRUE,
    feature_config = JSON_SET(feature_config, '$.sso_provider', 'okta')
WHERE node_id = 'acme-corp';
```

Lynq:
1. Updates main-app Deployment with new environment variables
2. Kubernetes rolls out the updated pods automatically
3. Application detects `FEATURE_SSO=true` and enables SSO

### Enable Infrastructure-Level Feature (Pattern 2)

```sql
-- Enable AI assistant for premium customer
UPDATE nodes
SET feature_ai_assistant = TRUE,
    feature_config = JSON_SET(feature_config, '$.ai_model', 'gpt-4')
WHERE node_id = 'acme-corp';
```

Since the database view `nodes_with_ai` filters on `feature_ai_assistant = TRUE`:
1. Hub nodes-with-ai syncs and detects new node
2. Creates LynqNode CR `acme-corp-ai-assistant`
3. Deploys AI assistant Deployment + Service with GPU
4. Marks LynqNode as Ready once all resources are up

### Gradual Rollout by Plan

```sql
-- Enable advanced reports for all pro/enterprise customers
UPDATE nodes
SET feature_advanced_reports = TRUE
WHERE plan_type IN ('pro', 'enterprise');
```

### Feature Flag A/B Testing

```sql
-- Enable webhooks for 10% of users (random sampling)
UPDATE nodes
SET feature_webhooks = TRUE
WHERE RAND() < 0.1 AND plan_type = 'pro';
```

This triggers creation of webhook worker deployments only for those 10% of nodes.

## Automatic Cleanup on Feature Disable

### Pattern 1 (Application-Level)

```sql
UPDATE nodes SET feature_sso = FALSE WHERE node_id = 'acme-corp';
```

- Environment variable updated in Deployment
- Kubernetes rolls out updated pods
- No resource deletion needed

### Pattern 2 (Infrastructure-Level)

```sql
UPDATE nodes SET feature_ai_assistant = FALSE WHERE node_id = 'acme-corp';
```

- Node `acme-corp` no longer appears in `nodes_with_ai` view
- Hub nodes-with-ai syncs and detects node removal
- Lynq deletes LynqNode CR `acme-corp-ai-assistant`
- AI assistant Deployment + Service automatically garbage collected
- GPU resources freed

## Verification: Feature Flag Change Propagation

Verify how feature flag changes propagate through the system:

### Pattern 1: Application-Level Feature Change

```sql
-- Enable SSO for acme-corp
UPDATE nodes SET feature_sso = TRUE WHERE node_id = 'acme-corp';
```

```bash
# Step 1: Verify database change
mysql -e "SELECT node_id, feature_sso FROM nodes WHERE node_id='acme-corp'"
# Expected: feature_sso = 1

# Step 2: Wait for Lynq sync (default: 1 minute)
# Or check LynqNode reconciliation
kubectl get lynqnode acme-corp-base-app -o jsonpath='{.metadata.annotations.lynq\.sh/last-sync-time}'

# Step 3: Check Deployment was updated
kubectl get deployment acme-corp-app -o jsonpath='{.spec.template.spec.containers[0].env}' | jq '.[] | select(.name=="FEATURE_SSO")'
# Expected: {"name":"FEATURE_SSO","value":"true"}

# Step 4: Verify new pods rolled out
kubectl rollout status deployment/acme-corp-app
# Expected: deployment "acme-corp-app" successfully rolled out

# Step 5: Verify application received new flag
kubectl exec deployment/acme-corp-app -- env | grep FEATURE_SSO
# Expected: FEATURE_SSO=true
```

**Timeline:**

| Time | Event |
|------|-------|
| T+0 | Database updated: `feature_sso = TRUE` |
| T+1min | Lynq Hub syncs, detects change |
| T+1min | LynqNode reconciled, Deployment updated |
| T+2min | Kubernetes rolls out new pods |
| T+3min | New pods running with `FEATURE_SSO=true` |

### Pattern 2: Infrastructure-Level Feature Change

```sql
-- Enable AI assistant for acme-corp
UPDATE nodes SET feature_ai_assistant = TRUE WHERE node_id = 'acme-corp';
```

```bash
# Step 1: Verify database view includes node
mysql -e "SELECT node_id FROM nodes_with_ai WHERE node_id='acme-corp'"
# Expected: acme-corp

# Step 2: Wait for Lynq sync
# Hub "nodes-with-ai" detects new row

# Step 3: Verify new LynqNode created
kubectl get lynqnode | grep acme-corp-ai
# Expected output:
# NAME                    READY   DESIRED   FAILED   AGE
# acme-corp-ai-assistant  2/2     2         0        2m

# Step 4: Verify AI deployment created
kubectl get deployment acme-corp-ai
# Expected: 1/1 Ready

# Step 5: Verify GPU allocated (if applicable)
kubectl describe pod -l app=acme-corp-ai | grep -A 5 "Limits:"
# Expected: nvidia.com/gpu: 1
```

### Feature Disable Verification

```sql
-- Disable AI assistant
UPDATE nodes SET feature_ai_assistant = FALSE WHERE node_id = 'acme-corp';
```

```bash
# Step 1: Node removed from view
mysql -e "SELECT COUNT(*) FROM nodes_with_ai WHERE node_id='acme-corp'"
# Expected: 0

# Step 2: LynqNode marked for deletion
kubectl get lynqnode acme-corp-ai-assistant -o jsonpath='{.metadata.deletionTimestamp}'
# Expected: <timestamp> (deletion in progress)

# Step 3: Resources cleaned up
kubectl get deployment acme-corp-ai 2>/dev/null || echo "Deployment deleted (expected)"
# Expected: Deployment deleted (expected)

# Step 4: Verify GPU released
kubectl describe nodes | grep "nvidia.com/gpu"
# GPU should be available again
```

### Complete Feature Flag Verification Script

```bash
#!/bin/bash
# verify-feature-flag.sh <node-id> <feature-name>

NODE_ID=$1
FEATURE=$2

echo "=== Feature Flag Verification for ${NODE_ID} ==="
echo "Feature: ${FEATURE}"
echo ""

# Check database
echo "1. Database State:"
mysql -N -e "SELECT feature_${FEATURE} FROM nodes WHERE node_id='${NODE_ID}'"

# Check environment variable (Pattern 1)
echo "2. Environment Variable (if app-level):"
kubectl exec deployment/${NODE_ID}-app -- env 2>/dev/null | grep -i "FEATURE_${FEATURE^^}" || echo "N/A"

# Check dedicated LynqNode (Pattern 2)
echo "3. Dedicated LynqNode (if infra-level):"
kubectl get lynqnode ${NODE_ID}-${FEATURE} -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "N/A"

# Check dedicated deployment
echo "4. Dedicated Deployment (if infra-level):"
kubectl get deployment ${NODE_ID}-${FEATURE} -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "N/A"

echo ""
echo "=== Verification Complete ==="
```

## Benefits

1. **Flexibility**: Enable/disable features without code deployment
2. **A/B Testing**: Easy to test features with subset of users
3. **Cost Control**: Expensive features only for paying customers
4. **Gradual Rollout**: Roll out features progressively
5. **Plan-Based Features**: Different features for different tiers

## Best Practices

1. **Default Values**: Set sensible defaults for feature flags
2. **Documentation**: Document what each feature flag controls
3. **Monitoring**: Track feature usage and performance
4. **Testing**: Test feature combinations thoroughly
5. **Cleanup**: Implement proper cleanup when features are disabled

## Related Documentation

- [Templates Guide](/templates) - Template functions and conditionals
- [Policies](/policies) - Resource lifecycle management
- [Monitoring](/monitoring) - Feature usage tracking
- [Advanced Use Cases](/advanced-use-cases) - Other patterns

## Next Steps

- Design feature flag schema
- Implement application-level feature detection
- Set up monitoring for feature usage
- Create feature rollout playbook
