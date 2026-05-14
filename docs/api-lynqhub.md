---
description: "LynqHub API reference — spec fields, status fields, and validation rules."
---

# LynqHub API Reference

**Kind:** `LynqHub`  
**API Version:** `operator.lynq.sh/v1`  
**Group:** `operator.lynq.sh`

LynqHub connects to a database, queries active rows on a configurable interval, and creates/deletes LynqNode CRs to match the active set. One hub can be referenced by multiple LynqForms.

→ [Datasource configuration guide](datasource.md) · [API index](api.md)

## Spec

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: my-hub
  namespace: lynq-system
spec:
  source:
    type: mysql                      # mysql (postgresql planned for v1.2)
    mysql:
      host: string                   # MySQL hostname or IP (required)
      port: 3306                     # MySQL port (default: 3306)
      username: string               # Database username (required)
      passwordRef:
        name: string                 # Kubernetes Secret name (required)
        key: string                  # Secret key containing password (required)
      database: string               # Database name (required)
      table: string                  # Table or view name (required)
    syncInterval: "1m"               # Poll frequency, e.g. 30s, 1m, 5m (required)

  valueMappings:
    uid: string                      # Column mapping for unique node ID (required)
    activate: string                 # Column mapping for activation flag (required)
    # hostOrUrl: string              # DEPRECATED since v1.1.11, removed in v1.3.0

  extraValueMappings:                # Optional additional column → variable mappings
    planId: subscription_plan        # Available as {{ .planId }} in templates
    region: deployment_region        # Available as {{ .region }} in templates
```

### `spec.source.mysql` fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `host` | string | ✓ | MySQL hostname or IP address |
| `port` | integer | | MySQL port (default: `3306`) |
| `username` | string | ✓ | Database username |
| `passwordRef.name` | string | ✓ | Kubernetes Secret name |
| `passwordRef.key` | string | ✓ | Key within the Secret |
| `database` | string | ✓ | Database name |
| `table` | string | ✓ | Table or view to query |

### `spec.source.syncInterval`

Duration string format: `<number><unit>` where unit is `s`, `m`, or `h`.

| Value | Meaning |
|-------|---------|
| `30s` | Every 30 seconds |
| `1m` | Every minute (recommended for production) |
| `5m` | Every 5 minutes (for large deployments) |

### `spec.valueMappings`

Maps database column names to the two required template variables.

| Key | Required | Purpose |
|-----|----------|---------|
| `uid` | ✓ | Maps a column to `.uid` — unique node identifier, used in resource naming |
| `activate` | ✓ | Maps a column to `.activate` — truthy/falsy activation flag |
| `hostOrUrl` | Deprecated | Removed in v1.3.0. Use `extraValueMappings` + `toHost()` instead |

**Accepted truthy values for `activate`:** `1`, `true`, `TRUE`, `True`, `yes`, `YES`, `Yes`. All other values (including `NULL`) are treated as inactive.

### `spec.extraValueMappings`

Optional map of `templateVariable: databaseColumn`. Each entry makes a new variable available in all templates for this hub's LynqNodes.

```yaml
extraValueMappings:
  planId: subscription_plan   # → {{ .planId }}
  region: aws_region          # → {{ .region }}
  nodeUrl: node_url           # → {{ .nodeUrl | toHost }} for hostname extraction
```

## Status

```yaml
status:
  observedGeneration: int64
  referencingTemplates: int32        # Number of LynqForms referencing this hub
  desired: int32                     # referencingTemplates × activeRows
  ready: int32                       # LynqNodes with Ready=True
  failed: int32                      # LynqNodes with reconciliation failures
  lastSyncTime: timestamp            # Last successful database sync
  conditions:
  - type: Ready
    status: "True" | "False" | "Unknown"
    reason: string
    message: string
    lastTransitionTime: timestamp
```

### `status.desired` calculation

```
desired = referencingTemplates × activeRows
```

A hub with 3 active rows and 2 referencing LynqForms has `desired: 6`.

### Ready condition reasons

| Reason | Status | Meaning |
|--------|--------|---------|
| `SyncSucceeded` | True | Last sync completed successfully |
| `DatabaseConnectionFailed` | False | Cannot reach the database |
| `QueryFailed` | False | Connection succeeded but query failed |
| `SyncInProgress` | Unknown | Sync is running |

## Validation

The admission webhook enforces:
- `spec.valueMappings` must include `uid` and `activate`
- `spec.source.syncInterval` must match `^\d+(s|m|h)$`
- `spec.source.mysql.host` is required when `type: mysql`

## Example

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: production-nodes
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
      database: saas_db
      table: node_configs
    syncInterval: 1m
  valueMappings:
    uid: node_id
    activate: is_active
  extraValueMappings:
    planId: subscription_plan
    region: deployment_region
```

## See Also

- [Datasource](datasource.md) — connection setup, column mappings, VIEW patterns
- [LynqForm API](api-lynqform.md) — resource blueprint
- [LynqNode API](api-lynqnode.md) — instance status
- [API index](api.md) — common types and kubectl reference
