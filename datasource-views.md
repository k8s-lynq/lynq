---
url: 'https://lynq.sh/datasource-views.md'
description: >-
  MySQL VIEW patterns for Lynq: transform status strings, combine columns,
  filter rows, add computed fields, and verify your VIEW before deployment.
---

# Datasource Views

When your database schema doesn't match the format Lynq expects, create a MySQL VIEW to transform the data. The hub points at the view name instead of the raw table — Lynq sees the transformed output.

```
Your table (any schema)  →  MySQL VIEW (normalized)  →  LynqHub
```

## When to Use a VIEW

* Your activate column holds strings like `"active"` / `"inactive"` (not `"1"` / `"0"`)
* The node ID needs to be constructed from multiple columns
* You want to apply business logic (only paying customers, only non-expired nodes)
* You need derived data in templates (computed strings, mapped values)
* You want to expose only specific columns to Lynq for security

## Patterns

### Pattern 1: Transform a Status String

**Problem:** `status` column has values like `"active"`, `"inactive"`, `"suspended"`.

```sql
CREATE VIEW node_configs AS
SELECT
    id           AS node_id,
    CASE
        WHEN status = 'active' THEN '1'
        ELSE '0'
    END          AS is_active,
    plan         AS subscription_plan,
    region       AS deployment_region
FROM nodes
WHERE status IN ('active', 'inactive');  -- exclude 'suspended' entirely
```

### Pattern 2: Combine Columns into a Node ID

**Problem:** No single column works as a unique identifier.

```sql
CREATE VIEW node_configs AS
SELECT
    CONCAT('customer-', customer_id) AS node_id,
    CONCAT('https://', subdomain, '.', domain) AS node_url,
    IF(enabled = 1, '1', '0')        AS is_active,
    plan
FROM customers;
```

### Pattern 3: Filter with Business Logic

**Problem:** Active nodes require a paid, non-expired subscription.

```sql
CREATE VIEW active_paying_nodes AS
SELECT
    n.node_id,
    '1'                    AS is_active,
    s.subscription_tier    AS plan,
    MAX(s.license_count)   AS max_users
FROM nodes n
JOIN subscriptions s ON n.id = s.node_id
WHERE
    s.status         = 'active'
    AND s.payment_status = 'paid'
    AND s.expiry_date     > NOW()
GROUP BY n.node_id, s.subscription_tier;
```

### Pattern 4: Add Computed Columns

**Problem:** Templates need derived values — CDN URLs, replica counts based on plan, etc.

```sql
CREATE VIEW node_configs AS
SELECT
    node_id,
    is_active,
    subscription_plan,
    CONCAT('https://cdn-', deployment_region, '.example.com') AS cdn_url,
    CASE subscription_plan
        WHEN 'enterprise' THEN '100'
        WHEN 'business'   THEN '50'
        ELSE                       '10'
    END AS max_replicas
FROM nodes;
```

Then in the hub:

```yaml
extraValueMappings:
  cdnUrl: cdn_url
  maxReplicas: max_replicas
```

And in templates:

```yaml
spec:
  replicas: {{ .maxReplicas }}
  containers:
  - name: app
    env:
    - name: CDN_URL
      value: "{{ .cdnUrl }}"
```

## Pre-Deployment Verification Checklist

Before pointing a LynqHub at your VIEW, run these checks.

### Step 1: Verify structure

```sql
DESCRIBE node_configs;
-- Check that uid and activate columns are present with expected names
```

### Step 2: Check activate values

```sql
SELECT
    is_active,
    COUNT(*) AS count,
    CASE
        WHEN is_active IN ('1', 'true', 'TRUE', 'yes', 'YES') THEN 'active'
        WHEN is_active IN ('0', 'false', 'FALSE', 'no', 'NO', '') THEN 'inactive'
        ELSE 'INVALID'
    END AS status
FROM node_configs
GROUP BY is_active;
-- "INVALID" rows will be ignored by Lynq; no error is raised
```

### Step 3: Check for duplicate UIDs

```sql
-- Must return empty
SELECT node_id, COUNT(*) AS count
FROM node_configs
GROUP BY node_id
HAVING count > 1;
```

### Step 4: Check for NULL UIDs

```sql
-- Must return empty
SELECT * FROM node_configs WHERE node_id IS NULL OR node_id = '';
```

### Step 5: Preview what Lynq will read

```sql
SELECT * FROM node_configs LIMIT 10;
-- Verify UIDs, activate values, and extra columns look correct
```

### Step 6: Test as the read-only user

```bash
mysql -h mysql-server -u node_reader -p
# Then:
mysql> SELECT * FROM mydb.node_configs LIMIT 5;
```

### Step 7: Test from inside the cluster

```bash
kubectl run mysql-test --rm -it --image=mysql:8 -- \
  mysql -h mysql.default.svc.cluster.local \
        -u node_reader \
        -p'your-password' \
        -e "SELECT * FROM mydb.node_configs LIMIT 5"
```

### Step 8: Count expected LynqNodes

```sql
SELECT COUNT(*) AS expected_lynqnodes
FROM node_configs
WHERE is_active IN ('1', 'true', 'TRUE', 'yes', 'YES');
-- This count × number of LynqForms = total LynqNodes that will be created
```

### Automated Script

Save as `verify_view.sql` and run `mysql -u root < verify_view.sql`:

```sql
USE mydb;

SELECT '=== Structure ===' AS step;
DESCRIBE node_configs;

SELECT '=== Required columns ===' AS step;
SELECT
    MAX(CASE WHEN COLUMN_NAME = 'node_id'   THEN 'FOUND' ELSE 'MISSING' END) AS uid_column,
    MAX(CASE WHEN COLUMN_NAME = 'is_active' THEN 'FOUND' ELSE 'MISSING' END) AS activate_column
FROM INFORMATION_SCHEMA.COLUMNS
WHERE TABLE_SCHEMA = 'mydb' AND TABLE_NAME = 'node_configs';

SELECT '=== Activate distribution ===' AS step;
SELECT is_active, COUNT(*) AS count FROM node_configs GROUP BY is_active;

SELECT '=== Duplicate UIDs (must be empty) ===' AS step;
SELECT node_id, COUNT(*) FROM node_configs GROUP BY node_id HAVING COUNT(*) > 1;

SELECT '=== NULL UIDs (must be empty) ===' AS step;
SELECT * FROM node_configs WHERE node_id IS NULL OR node_id = '';

SELECT '=== Sample rows ===' AS step;
SELECT * FROM node_configs LIMIT 10;

SELECT '=== Expected LynqNodes ===' AS step;
SELECT COUNT(*) AS active_rows
FROM node_configs
WHERE is_active IN ('1', 'true', 'TRUE', 'yes', 'YES');
```

### After Deploying the Hub

```bash
kubectl get lynqhub my-hub -n lynq-system -o jsonpath='{.status}'
# "desired" should match the active_rows count from Step 8 (times number of forms)

kubectl get lynqnodes -n lynq-system
# One LynqNode per active row per LynqForm
```

## Security: Restrict Column Access

Grant the read-only user access only to the view, not the underlying table:

```sql
-- ❌ Too broad
GRANT SELECT ON nodes.* TO 'node_reader'@'%';

-- ✅ View only (sensitive columns never exposed)
GRANT SELECT ON nodes.node_configs TO 'node_reader'@'%';
```

## See Also

* [Datasource](datasource.md) — connection setup, column mappings, basic schema examples
* [Templates](templates.md) — using mapped variables in LynqForm templates
* [Contributing a Datasource](contributing-datasource.md) — add support for PostgreSQL or other sources
