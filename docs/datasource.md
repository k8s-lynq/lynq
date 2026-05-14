---
description: "Connect Lynq to MySQL. Covers connection setup, credential management, column mappings, and extraValueMappings."
---

# Datasource Configuration

LynqHub polls your database at a configured interval, reads active rows, and reconciles the corresponding LynqNode CRs. This page covers connecting to MySQL, mapping columns to template variables, and handling common schema shapes.

## Supported Datasources

| Datasource | Status | Since |
|------------|--------|-------|
| MySQL | Stable | v1.0 |
| PostgreSQL | Planned | v1.2 |
| Custom | [Contribute](contributing-datasource.md) | — |

## MySQL Connection

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: my-hub
  namespace: lynq-system
spec:
  source:
    type: mysql
    mysql:
      host: mysql.default.svc.cluster.local
      port: 3306
      username: node_reader
      passwordRef:
        name: mysql-credentials
        key: password
      database: nodes
      table: node_configs
    syncInterval: 1m
```

| Field | Description | Default |
|-------|-------------|---------|
| `host` | MySQL hostname or IP | — |
| `port` | MySQL port | `3306` |
| `username` | Database user (use read-only) | — |
| `passwordRef` | Secret containing the password | — |
| `database` | Database name | — |
| `table` | Table or view name | — |
| `syncInterval` | Poll frequency (`30s`, `1m`, `5m`) | `1m` |

**Kubernetes Secret:**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mysql-credentials
  namespace: lynq-system
type: Opaque
stringData:
  password: your-password-here
```

## Column Mappings

### Required

Every LynqHub needs exactly two required mappings:

```yaml
valueMappings:
  uid: node_id       # unique node identifier
  activate: is_active  # activation flag
```

**`uid`** — Unique string identifier per row. Used in resource naming and labels.

**`activate`** — Controls whether a row is provisioned. Accepted truthy values: `1`, `true`, `TRUE`, `True`, `yes`, `YES`, `Yes`. Everything else (including `NULL`) is treated as inactive.

### Extra Mappings

Any additional columns can be mapped to template variables:

```yaml
extraValueMappings:
  planId: subscription_plan   # column → template variable
  region: deployment_region
  maxUsers: max_user_count
```

::: v-pre
These become available in all templates as `{{ .planId }}`, `{{ .region }}`, etc.
:::

## Schema Examples

### Simple table

```sql
CREATE TABLE node_configs (
    node_id         VARCHAR(255) PRIMARY KEY,
    is_active       TINYINT(1)  DEFAULT 0,
    subscription_plan VARCHAR(50),
    deployment_region VARCHAR(50)
);
```

```yaml
valueMappings:
  uid: node_id
  activate: is_active
extraValueMappings:
  planId: subscription_plan
  region: deployment_region
```

### TINYINT activate column

MySQL `TINYINT(1)` returns the string `"1"` or `"0"` when queried — both are valid activate values.

```sql
CREATE TABLE nodes (
    id      VARCHAR(255) PRIMARY KEY,
    enabled TINYINT(1) DEFAULT 0
);
```

```yaml
valueMappings:
  uid: id
  activate: enabled  # "1" = active, "0" = inactive
```

### Status string (requires a VIEW)

If your activate column holds strings like `"active"` / `"inactive"`, those aren't directly valid. Create a MySQL VIEW to transform them:

```sql
CREATE VIEW node_configs AS
SELECT
    id           AS node_id,
    CASE status
        WHEN 'active' THEN '1'
        ELSE '0'
    END          AS is_active,
    plan         AS subscription_plan
FROM nodes;
```

Then point the hub at the view name instead of the raw table. See [Datasource Views](datasource-views.md) for more VIEW patterns.

## Best Practices

**Use a read-only database user.** Lynq only needs SELECT on the target table/view.

```sql
CREATE USER 'node_reader'@'%' IDENTIFIED BY 'secure_password';
GRANT SELECT ON nodes.node_configs TO 'node_reader'@'%';
FLUSH PRIVILEGES;
```

**Use a VIEW to isolate sensitive columns.** Grant SELECT only on the view, not the underlying table.

**Index the activate column** for faster filtering at scale:

```sql
CREATE INDEX idx_active ON nodes(is_active);
```

**Set syncInterval based on scale:**

```yaml
syncInterval: 30s  # development
syncInterval: 1m   # production (recommended)
syncInterval: 5m   # large deployments (1000+ nodes)
```

## Troubleshooting

**Nodes not created — check activate values:**

```sql
SELECT node_id, is_active FROM node_configs
WHERE is_active NOT IN ('0', '1', 'true', 'false', 'yes', 'no', 'TRUE', 'FALSE');
-- Any rows here have invalid activate values
```

**Check operator logs for sync errors:**

```bash
kubectl logs -n lynq-system -l control-plane=controller-manager | grep -i "hub\|sync\|query"
```

**Connection errors — test from inside the cluster:**

```bash
kubectl run mysql-test --rm -it --image=mysql:8 -- \
  mysql -h mysql.default.svc.cluster.local -u node_reader -p
```

**Verify the Secret:**

```bash
kubectl get secret mysql-credentials -n lynq-system -o jsonpath='{.data.password}' | base64 -d
```

## Caveats

**`hostOrUrl` mapping is deprecated** (since v1.1.11, removed in v1.3.0). If your hub currently uses it:

```yaml
# Deprecated
valueMappings:
  hostOrUrl: node_url

# Migrate to:
extraValueMappings:
  nodeUrl: node_url
# Then use {{ .nodeUrl | toHost }} in templates instead of {{ .host }}
```

## See Also

- [Datasource Views](datasource-views.md) — VIEW patterns and a complete pre-deployment verification checklist
- [Templates](templates.md) — using datasource variables in templates
- [Security](security.md) — credential management
- [Contributing a Datasource](contributing-datasource.md) — add PostgreSQL or another source
