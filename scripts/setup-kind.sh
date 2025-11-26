#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Kind Setup Script for Lynq E2E Tests ===${NC}"
echo ""

# Default values
CLUSTER_NAME="${KIND_CLUSTER:-lynq-test-e2e}"
K8S_VERSION="${KIND_K8S_VERSION:-v1.28.3}"
CERT_MANAGER_VERSION="${CERT_MANAGER_VERSION:-v1.16.3}"

# Check if kind is installed
if ! command -v kind &> /dev/null; then
    echo -e "${RED}Error: kind is not installed${NC}"
    echo ""
    echo "Please install kind first:"
    echo "  macOS: brew install kind"
    echo "  Linux: curl -Lo ./kind https://kind.sigs.k8s.io/dl/latest/kind-linux-amd64"
    exit 1
fi

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}Error: kubectl is not installed${NC}"
    echo ""
    echo "Please install kubectl first:"
    echo "  macOS: brew install kubectl"
    echo "  Linux: curl -LO https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
    exit 1
fi

# Display configuration
echo -e "${YELLOW}Configuration:${NC}"
echo "  Cluster Name:   $CLUSTER_NAME"
echo "  K8s Version:    $K8S_VERSION"
echo "  Cert-Manager:   $CERT_MANAGER_VERSION"
echo ""

# Check if cluster already exists
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    echo -e "${YELLOW}Kind cluster '${CLUSTER_NAME}' already exists${NC}"
    echo -e "${YELLOW}Using existing cluster${NC}"
    kubectl config use-context "kind-${CLUSTER_NAME}"
else
    # Create kind cluster
    echo -e "${YELLOW}Creating kind cluster '${CLUSTER_NAME}'...${NC}"
    
    # Create kind config with extra port mappings if needed
    cat <<EOF | kind create cluster --name "${CLUSTER_NAME}" --image "kindest/node:${K8S_VERSION}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
EOF

    echo -e "${GREEN}✓ Kind cluster created${NC}"
fi

# Set kubectl context
kubectl config use-context "kind-${CLUSTER_NAME}"
echo -e "${GREEN}✓ Kubectl context set to 'kind-${CLUSTER_NAME}'${NC}"

# Wait for cluster to be ready
echo -e "${YELLOW}Waiting for cluster to be ready...${NC}"
kubectl wait --for=condition=Ready nodes --all --timeout=300s
echo -e "${GREEN}✓ Cluster is ready${NC}"

# Install cert-manager
echo ""
echo -e "${YELLOW}Installing cert-manager...${NC}"
kubectl apply -f "https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml"
echo -e "${GREEN}✓ cert-manager manifests applied${NC}"

# Wait for cert-manager to be ready
echo -e "${YELLOW}Waiting for cert-manager to be ready...${NC}"
kubectl wait --namespace cert-manager \
    --for=condition=ready pod \
    --selector=app.kubernetes.io/instance=cert-manager \
    --timeout=300s
echo -e "${GREEN}✓ cert-manager is ready${NC}"

# Wait for cert-manager webhook to be fully functional
echo -e "${YELLOW}Waiting for cert-manager webhook to be fully ready...${NC}"
kubectl wait pod \
    -l app.kubernetes.io/name=webhook \
    --for condition=Ready \
    --namespace cert-manager \
    --timeout=2m

kubectl wait pod \
    -l app.kubernetes.io/name=cainjector \
    --for condition=Ready \
    --namespace cert-manager \
    --timeout=2m

# Wait for caBundle injection
echo -e "${YELLOW}Waiting for cert-manager webhook caBundle injection...${NC}"
for i in {1..60}; do
    CA_BUNDLE=$(kubectl get validatingwebhookconfiguration cert-manager-webhook -o jsonpath='{.webhooks[0].clientConfig.caBundle}' 2>/dev/null || echo "")
    if [ -n "$CA_BUNDLE" ]; then
        echo -e "${GREEN}✓ caBundle injected${NC}"
        break
    fi
    if [ $i -eq 60 ]; then
        echo -e "${RED}Error: caBundle was not injected${NC}"
        exit 1
    fi
    sleep 2
done

echo ""
echo -e "${GREEN}=== Kind Cluster Setup Complete ===${NC}"
echo ""
echo -e "${BLUE}Cluster Information:${NC}"
echo "  Cluster Name:   $CLUSTER_NAME"
echo "  Context:        $(kubectl config current-context)"
echo "  Kubernetes:     $(kubectl version --output=json 2>/dev/null | jq -r .serverVersion.gitVersion || kubectl version --short 2>/dev/null | grep Server)"
echo "  Nodes:          $(kubectl get nodes --no-headers | wc -l | tr -d ' ')"
echo ""
echo -e "${BLUE}Useful Commands:${NC}"
echo "  Check cluster:       kind get clusters"
echo "  Delete cluster:      ./scripts/cleanup-kind.sh"
echo "  Load image:          kind load docker-image <image> --name ${CLUSTER_NAME}"
echo ""
