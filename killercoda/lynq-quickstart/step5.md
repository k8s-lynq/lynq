# Step 5: Create LynqForm Template

Now let's create a **LynqForm** that defines what Kubernetes resources to create for each tenant.

## Understanding LynqForm

A LynqForm:
- Defines resource blueprints (Deployments, Services, etc.)
- Uses Go templates with 200+ Sprig functions
- Supports resource dependencies
- Controls lifecycle policies (creation, deletion, conflict handling)

## Create the LynqForm

```bash
cat <<'EOF' | kubectl apply -f -
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: webapp-template
  namespace: lynq-demo
spec:
  hubId: tenant-hub

  # ConfigMap for tenant-specific configuration
  configMaps:
    - id: config
      nameTemplate: "{{ .uid }}-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        metadata:
          labels:
            app: "{{ .uid }}"
            lynq.sh/tenant: "{{ .uid }}"
        data:
          TENANT_ID: "{{ .uid }}"
          TENANT_URL: "{{ .tenantUrl }}"
          PLAN: "{{ .plan }}"

  # Deployment for the tenant application
  deployments:
    - id: app
      nameTemplate: "{{ .uid }}-app"
      dependIds: [config]
      waitForReady: true
      timeoutSeconds: 120
      spec:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          labels:
            app: "{{ .uid }}"
            lynq.sh/tenant: "{{ .uid }}"
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: "{{ .uid }}"
          template:
            metadata:
              labels:
                app: "{{ .uid }}"
            spec:
              containers:
                - name: nginx
                  image: nginx:alpine
                  ports:
                    - containerPort: 80
                  envFrom:
                    - configMapRef:
                        name: "{{ .uid }}-config"
                  resources:
                    requests:
                      cpu: 50m
                      memory: 64Mi
                    limits:
                      cpu: 200m
                      memory: 128Mi

  # Service to expose the tenant application
  services:
    - id: svc
      nameTemplate: "{{ .uid }}-svc"
      dependIds: [app]
      spec:
        apiVersion: v1
        kind: Service
        metadata:
          labels:
            app: "{{ .uid }}"
            lynq.sh/tenant: "{{ .uid }}"
        spec:
          type: ClusterIP
          selector:
            app: "{{ .uid }}"
          ports:
            - port: 80
              targetPort: 80
EOF
```{{exec}}

## Watch Resources Being Created

The operator will now create resources for each active tenant. Watch the magic happen:

```bash
kubectl get lynqnodes -n lynq-demo -w
```{{exec}}

> Press `Ctrl+C` to stop watching

## Verify Created Resources

Check the LynqNode CRs (one per active tenant):

```bash
kubectl get lynqnodes -n lynq-demo
```{{exec}}

Check the tenant resources:

```bash
kubectl get configmaps,deployments,services -n lynq-demo -l lynq.sh/tenant
```{{exec}}

You should see resources for `acme-corp` and `beta-inc`, but **not** for `gamma-ltd` (inactive).

## View Tenant Deployment Details

```bash
kubectl get deployments -n lynq-demo -l lynq.sh/tenant -o wide
```{{exec}}

Each tenant has 1 replica running.

✅ **Checkpoint**:
- LynqForm template created
- Resources automatically provisioned for 2 active tenants
- Inactive tenant (`gamma-ltd`) was skipped

Click **Continue** to test the lifecycle.
