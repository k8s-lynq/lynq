# Step 4: Deploy Lynq Operator

Now let's deploy the Lynq operator that will manage our existing resources.

## Create Lynq Namespace

```bash
kubectl create namespace lynq-system
```{{exec}}

## Add the Helm Repository

```bash
helm repo add lynq https://k8s-lynq.github.io/lynq
helm repo update
```{{exec}}

## Install Lynq Operator

Deploy the operator with Helm:

```bash
helm install lynq lynq/lynq --namespace lynq-system
```{{exec}}

Wait for the operator to be ready:

```bash
kubectl wait --for=condition=Available --timeout=300s -n lynq-system \
  deployment/lynq-controller-manager
```{{exec}}

## Verify Installation

Check that the operator is running:

```bash
kubectl get pods -n lynq-system
```{{exec}}

You should see the `lynq-controller-manager` pod in `Running` state.

## Verify CRDs

Check the installed Custom Resource Definitions:

```bash
kubectl get crds | grep lynq
```{{exec}}

You should see:
- `lynqhubs.operator.lynq.sh` - Database connection configuration
- `lynqforms.operator.lynq.sh` - Resource templates
- `lynqnodes.operator.lynq.sh` - Individual node instances

## Verify Existing ConfigMaps Still Exist

Before we proceed, confirm our existing ConfigMaps are still there, unchanged:

```bash
kubectl get configmaps -n lynq-demo -l team=platform
```{{exec}}

Check one of them to verify the old values are still present:

```bash
kubectl get configmap app-alpha-config -n lynq-demo -o jsonpath='{.data.DATABASE_URL}'; echo ""
```{{exec}}

Should show: `postgres://old-db:5432/alpha`

**This is about to change when Lynq takes over!**

✅ **Checkpoint**:
- Lynq operator is running
- Existing ConfigMaps still have their original values
- Ready to adopt resources with Lynq

Click **Continue** to create the LynqHub.
