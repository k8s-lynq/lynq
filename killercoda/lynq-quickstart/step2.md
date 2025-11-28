# Step 2: Deploy Lynq Operator

Now let's deploy the Lynq operator using Helm.

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

Lynq installs three Custom Resource Definitions:

```bash
kubectl get crds | grep lynq
```{{exec}}

You should see:
- `lynqhubs.operator.lynq.sh` - Database connection configuration
- `lynqforms.operator.lynq.sh` - Resource templates
- `lynqnodes.operator.lynq.sh` - Individual node instances

## View Operator Logs (Optional)

You can watch the operator logs to see what it's doing:

```bash
kubectl logs -n lynq-system -l control-plane=controller-manager --tail=20
```{{exec}}

✅ **Checkpoint**: The Lynq operator is now running and ready to manage your infrastructure!

Click **Continue** to set up the MySQL database.
