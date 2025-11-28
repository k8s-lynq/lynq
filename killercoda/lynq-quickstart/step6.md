# Step 6: Test Tenant Lifecycle

Let's see how Lynq automatically responds to database changes!

## Scenario 1: Add a New Tenant

Insert a new tenant into the database:

```bash
kubectl exec -n lynq-demo deployment/mysql -- \
  mysql -h 127.0.0.1 -u root -prootpass123 -e \
  "INSERT INTO tenants.tenant_configs (tenant_id, tenant_url, is_active, plan) VALUES ('delta-co', 'https://delta.example.com', true, 'pro');"
```{{exec}}

Wait about 30 seconds (sync interval), then check:

```bash
sleep 35 && kubectl get lynqnodes -n lynq-demo
```{{exec}}

You should see a new `delta-co-webapp-template` LynqNode!

Verify the resources:

```bash
kubectl get configmaps,deployments,services -n lynq-demo -l app=delta-co
```{{exec}}

---

## Scenario 2: Deactivate a Tenant

Let's deactivate `beta-inc`:

```bash
kubectl exec -n lynq-demo deployment/mysql -- \
  mysql -h 127.0.0.1 -u root -prootpass123 -e \
  "UPDATE tenants.tenant_configs SET is_active = false WHERE tenant_id = 'beta-inc';"
```{{exec}}

Wait for sync and check:

```bash
sleep 35 && kubectl get lynqnodes -n lynq-demo
```{{exec}}

`beta-inc` should be gone! Verify resources are cleaned up:

```bash
kubectl get all -n lynq-demo -l app=beta-inc
```{{exec}}

**No resources found** - Lynq automatically cleaned up!

---

## Scenario 3: Reactivate a Tenant

Reactivate the previously inactive `gamma-ltd`:

```bash
kubectl exec -n lynq-demo deployment/mysql -- \
  mysql -h 127.0.0.1 -u root -prootpass123 -e \
  "UPDATE tenants.tenant_configs SET is_active = true WHERE tenant_id = 'gamma-ltd';"
```{{exec}}

Wait and verify:

```bash
sleep 35 && kubectl get lynqnodes,deployments -n lynq-demo
```{{exec}}

`gamma-ltd` is now provisioned!

---

## View Final State

See all active tenants:

```bash
kubectl exec -n lynq-demo deployment/mysql -- \
  mysql -h 127.0.0.1 -u root -prootpass123 -e "SELECT tenant_id, is_active, plan FROM tenants.tenant_configs;"
```{{exec}}

See all LynqNodes:

```bash
kubectl get lynqnodes -n lynq-demo -o wide
```{{exec}}

See all tenant resources:

```bash
kubectl get configmaps,deployments,services -n lynq-demo -l lynq.sh/tenant
```{{exec}}

✅ **Checkpoint**: You've successfully tested:
- Adding new tenants
- Deactivating tenants (automatic cleanup)
- Reactivating tenants

Click **Continue** to see what you've learned!
