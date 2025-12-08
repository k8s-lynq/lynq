# Step 3: Setup MySQL Database

Now let's deploy a MySQL database that will become the **single source of truth** for our application configurations.

## Deploy MySQL

Create the MySQL deployment with a configuration table:

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
    CREATE DATABASE IF NOT EXISTS config_db;
    USE config_db;

    CREATE TABLE IF NOT EXISTS app_configs (
      id INT AUTO_INCREMENT PRIMARY KEY,
      app_id VARCHAR(63) NOT NULL UNIQUE,
      app_name VARCHAR(255),
      database_url VARCHAR(255),
      feature_flag BOOLEAN DEFAULT false,
      max_connections INT DEFAULT 100,
      log_level VARCHAR(20) DEFAULT 'info',
      is_active BOOLEAN DEFAULT true,
      updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
    );

    -- Insert configuration matching our existing ConfigMaps
    -- Note: These values will OVERRIDE the existing ConfigMap values!
    INSERT INTO app_configs (app_id, app_name, database_url, feature_flag, max_connections, log_level, is_active) VALUES
    ('app-alpha', 'Alpha Application v2.0', 'postgres://new-db.prod:5432/alpha', true, 100, 'info', true),
    ('app-beta', 'Beta Application v2.0', 'postgres://new-db.prod:5432/beta', true, 200, 'info', true),
    ('app-gamma', 'Gamma Application v2.0', 'postgres://new-db.prod:5432/gamma', false, 50, 'warn', true);

    -- Create read-only user for Lynq
    CREATE USER IF NOT EXISTS 'lynq_reader'@'%' IDENTIFIED BY 'lynqpass123';
    GRANT SELECT ON config_db.* TO 'lynq_reader'@'%';
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

Wait for MySQL pod to be fully ready:

```bash
kubectl wait --for=condition=Ready --timeout=120s -n lynq-demo pod -l app=mysql
```{{exec}}

## View Database Configuration

Check the configuration data in MySQL:

```bash
kubectl exec -n lynq-demo deployment/mysql -- \
  mysql -h 127.0.0.1 -u root -prootpass123 -e \
  "SELECT app_id, app_name, database_url, feature_flag, max_connections, log_level FROM config_db.app_configs;"
```{{exec}}

## Compare: Database vs Current ConfigMaps

Notice the differences:

| Field | Current ConfigMap | Database (New) |
|-------|-------------------|----------------|
| `DATABASE_URL` | `postgres://old-db:5432/*` | `postgres://new-db.prod:5432/*` |
| `FEATURE_FLAG` | `false` (alpha, beta) | `true` (alpha, beta) |
| `MAX_CONNECTIONS` | Varies | Updated values |
| `LOG_LEVEL` | Varies | Standardized |

The database contains the **desired state**. Lynq will sync these values to the ConfigMaps!

✅ **Checkpoint**: MySQL is running with configuration data that will be synced to existing ConfigMaps.

Click **Continue** to install the Lynq operator.
