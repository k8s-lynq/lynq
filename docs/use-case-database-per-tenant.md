---
description: "Provision a dedicated cloud database (RDS, Cloud SQL) per node using Lynq and Crossplane, driven by a database row."
---

# Database per Node with Crossplane

Some compliance requirements, data isolation needs, or enterprise contracts mean every customer must have their own database instance — not a schema, not a tenant prefix, a full RDS instance. Provisioning and decommissioning these manually is error-prone and time-consuming.

Add a row to your `nodes` table and Lynq creates the Kubernetes workload; Crossplane provisions the RDS instance. When the row is deleted, the app is torn down and the database is preserved with a final snapshot.

::: tip Time to working
~15 minutes to configure. RDS instance provisioning takes 15–30 minutes after the row is inserted (AWS limitation, not Lynq).
:::

## How It Works

- Each active row triggers a Crossplane `RDSInstance` resource alongside the application Deployment.
- The application Deployment has `dependIds: ["rds-instance"]` — it won't start until RDS is `READY`.
- `creationPolicy: Once` on the RDS resource means the database is created once and never replaced, even if the node spec changes. `deletionPolicy: Retain` preserves it when the node is deleted.

## Prerequisites

Crossplane must be installed with the AWS provider configured. See [Crossplane Integration](./integration-crossplane.md) for a complete setup guide.

```bash
# Verify Crossplane and AWS provider are ready
kubectl get providers
# NAME           INSTALLED   HEALTHY   PACKAGE                                    AGE
# provider-aws   True        True      xpkg.upbound.io/upbound/provider-aws:...   5m
```

## Database Schema

```sql
CREATE TABLE nodes (
  node_id           VARCHAR(63)   PRIMARY KEY,
  is_active         BOOLEAN       DEFAULT TRUE,

  -- RDS configuration; set once when the node is onboarded
  db_instance_class VARCHAR(30)   DEFAULT 'db.t3.micro',  -- db.t3.micro | db.t3.small | db.m5.large
  db_storage_gb     INT           DEFAULT 20,
  db_multi_az       BOOLEAN       DEFAULT FALSE,
  plan_type         VARCHAR(20)   DEFAULT 'basic'          -- basic | pro | enterprise
);
```

## Minimal Setup

The core pattern: Crossplane `RDSInstance` + application Deployment with dependency ordering.

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: database-per-node
  namespace: lynq-system
spec:
  source:
    type: mysql
    syncInterval: 1m
    mysql:
      host: mysql.internal.svc.cluster.local
      port: 3306
      database: nodes_db
      table: nodes
      username: lynq_reader
      passwordRef:
        name: mysql-credentials
        key: password
  valueMappings:
    uid: node_id
    activate: is_active
  extraValueMappings:
    dbInstanceClass: db_instance_class
    dbStorageGb: db_storage_gb
    dbMultiAz: db_multi_az
    planType: plan_type
```

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: database-provisioning
  namespace: lynq-system
spec:
  hubId: database-per-node

  namespaces:
    - id: ns
      nameTemplate: "node-{{ .uid }}"
      spec:
        apiVersion: v1
        kind: Namespace
        metadata:
          labels:
            node-id: "{{ .uid }}"

  manifests:
    - id: rds-instance
      nameTemplate: "{{ .uid }}-postgres"
      targetNamespace: "node-{{ .uid }}"
      dependIds: ["ns"]
      creationPolicy: Once    # create once, never replace
      deletionPolicy: Retain  # keep the database when the node is deleted
      waitForReady: true
      timeoutSeconds: 1800    # RDS provisioning takes 15-30 minutes
      spec:
        apiVersion: database.aws.crossplane.io/v1beta1
        kind: RDSInstance
        metadata:
          labels:
            node-id: "{{ .uid }}"
        spec:
          forProvider:
            region: us-west-2
            dbInstanceClass: "{{ .dbInstanceClass }}"
            engine: postgres
            engineVersion: "15.3"
            masterUsername: "{{ .uid }}"
            allocatedStorage: {{ .dbStorageGb | int }}
            storageEncrypted: true
            multiAZ: {{ .dbMultiAz }}
            publiclyAccessible: false
            skipFinalSnapshot: false
            finalDBSnapshotIdentifier: "{{ .uid }}-final-{{ now | date \"20060102150405\" }}"
          writeConnectionSecretToRef:
            name: "{{ .uid }}-db-conn"
            namespace: "node-{{ .uid }}"
          providerConfigRef:
            name: default
```

## Full Example

Adds the application Deployment that waits for the RDS instance to be ready and reads credentials from the Crossplane-managed Secret.

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: database-provisioning
  namespace: lynq-system
spec:
  hubId: database-per-node

  namespaces:
    - id: ns
      nameTemplate: "node-{{ .uid }}"
      spec:
        apiVersion: v1
        kind: Namespace
        metadata:
          labels:
            node-id: "{{ .uid }}"

  manifests:
    - id: rds-instance
      nameTemplate: "{{ .uid }}-postgres"
      targetNamespace: "node-{{ .uid }}"
      dependIds: ["ns"]
      creationPolicy: Once
      deletionPolicy: Retain
      waitForReady: true
      timeoutSeconds: 1800
      spec:
        apiVersion: database.aws.crossplane.io/v1beta1
        kind: RDSInstance
        metadata:
          labels:
            node-id: "{{ .uid }}"
            plan-type: "{{ .planType }}"
        spec:
          forProvider:
            region: us-west-2
            dbInstanceClass: "{{ .dbInstanceClass }}"
            engine: postgres
            engineVersion: "15.3"
            masterUsername: "{{ .uid }}"
            allocatedStorage: {{ .dbStorageGb | int }}
            storageType: gp3
            storageEncrypted: true
            multiAZ: {{ .dbMultiAz }}
            publiclyAccessible: false
            vpcSecurityGroupIds:
              - sg-0123456789abcdef0
            dbSubnetGroupName: node-db-subnet-group
            skipFinalSnapshot: false
            finalDBSnapshotIdentifier: "{{ .uid }}-final-{{ now | date \"20060102150405\" }}"
            tags:
              - key: node-id
                value: "{{ .uid }}"
              - key: managed-by
                value: lynq
          writeConnectionSecretToRef:
            name: "{{ .uid }}-db-conn"
            namespace: "node-{{ .uid }}"
          providerConfigRef:
            name: default

  deployments:
    - id: app
      nameTemplate: "{{ .uid }}-app"
      targetNamespace: "node-{{ .uid }}"
      dependIds: ["rds-instance"]
      waitForReady: true
      spec:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          labels:
            app: "{{ .uid }}"
        spec:
          replicas: {{ ternary 2 1 (eq .planType "enterprise") | int }}
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
                  image: registry.example.com/node-app:v1.0.0
                  env:
                    - name: NODE_ID
                      value: "{{ .uid }}"
                    - name: DATABASE_HOST
                      valueFrom:
                        secretKeyRef:
                          name: "{{ .uid }}-db-conn"
                          key: endpoint
                    - name: DATABASE_PORT
                      valueFrom:
                        secretKeyRef:
                          name: "{{ .uid }}-db-conn"
                          key: port
                    - name: DATABASE_USER
                      valueFrom:
                        secretKeyRef:
                          name: "{{ .uid }}-db-conn"
                          key: username
                    - name: DATABASE_PASSWORD
                      valueFrom:
                        secretKeyRef:
                          name: "{{ .uid }}-db-conn"
                          key: password
                    - name: DATABASE_NAME
                      value: "{{ .uid }}"
                  ports:
                    - containerPort: 8080
                  resources:
                    requests:
                      cpu: "{{ ternary \"1000m\" \"500m\" (eq .planType \"enterprise\") }}"
                      memory: "{{ ternary \"2Gi\" \"1Gi\" (eq .planType \"enterprise\") }}"
```

::: tip Crossplane connection secret
Crossplane writes `endpoint`, `port`, `username`, and `password` into the Secret specified by `writeConnectionSecretToRef`. The app reads credentials directly from that Secret — no manual secret management needed.
:::

## Provisioning Workflow

### 1. Insert a node record

```sql
INSERT INTO nodes (node_id, is_active, db_instance_class, db_storage_gb, plan_type)
VALUES ('acme-corp', TRUE, 'db.t3.small', 50, 'pro');
```

### 2. Lynq creates the LynqNode and starts provisioning

```bash
# After ~1 minute (hub sync interval):
kubectl get lynqnode -n lynq-system | grep acme-corp
# NAME                               READY   DESIRED   FAILED   AGE
# acme-corp-database-provisioning    False   2/3       0        1m
# ↑ RDS is pending, app is waiting (dependIds)

# Watch RDS provisioning (takes 15-30 minutes)
kubectl get rdsinstance -l node-id=acme-corp -w
# NAME               READY   SYNCED   STATE       AGE
# acme-corp-postgres False   True     creating    1m
# acme-corp-postgres False   True     backing-up  10m
# acme-corp-postgres True    True     available   20m
```

### 3. Decommission a node

```sql
-- Soft delete: preserves RDS instance (deletionPolicy: Retain)
UPDATE nodes SET is_active = FALSE WHERE node_id = 'acme-corp';
-- Or hard delete:
DELETE FROM nodes WHERE node_id = 'acme-corp';
```

The app Deployment is removed. The RDS instance is retained with a final snapshot.

## Verify It Works

```bash
# LynqNode ready after RDS is available
kubectl get lynqnode acme-corp-database-provisioning -n lynq-system
# NAME                               READY   DESIRED   FAILED
# acme-corp-database-provisioning    True    3/3       0

# Connection secret populated by Crossplane
kubectl get secret acme-corp-db-conn -n node-acme-corp -o jsonpath='{.data}' | jq 'keys'
# ["endpoint","password","port","username"]

# App is running and connected
kubectl get deployment acme-corp-app -n node-acme-corp
# NAME            READY   UP-TO-DATE   AVAILABLE
# acme-corp-app   1/1     1            1
```

## Caveats

- **RDS provisioning takes 15–30 minutes.** Set `timeoutSeconds: 1800` on the `rds-instance` resource or the node will be marked failed before RDS is ready.
- **`creationPolicy: Once` means the RDS instance is never replaced by Lynq** even if `db_instance_class` changes. To resize an RDS instance, update it directly in AWS Console or via Crossplane.
- **AWS account limits** cap how many RDS instances you can have per region. Check your limit before scaling to many nodes.

## See Also

- [Crossplane Integration](./integration-crossplane.md) — provider setup, ProviderConfig, VPC configuration
- [Policies](./policies.md) — `creationPolicy: Once` and `deletionPolicy: Retain` explained
- [Multi-Tier Stack](./use-case-multi-tier.md) — in-cluster PostgreSQL StatefulSet if you don't need managed RDS
