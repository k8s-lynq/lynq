# Step 2: Create Existing Resources

Let's simulate a common scenario: you already have ConfigMaps deployed that contain application configuration. These were created manually or by another automation tool.

## Create Existing ConfigMaps

These ConfigMaps represent your current production configuration, manually managed until now:

```bash
cat <<'EOF' | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-alpha-config
  namespace: lynq-demo
  labels:
    app: app-alpha
    team: platform
data:
  APP_NAME: "Alpha Application"
  DATABASE_URL: "postgres://old-db:5432/alpha"
  FEATURE_FLAG: "false"
  MAX_CONNECTIONS: "50"
  LOG_LEVEL: "warn"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-beta-config
  namespace: lynq-demo
  labels:
    app: app-beta
    team: platform
data:
  APP_NAME: "Beta Application"
  DATABASE_URL: "postgres://old-db:5432/beta"
  FEATURE_FLAG: "false"
  MAX_CONNECTIONS: "100"
  LOG_LEVEL: "info"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-gamma-config
  namespace: lynq-demo
  labels:
    app: app-gamma
    team: platform
data:
  APP_NAME: "Gamma Application"
  DATABASE_URL: "postgres://old-db:5432/gamma"
  FEATURE_FLAG: "true"
  MAX_CONNECTIONS: "25"
  LOG_LEVEL: "debug"
EOF
```{{exec}}

## Verify Existing Resources

Check that the ConfigMaps were created:

```bash
kubectl get configmaps -n lynq-demo -l team=platform
```{{exec}}

View the current configuration values:

```bash
kubectl get configmap app-alpha-config -n lynq-demo -o yaml
```{{exec}}

## Note the Current State

These ConfigMaps have:
- ❌ No central source of truth
- ❌ No audit trail for changes
- ❌ Manual update process required
- ❌ Risk of configuration drift

**Key observation**: Look at the `metadata` - there's no `ownerReferences`. These resources are **unmanaged**.

```bash
kubectl get configmap app-alpha-config -n lynq-demo -o jsonpath='{.metadata.ownerReferences}' ; echo "[empty = unmanaged]"
```{{exec}}

## Current Pain Points

Imagine you need to update `DATABASE_URL` for all apps to point to a new database server:

1. 😫 Edit each ConfigMap manually
2. 😫 Restart each application to pick up changes
3. 😫 Hope you didn't miss any ConfigMap
4. 😫 No record of who changed what and when

**Lynq will solve this** by making the database the single source of truth!

✅ **Checkpoint**: You now have 3 existing ConfigMaps that we'll adopt with Lynq:
- `app-alpha-config`
- `app-beta-config`
- `app-gamma-config`

Click **Continue** to set up the MySQL database as the central configuration store.
