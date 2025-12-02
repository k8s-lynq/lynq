# Step 7: Test Database-Driven Updates

Now let's see the real power of Lynq - **automatic synchronization** when database values change!

## Scenario 1: Update Configuration in Database

Let's update the `max_connections` and `log_level` for `app-alpha` in the database:

```bash
kubectl exec -n lynq-demo deployment/mysql -- \
  mysql -h 127.0.0.1 -u root -prootpass123 -e \
  "UPDATE config_db.app_configs SET max_connections = 500, log_level = 'debug' WHERE app_id = 'app-alpha';"
```{{exec}}

Wait for Lynq to sync (30 seconds):

```bash
echo "Waiting for sync..." && sleep 35
```{{exec}}

Check the ConfigMap - it should reflect the new values:

```bash
kubectl get configmap app-alpha-config -n lynq-demo -o jsonpath='{.data.MAX_CONNECTIONS}' && echo " (was 100, now 500)"
kubectl get configmap app-alpha-config -n lynq-demo -o jsonpath='{.data.LOG_LEVEL}' && echo " (was info, now debug)"
```{{exec}}

**The ConfigMap was automatically updated!** 🔄

---

## Scenario 2: Add a New Application

Add a new application to the database:

```bash
kubectl exec -n lynq-demo deployment/mysql -- \
  mysql -h 127.0.0.1 -u root -prootpass123 -e \
  "INSERT INTO config_db.app_configs (app_id, app_name, database_url, feature_flag, max_connections, log_level, is_active) VALUES ('app-delta', 'Delta Application v1.0', 'postgres://new-db.prod:5432/delta', true, 150, 'info', true);"
```{{exec}}

Wait for sync and check:

```bash
sleep 35 && kubectl get configmaps -n lynq-demo -l managed-by=lynq
```{{exec}}

A new ConfigMap `app-delta-config` should appear!

```bash
kubectl get configmap app-delta-config -n lynq-demo -o jsonpath='{.data}'  | jq .
```{{exec}}

---

## Scenario 3: Deactivate an Application

Deactivate `app-gamma` (set `is_active = false`):

```bash
kubectl exec -n lynq-demo deployment/mysql -- \
  mysql -h 127.0.0.1 -u root -prootpass123 -e \
  "UPDATE config_db.app_configs SET is_active = false WHERE app_id = 'app-gamma';"
```{{exec}}

Wait and check:

```bash
sleep 35 && kubectl get lynqnodes -n lynq-demo
```{{exec}}

The `app-gamma` LynqNode should be deleted!

Check ConfigMaps:

```bash
kubectl get configmaps -n lynq-demo -l team=platform
```{{exec}}

**`app-gamma-config` is gone!** Lynq automatically cleaned up the resource.

---

## Scenario 4: Bulk Update (Database Migration)

Simulate a database server migration - update all apps to use a new database server:

```bash
kubectl exec -n lynq-demo deployment/mysql -- \
  mysql -h 127.0.0.1 -u root -prootpass123 -e \
  "UPDATE config_db.app_configs SET database_url = REPLACE(database_url, 'new-db.prod', 'db-cluster.prod');"
```{{exec}}

Wait and verify all ConfigMaps updated:

```bash
sleep 35 && \
kubectl get configmaps -n lynq-demo -l managed-by=lynq -o jsonpath='{range .items[*]}{.metadata.name}: {.data.DATABASE_URL}{"\n"}{end}'
```{{exec}}

All ConfigMaps now point to `db-cluster.prod`!

---

## Scenario 5: Manual Change Protection (Drift Correction)

Try to manually edit a ConfigMap:

```bash
kubectl patch configmap app-alpha-config -n lynq-demo \
  --type merge -p '{"data":{"LOG_LEVEL":"manual-override","HACKED":"true"}}'
```{{exec}}

Check the immediate result:

```bash
kubectl get configmap app-alpha-config -n lynq-demo -o jsonpath='{.data.LOG_LEVEL}' && echo ""
kubectl get configmap app-alpha-config -n lynq-demo -o jsonpath='{.data.HACKED}' && echo ""
```{{exec}}

Wait for Lynq's reconciliation:

```bash
sleep 35 && \
kubectl get configmap app-alpha-config -n lynq-demo -o jsonpath='{.data.LOG_LEVEL}' && echo " (reverted to database value)"
kubectl get configmap app-alpha-config -n lynq-demo -o jsonpath='{.data.HACKED}' && echo " (removed - not in template)"
```{{exec}}

**Lynq automatically corrects drift!** The database remains the source of truth.

---

## View Final State

Current database state:

```bash
kubectl exec -n lynq-demo deployment/mysql -- \
  mysql -h 127.0.0.1 -u root -prootpass123 -e \
  "SELECT app_id, app_name, is_active FROM config_db.app_configs;"
```{{exec}}

Current LynqNodes:

```bash
kubectl get lynqnodes -n lynq-demo
```{{exec}}

Current ConfigMaps:

```bash
kubectl get configmaps -n lynq-demo -l managed-by=lynq
```{{exec}}

✅ **Checkpoint**: You've tested:
- Configuration updates from database
- Adding new applications
- Deactivating applications (automatic cleanup)
- Bulk updates (database migration)
- Drift correction (manual change protection)

Click **Continue** to see what you've learned!
