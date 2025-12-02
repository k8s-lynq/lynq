# Step 5: Create LynqHub

Now let's create a **LynqHub** that connects to our MySQL database and syncs the configuration data.

## Understanding LynqHub

A LynqHub:
- Connects to an external database (MySQL, PostgreSQL, etc.)
- Periodically syncs data (configurable interval)
- Maps database columns to template variables
- Creates LynqNode CRs for each active row

## Create the LynqHub

```bash
cat <<'EOF' | kubectl apply -f -
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: config-hub
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
      database: config_db
      table: app_configs

  # Map database columns to template variables
  valueMappings:
    uid: app_id           # Required: unique identifier
    activate: is_active   # Required: activation flag

  # Additional column mappings for use in templates
  extraValueMappings:
    appName: app_name
    databaseUrl: database_url
    featureFlag: feature_flag
    maxConnections: max_connections
    logLevel: log_level
EOF
```{{exec}}

## Verify LynqHub Status

Check that the hub is syncing:

```bash
kubectl get lynqhub config-hub -n lynq-demo
```{{exec}}

View detailed status:

```bash
kubectl describe lynqhub config-hub -n lynq-demo
```{{exec}}

## Understanding the Column Mappings

| Database Column | Template Variable | Description |
|-----------------|-------------------|-------------|
| `app_id` | `.uid` | Unique identifier (required) |
| `is_active` | `.activate` | Whether to manage this app (required) |
| `app_name` | `.appName` | Application display name |
| `database_url` | `.databaseUrl` | Database connection string |
| `feature_flag` | `.featureFlag` | Feature toggle |
| `max_connections` | `.maxConnections` | Connection pool size |
| `log_level` | `.logLevel` | Logging verbosity |

These variables will be used in our LynqForm template!

✅ **Checkpoint**: LynqHub is created and connected to MySQL.

> **Note**: No LynqNodes are created yet because we haven't defined a template (LynqForm).

Click **Continue** to create the LynqForm with the Force policy.
