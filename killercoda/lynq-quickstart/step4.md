# Step 4: Create LynqHub

Now let's create a **LynqHub** that connects to our MySQL database and syncs tenant information.

## Understanding LynqHub

A LynqHub:
- Connects to an external database (MySQL)
- Periodically syncs data (every 30 seconds by default)
- Maps database columns to template variables
- Creates LynqNode CRs for each active row

## Create the LynqHub

```bash
cat <<'EOF' | kubectl apply -f -
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: tenant-hub
  namespace: lynq-demo
spec:
  source:
    type: mysql
    syncInterval: 30s
    mysql:
      host: mysql.lynq-demo.svc.cluster.local
      port: 3306
      username: lynq_reader
      passwordRef:
        name: mysql-credentials
        key: password
      database: tenants
      table: tenant_configs

  # Map database columns to template variables
  valueMappings:
    uid: tenant_id        # Required: unique identifier
    activate: is_active   # Required: activation flag

  # Additional column mappings
  extraValueMappings:
    tenantUrl: tenant_url
    plan: plan
EOF
```{{exec}}

## Verify LynqHub Status

Check that the hub is syncing:

```bash
kubectl get lynqhub tenant-hub -n lynq-demo
```{{exec}}

View detailed status:

```bash
kubectl describe lynqhub tenant-hub -n lynq-demo
```{{exec}}

## Understanding the Column Mappings

| Database Column | Template Variable | Description |
|-----------------|-------------------|-------------|
| `tenant_id` | `.uid` | Unique identifier (required) |
| `is_active` | `.activate` | Whether to provision (required) |
| `tenant_url` | `.tenantUrl` | Custom: tenant's URL |
| `plan` | `.plan` | Custom: subscription plan |

These variables will be available in your LynqForm templates!

✅ **Checkpoint**: LynqHub is created and connected to MySQL.

> **Note**: No LynqNodes are created yet because we haven't defined a template (LynqForm).

Click **Continue** to create the template.
