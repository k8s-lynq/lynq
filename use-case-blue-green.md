---
url: 'https://lynq.sh/use-case-blue-green.md'
description: >-
  Zero-downtime blue-green deployments per node, controlled by a single database
  column. One SQL UPDATE switches traffic; another reverts it.
---

# Blue-Green Deployments

A deployment goes wrong at 2am. Your rollback is one SQL statement: `UPDATE nodes SET active_color = 'blue'`. Traffic switches within the next sync interval — no YAML edits, no `helm rollback`, no kubectl involved.

The `active_color` column in your database determines which environment is live. Lynq keeps both deployments running; the Service selector follows the column value.

::: tip Time to working
\~5 minutes to configure. Traffic switches within 1 minute of a column update.
:::

## How It Works

* Each node has two Deployments (`blue` and `green`) always running — the active one at full replicas, the inactive one at 1.
  ::: v-pre
* The Service selector is a template expression: `color: "{{ .activeColor }}"`. When the column changes, Lynq updates the selector and traffic shifts.
  :::
* Rollback is the same operation as a deploy: update `active_color` and the Service re-points to the previous environment.

## Database Schema

```sql
CREATE TABLE nodes (
  node_id    VARCHAR(63)  PRIMARY KEY,
  is_active  BOOLEAN      DEFAULT TRUE,

  -- Blue-green control
  active_color       VARCHAR(10)  DEFAULT 'blue',  -- 'blue' | 'green'
  blue_version       VARCHAR(20)  DEFAULT 'v1.0.0',
  green_version      VARCHAR(20)  DEFAULT 'v1.0.0',
  deployment_status  VARCHAR(20)  DEFAULT 'stable' -- stable | deploying | testing | rolled-back
);
```

## Minimal Setup

The core of the pattern: two Deployments whose replica count follows the active color, and a Service whose selector does the same.

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: blue-green-nodes
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
    activeColor: active_color
    blueVersion: blue_version
    greenVersion: green_version
```

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: blue-green-app
  namespace: lynq-system
spec:
  hubId: blue-green-nodes

  deployments:
    - id: blue
      nameTemplate: "{{ .uid }}-blue"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: {{ ternary 3 1 (eq .activeColor "blue") | int }}
          selector:
            matchLabels:
              app: "{{ .uid }}"
              color: blue
          template:
            metadata:
              labels:
                app: "{{ .uid }}"
                color: blue
            spec:
              containers:
                - name: app
                  image: "registry.example.com/app:{{ .blueVersion }}"
                  ports:
                    - containerPort: 8080

    - id: green
      nameTemplate: "{{ .uid }}-green"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: {{ ternary 3 1 (eq .activeColor "green") | int }}
          selector:
            matchLabels:
              app: "{{ .uid }}"
              color: green
          template:
            metadata:
              labels:
                app: "{{ .uid }}"
                color: green
            spec:
              containers:
                - name: app
                  image: "registry.example.com/app:{{ .greenVersion }}"
                  ports:
                    - containerPort: 8080

  services:
    - id: main
      nameTemplate: "{{ .uid }}-app"
      dependIds: ["blue", "green"]
      spec:
        apiVersion: v1
        kind: Service
        spec:
          selector:
            app: "{{ .uid }}"
            color: "{{ .activeColor }}"
          ports:
            - port: 80
              targetPort: 8080
```

## Full Example

Add per-color test Services and Ingresses for smoke-testing before switching traffic.

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: blue-green-app
  namespace: lynq-system
spec:
  hubId: blue-green-nodes

  deployments:
    - id: blue
      nameTemplate: "{{ .uid }}-blue"
      labelsTemplate:
        app: "{{ .uid }}"
        color: blue
        active: "{{ ternary \"true\" \"false\" (eq .activeColor \"blue\") }}"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          labels:
            app: "{{ .uid }}"
            color: blue
        spec:
          replicas: {{ ternary 3 1 (eq .activeColor "blue") | int }}
          selector:
            matchLabels:
              app: "{{ .uid }}"
              color: blue
          template:
            metadata:
              labels:
                app: "{{ .uid }}"
                color: blue
                version: "{{ .blueVersion }}"
            spec:
              containers:
                - name: app
                  image: "registry.example.com/app:{{ .blueVersion }}"
                  env:
                    - name: NODE_ID
                      value: "{{ .uid }}"
                    - name: ENVIRONMENT_COLOR
                      value: blue
                  ports:
                    - containerPort: 8080
                  resources:
                    requests:
                      cpu: 500m
                      memory: 1Gi

    - id: green
      nameTemplate: "{{ .uid }}-green"
      labelsTemplate:
        app: "{{ .uid }}"
        color: green
        active: "{{ ternary \"true\" \"false\" (eq .activeColor \"green\") }}"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          labels:
            app: "{{ .uid }}"
            color: green
        spec:
          replicas: {{ ternary 3 1 (eq .activeColor "green") | int }}
          selector:
            matchLabels:
              app: "{{ .uid }}"
              color: green
          template:
            metadata:
              labels:
                app: "{{ .uid }}"
                color: green
                version: "{{ .greenVersion }}"
            spec:
              containers:
                - name: app
                  image: "registry.example.com/app:{{ .greenVersion }}"
                  env:
                    - name: NODE_ID
                      value: "{{ .uid }}"
                    - name: ENVIRONMENT_COLOR
                      value: green
                  ports:
                    - containerPort: 8080
                  resources:
                    requests:
                      cpu: 500m
                      memory: 1Gi

  services:
    - id: main
      nameTemplate: "{{ .uid }}-app"
      dependIds: ["blue", "green"]
      spec:
        apiVersion: v1
        kind: Service
        spec:
          selector:
            app: "{{ .uid }}"
            color: "{{ .activeColor }}"
          ports:
            - port: 80
              targetPort: 8080

    - id: blue-test
      nameTemplate: "{{ .uid }}-blue-test"
      spec:
        apiVersion: v1
        kind: Service
        spec:
          selector:
            app: "{{ .uid }}"
            color: blue
          ports:
            - port: 80
              targetPort: 8080

    - id: green-test
      nameTemplate: "{{ .uid }}-green-test"
      spec:
        apiVersion: v1
        kind: Service
        spec:
          selector:
            app: "{{ .uid }}"
            color: green
          ports:
            - port: 80
              targetPort: 8080

  ingresses:
    - id: main
      nameTemplate: "{{ .uid }}-ingress"
      dependIds: ["main"]
      spec:
        apiVersion: networking.k8s.io/v1
        kind: Ingress
        spec:
          ingressClassName: nginx
          rules:
            - host: "{{ .uid }}.example.com"
              http:
                paths:
                  - path: /
                    pathType: Prefix
                    backend:
                      service:
                        name: "{{ .uid }}-app"
                        port:
                          number: 80

    - id: blue-test-ingress
      nameTemplate: "{{ .uid }}-blue-test"
      dependIds: ["blue-test"]
      spec:
        apiVersion: networking.k8s.io/v1
        kind: Ingress
        spec:
          ingressClassName: nginx
          rules:
            - host: "{{ .uid }}-blue.test.example.com"
              http:
                paths:
                  - path: /
                    pathType: Prefix
                    backend:
                      service:
                        name: "{{ .uid }}-blue-test"
                        port:
                          number: 80

    - id: green-test-ingress
      nameTemplate: "{{ .uid }}-green-test"
      dependIds: ["green-test"]
      spec:
        apiVersion: networking.k8s.io/v1
        kind: Ingress
        spec:
          ingressClassName: nginx
          rules:
            - host: "{{ .uid }}-green.test.example.com"
              http:
                paths:
                  - path: /
                    pathType: Prefix
                    backend:
                      service:
                        name: "{{ .uid }}-green-test"
                        port:
                          number: 80
```

## Deployment Workflow

### 1. Stage the new version on the inactive environment

```sql
-- Blue is active. Deploy v2.0.0 to green without affecting production.
UPDATE nodes
SET green_version = 'v2.0.0',
    deployment_status = 'deploying'
WHERE node_id = 'acme-corp';
```

Lynq updates the green Deployment image. The blue Service selector is unchanged — production traffic is unaffected.

### 2. Test against the inactive environment

```bash
# Green is accessible via its dedicated test Service/Ingress
curl https://acme-corp-green.test.example.com/healthz
# {"status":"healthy","version":"v2.0.0"}
```

### 3. Switch traffic

```sql
UPDATE nodes
SET active_color = 'green',
    deployment_status = 'stable'
WHERE node_id = 'acme-corp';
```

Lynq updates the Service selector. Green scales to 3 replicas; blue scales to 1.

### 4. Roll back

```sql
UPDATE nodes
SET active_color = 'blue',
    deployment_status = 'rolled-back'
WHERE node_id = 'acme-corp';
```

### Deployment timeline

| Time | Action | Traffic |
|------|--------|---------|
| T+0 | `green_version = 'v2.0.0'` | Blue (v1.0.0) |
| T+1m | Lynq updates green Deployment | Blue (v1.0.0) |
| T+5m | Green pods ready, smoke tests | Blue (v1.0.0) |
| T+15m | `active_color = 'green'` | **Green (v2.0.0)** |
| T+20m | Issue detected | Green (v2.0.0) |
| T+21m | `active_color = 'blue'` | **Blue (v1.0.0)** |
| T+22m | Traffic restored | Blue (v1.0.0) |

**Total rollback time: ~1–2 minutes.**

## Verify It Works

```bash
# Service selector follows active_color
kubectl get svc acme-corp-app -o jsonpath='{.spec.selector.color}'
# blue

# After switching active_color to green:
kubectl get svc acme-corp-app -o jsonpath='{.spec.selector.color}'
# green

# Replica counts reflect active/inactive
kubectl get deployment -l app=acme-corp
# NAME              READY   REPLICAS
# acme-corp-blue    1/1     1        ← inactive (scaled down)
# acme-corp-green   3/3     3        ← active (scaled up)
```

## Caveats

* **Schema changes must be backward compatible** during the transition window — both colors run simultaneously and share the same database.
* **Stateful applications** (session affinity, in-flight requests) need additional handling; the Service selector switch is instant but in-flight connections to the old pods drain naturally.
* **Double resource usage** during deployments. The inactive environment runs at 1 replica to minimize cost, not 0 — scaling to 0 would add pod start latency to the deploy.

## See Also

* [Feature Flags](./use-case-feature-flags.md) — use the same column-as-switch pattern for application-level feature toggles
* [Policies](./policies.md) — `deletionPolicy`, `creationPolicy` for controlling resource lifecycle
* [Templates](./templates.md) — `ternary` and conditional template expressions
