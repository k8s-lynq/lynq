# Step 6: Create LynqForm with Force Policy

This is the key step! We'll create a **LynqForm** that uses `conflictPolicy: Force` to adopt existing ConfigMaps.

## Understanding ConflictPolicy: Force

When Lynq encounters an existing resource:

| Policy | Behavior |
|--------|----------|
| `Stuck` (default) | Fail with error, don't modify existing resource |
| **`Force`** | **Take ownership via SSA, update to match template** |

The `Force` policy uses Kubernetes Server-Side Apply (SSA) with `force=true`, which:
1. Takes over field ownership from the original manager
2. Updates the resource to match the template
3. Preserves any fields not managed by Lynq

## Create the LynqForm

**Important**: Notice the `conflictPolicy: Force` setting!

```bash
cat <<'EOF' | kubectl apply -f -
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: app-config-template
  namespace: lynq-demo
spec:
  hubId: config-hub

  configMaps:
    - id: config
      # The nameTemplate MUST match existing ConfigMap names!
      nameTemplate: "{{ .uid }}-config"

      # KEY SETTING: Force policy to adopt existing resources
      conflictPolicy: Force

      # Don't wait for ConfigMap "ready" (it's instant)
      waitForReady: false

      spec:
        apiVersion: v1
        kind: ConfigMap
        metadata:
          labels:
            app: "{{ .uid }}"
            team: platform
            managed-by: lynq
        data:
          # Values from database will overwrite existing ConfigMap data
          APP_NAME: "{{ .appName }}"
          DATABASE_URL: "{{ .databaseUrl }}"
          FEATURE_FLAG: "{{ .featureFlag }}"
          MAX_CONNECTIONS: "{{ .maxConnections }}"
          LOG_LEVEL: "{{ .logLevel }}"
          # Add new field showing last sync time
          LYNQ_MANAGED: "true"
          LAST_SYNC: "synced-by-lynq"
EOF
```{{exec}}

## Watch the Adoption Happen!

Watch as Lynq creates LynqNodes and adopts the existing ConfigMaps:

```bash
kubectl get lynqnodes -n lynq-demo -w
```{{exec}}

> Press `Ctrl+C` after you see all three nodes

## Verify LynqNodes Created

Check the LynqNode CRs:

```bash
kubectl get lynqnodes -n lynq-demo
```{{exec}}

## Verify ConfigMaps Were Adopted

Now check the ConfigMaps - they should have **new values from the database**:

```bash
kubectl get configmap app-alpha-config -n lynq-demo -o yaml
```{{exec}}

Key changes to observe:
1. **`DATABASE_URL`** → Now `postgres://new-db.prod:5432/alpha`
2. **`FEATURE_FLAG`** → Now `true` (was `false`)
3. **`LYNQ_MANAGED`** → New field added
4. **`ownerReferences`** → Now owned by LynqNode!

## Compare All ConfigMaps

See the updated values across all apps:

```bash
echo "=== app-alpha-config ===" && \
kubectl get configmap app-alpha-config -n lynq-demo -o jsonpath='{.data}' | jq . && \
echo -e "\n=== app-beta-config ===" && \
kubectl get configmap app-beta-config -n lynq-demo -o jsonpath='{.data}' | jq . && \
echo -e "\n=== app-gamma-config ===" && \
kubectl get configmap app-gamma-config -n lynq-demo -o jsonpath='{.data}' | jq .
```{{exec}}

## Verify Ownership Transfer

Check that Lynq now owns the ConfigMaps:

```bash
kubectl get configmap app-alpha-config -n lynq-demo -o jsonpath='{.metadata.ownerReferences[0].kind}' ; echo " (owned by LynqNode)"
```{{exec}}

**The adoption is complete!** 🎉

| Before | After |
|--------|-------|
| Unmanaged ConfigMaps | Lynq-managed ConfigMaps |
| Manual updates | Database-driven updates |
| No audit trail | Database as source of truth |

✅ **Checkpoint**:
- LynqForm created with `conflictPolicy: Force`
- Existing ConfigMaps adopted by Lynq
- Values updated from database

Click **Continue** to test database-driven updates!
