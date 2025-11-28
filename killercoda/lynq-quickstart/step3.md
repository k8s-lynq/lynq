# Step 3: Setup MySQL Database

Now let's deploy a MySQL database with sample tenant data. This simulates your production database containing customer/tenant information.

## Deploy MySQL

Create the MySQL deployment with initial data:

```bash
cat <<'EOF' | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: mysql-credentials
  namespace: lynq-demo
type: Opaque
stringData:
  root-password: "rootpass123"
  password: "lynqpass123"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: mysql-init
  namespace: lynq-demo
data:
  init.sql: |
    CREATE DATABASE IF NOT EXISTS tenants;
    USE tenants;

    CREATE TABLE IF NOT EXISTS tenant_configs (
      id INT AUTO_INCREMENT PRIMARY KEY,
      tenant_id VARCHAR(63) NOT NULL UNIQUE,
      tenant_url VARCHAR(255),
      is_active BOOLEAN DEFAULT true,
      plan VARCHAR(50) DEFAULT 'basic',
      created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );

    -- Insert sample tenants
    INSERT INTO tenant_configs (tenant_id, tenant_url, is_active, plan) VALUES
    ('acme-corp', 'https://acme.example.com', true, 'enterprise'),
    ('beta-inc', 'https://beta.example.com', true, 'basic'),
    ('gamma-ltd', 'https://gamma.example.com', false, 'basic');

    -- Create read-only user for Lynq
    CREATE USER IF NOT EXISTS 'lynq_reader'@'%' IDENTIFIED BY 'lynqpass123';
    GRANT SELECT ON tenants.* TO 'lynq_reader'@'%';
    FLUSH PRIVILEGES;
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mysql
  namespace: lynq-demo
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mysql
  template:
    metadata:
      labels:
        app: mysql
    spec:
      containers:
      - name: mysql
        image: mysql:8.0
        env:
        - name: MYSQL_ROOT_PASSWORD
          valueFrom:
            secretKeyRef:
              name: mysql-credentials
              key: root-password
        ports:
        - containerPort: 3306
        readinessProbe:
          exec:
            command:
            - mysqladmin
            - ping
            - -h
            - "127.0.0.1"
            - -uroot
            - -prootpass123
          initialDelaySeconds: 10
          periodSeconds: 5
          timeoutSeconds: 5
        volumeMounts:
        - name: init-script
          mountPath: /docker-entrypoint-initdb.d
      volumes:
      - name: init-script
        configMap:
          name: mysql-init
---
apiVersion: v1
kind: Service
metadata:
  name: mysql
  namespace: lynq-demo
spec:
  selector:
    app: mysql
  ports:
  - port: 3306
    targetPort: 3306
EOF
```{{exec}}

## Wait for MySQL

Wait for MySQL pod to be fully ready (this may take 30-60 seconds):

```bash
kubectl wait --for=condition=Ready --timeout=120s -n lynq-demo pod -l app=mysql
```{{exec}}

## Verify Database

Check the sample data in MySQL:

```bash
kubectl exec -n lynq-demo deployment/mysql -- \
  mysql -h 127.0.0.1 -u root -prootpass123 -e "SELECT tenant_id, is_active, plan FROM tenants.tenant_configs;"
```{{exec}}

You should see three tenants:
- `acme-corp` - Active (enterprise plan)
- `beta-inc` - Active (basic plan)
- `gamma-ltd` - **Inactive** (won't be provisioned)

✅ **Checkpoint**: MySQL is running with:
- 2 active tenants (acme-corp, beta-inc)
- 1 inactive tenant (gamma-ltd)

Click **Continue** to create the LynqHub.
