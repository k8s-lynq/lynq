# Database per Node with Crossplane

::: warning Historical File Name
This file name contains "tenant" for historical reasons. The content has been updated to use "node" terminology throughout.
:::

::: info Multi-Tenancy Example
This guide uses **Multi-Tenancy** (SaaS application with multiple customers) as an example, which is the most common use case for Lynq. The pattern shown here can be adapted for any database-driven infrastructure automation scenario.
:::

## Overview

Provision isolated cloud databases (RDS, Cloud SQL) automatically for each node using Crossplane.

This pattern provides:
- **True Data Isolation**: Each node gets a dedicated database instance
- **Compliance**: Meets regulatory requirements for data separation
- **Performance**: No noisy neighbor problems
- **Scalability**: Independent database sizing per node
- **Automated Provisioning**: Cloud resources managed as Kubernetes objects

## Prerequisites

This pattern assumes **Crossplane** is already installed in your cluster.

::: tip
For Crossplane installation, see [Crossplane Integration](/integration-crossplane) documentation.
:::

### Install AWS Provider

```bash
# Install AWS provider
kubectl apply -f - <<EOF
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-aws
spec:
  package: xpkg.upbound.io/upbound/provider-aws:v0.40.0
EOF

# Configure AWS credentials
kubectl create secret generic aws-creds -n crossplane-system \
  --from-file=credentials=$HOME/.aws/credentials

# Create ProviderConfig
kubectl apply -f - <<EOF
apiVersion: aws.upbound.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: aws-creds
      key: credentials
EOF
```

## Database Schema

```sql
CREATE TABLE nodes (
  node_id VARCHAR(63) PRIMARY KEY,
  domain VARCHAR(255) NOT NULL,
  is_active BOOLEAN DEFAULT TRUE,

  -- Database provisioning
  db_type VARCHAR(20) DEFAULT 'postgres',           -- postgres, mysql
  db_instance_class VARCHAR(30) DEFAULT 'db.t3.micro',
  db_storage_gb INT DEFAULT 20,
  db_multi_az BOOLEAN DEFAULT FALSE,

  -- RDS identifier (populated after provisioning)
  rds_instance_id VARCHAR(255),
  rds_endpoint VARCHAR(255),

  plan_type VARCHAR(20) DEFAULT 'basic'             -- basic, pro, enterprise
);
```

## LynqHub

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
      host: mysql.database.svc.cluster.local
      port: 3306
      database: nodes_db
      username: node_reader
      passwordRef:
        name: mysql-credentials
        key: password
      table: nodes

  valueMappings:
    uid: node_id
    # DEPRECATED v1.1.11+: Use extraValueMappings instead
    #     hostOrUrl: domain
    activate: is_active

  extraValueMappings:
    dbInstanceClass: db_instance_class
    dbStorageGb: db_storage_gb
    dbMultiAz: db_multi_az
    planType: plan_type
```

## LynqForm with Crossplane Resources

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: database-provisioning
  namespace: lynq-system
spec:
  hubId: database-per-node

  # Create node namespace
  namespaces:
    - id: node-namespace
      nameTemplate: "node-{{ .uid }}"
      spec:
        apiVersion: v1
        kind: Namespace
        metadata:
          labels:
            node-id: "{{ .uid }}"

  # Provision RDS instance via Crossplane
  manifests:
    - id: rds-instance
      nameTemplate: "{{ .uid }}-postgres"
      targetNamespace: "node-{{ .uid }}"
      dependIds: ["node-namespace"]
      creationPolicy: Once  # Create database only once
      deletionPolicy: Retain  # Retain database even if node deleted (backup first!)
      waitForReady: true
      timeoutSeconds: 1800  # RDS can take 15-30 minutes
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
            allocatedStorage: {{ .dbStorageGb }}
            storageType: gp3
            storageEncrypted: true
            multiAZ: {{ .dbMultiAz }}
            publiclyAccessible: false
            vpcSecurityGroupIds:
              - sg-0123456789abcdef0  # Your VPC security group
            dbSubnetGroupName: node-db-subnet-group
            skipFinalSnapshot: false
            finalDBSnapshotIdentifier: "{{ .uid }}-final-snapshot-{{ now | date \"20060102150405\" }}"
            tags:
              - key: node-id
                value: "{{ .uid }}"
              - key: plan-type
                value: "{{ .planType }}"
              - key: managed-by
                value: lynq
          writeConnectionSecretToRef:
            name: "{{ .uid }}-db-conn"
            namespace: "node-{{ .uid }}"
          providerConfigRef:
            name: aws-provider-config

  # Application deployment (waits for database)
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
            node-id: "{{ .uid }}"
        spec:
          replicas: 2
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
                    # Crossplane automatically creates connection secret
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
                    - name: DATABASE_NAME
                      value: "{{ .uid }}"
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
                  ports:
                    - containerPort: 8080
                  resources:
                    requests:
                      cpu: "{{ if eq .planType \"enterprise\" }}1000m{{ else }}500m{{ end }}"
                      memory: "{{ if eq .planType \"enterprise\" }}2Gi{{ else }}1Gi{{ end }}"
```

::: tip Connection Secret
Crossplane automatically creates a Secret with connection details (endpoint, port, username, password) that your application can consume.
:::

## Database Connection Secret

Crossplane automatically creates a Secret with connection details:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: acme-corp-db-conn
  namespace: node-acme-corp
type: connection.crossplane.io/v1alpha1
data:
  endpoint: <base64-encoded-rds-endpoint>
  port: NTQzMg==  # 5432
  username: <base64-encoded-username>
  password: <base64-encoded-password>
```

## Monitoring Provisioning

```bash
# Check RDS provisioning status
kubectl get rdsinstance -l node-id=acme-corp

# Expected output:
# NAME              READY   SYNCED   EXTERNAL-NAME                    AGE
# acme-corp-db      True    True     acme-corp-db-20231105123456      25m

# Check connection secret
kubectl get secret acme-corp-db-conn -n node-acme-corp -o yaml

# Verify application can connect
kubectl logs -n node-acme-corp deployment/acme-corp-app
```

## Complete Verification Flow

Verify the complete database provisioning workflow:

### Step 1: Database Record Created

```sql
-- Insert new node with database requirements
INSERT INTO nodes (node_id, domain, is_active, db_instance_class, db_storage_gb, plan_type)
VALUES ('acme-corp', 'acme.example.com', TRUE, 'db.t3.small', 50, 'pro');
```

### Step 2: Verify LynqNode Created

```bash
# Wait for Lynq to sync (default: 1 minute)
kubectl get lynqnode -n lynq-system | grep acme-corp

# Expected output:
# NAME                          READY   DESIRED   FAILED   AGE
# acme-corp-database-provisioning   0/4     4         0        30s

# Check LynqNode status
kubectl describe lynqnode acme-corp-database-provisioning -n lynq-system | tail -20
# Look for: "Waiting for rds-instance to become ready"
```

### Step 3: Monitor Crossplane RDS Provisioning

```bash
# Watch RDS instance status (takes 15-30 minutes)
kubectl get rdsinstance -l node-id=acme-corp -w

# Example output over time:
# NAME               READY   SYNCED   STATE        AGE
# acme-corp-postgres False   True     creating     1m
# acme-corp-postgres False   True     creating     5m
# acme-corp-postgres False   True     backing-up   10m
# acme-corp-postgres True    True     available    15m

# Check detailed status
kubectl describe rdsinstance acme-corp-postgres | grep -A 10 "Status:"
```

### Step 4: Verify Connection Secret Created

```bash
# Check Crossplane created the connection secret
kubectl get secret acme-corp-db-conn -n node-acme-corp

# Expected output:
# NAME                  TYPE                              DATA   AGE
# acme-corp-db-conn     connection.crossplane.io/v1alpha1 4      15m

# Verify secret keys
kubectl get secret acme-corp-db-conn -n node-acme-corp -o jsonpath='{.data}' | jq 'keys'
# Expected: ["endpoint", "password", "port", "username"]
```

### Step 5: Verify Application Can Connect

```bash
# Check deployment is running
kubectl get deployment acme-corp-app -n node-acme-corp

# Expected:
# NAME              READY   UP-TO-DATE   AVAILABLE   AGE
# acme-corp-app     2/2     2            2           10m

# Check application logs for database connection
kubectl logs -n node-acme-corp deployment/acme-corp-app --tail=20 | grep -i database
# Expected: "Connected to database successfully" or similar

# Test database connectivity from pod
kubectl exec -n node-acme-corp deployment/acme-corp-app -- \
  env | grep DATABASE
# Expected: DATABASE_HOST=acme-corp-postgres-xxxxx.rds.amazonaws.com
```

### Complete Health Check Script

```bash
#!/bin/bash
# verify-database-per-node.sh <node-id>

NODE_ID=$1
NAMESPACE="node-${NODE_ID}"

echo "=== Database-per-Node Verification for ${NODE_ID} ==="
echo ""

# 1. LynqNode status
echo "1. LynqNode Status:"
kubectl get lynqnode ${NODE_ID}-database-provisioning -n lynq-system -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
echo ""

# 2. RDS Instance status
echo "2. RDS Instance Status:"
kubectl get rdsinstance ${NODE_ID}-postgres -o jsonpath='{.status.atProvider.dbInstanceStatus}' 2>/dev/null || echo "Not found"
echo ""

# 3. Connection secret
echo "3. Connection Secret:"
kubectl get secret ${NODE_ID}-db-conn -n ${NAMESPACE} -o jsonpath='{.data.endpoint}' | base64 -d 2>/dev/null || echo "Not found"
echo ""

# 4. Application deployment
echo "4. Application Status:"
kubectl get deployment ${NODE_ID}-app -n ${NAMESPACE} -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || echo "Not found"
echo ""

# 5. Database connectivity test
echo "5. Database Connectivity:"
kubectl exec -n ${NAMESPACE} deployment/${NODE_ID}-app -- \
  sh -c 'pg_isready -h $DATABASE_HOST -p 5432' 2>/dev/null || echo "Cannot test"

echo ""
echo "=== Verification Complete ==="
```

## Cost Optimization

Define tiered database offerings:

```yaml
# Different instance classes per plan
db.t3.micro   # Basic plan: $15/month
db.t3.small   # Pro plan: $30/month
db.m5.large   # Enterprise plan: $150/month
```

Use database views to filter nodes by plan:

```sql
CREATE VIEW enterprise_nodes AS
SELECT * FROM nodes WHERE plan_type = 'enterprise' AND is_active = TRUE;
```

Then create separate registries and templates per plan tier.

## Benefits

1. **True Isolation**: Each node gets dedicated database instance
2. **Cloud-Native**: Leverage managed database services (RDS, Cloud SQL)
3. **Automatic Credentials**: Crossplane manages connection secrets
4. **Declarative**: Database provisioning as code
5. **Retention Policy**: Keep data even after node deletion

## Limitations

1. **Cost**: More expensive than shared database
2. **Provisioning Time**: RDS takes 15-30 minutes to provision
3. **Management Overhead**: More databases to backup and maintain
4. **Resource Limits**: AWS account limits on RDS instances

## Related Documentation

- [Crossplane Integration](/integration-crossplane) - Detailed Crossplane setup
- [Policies](/policies) - CreationPolicy and DeletionPolicy for databases
- [Advanced Use Cases](/advanced-use-cases) - Other infrastructure patterns

## Next Steps

- Set up Crossplane provider
- Configure VPC and security groups
- Implement backup strategy
- Set up cost monitoring
