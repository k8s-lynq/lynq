#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== Kind Cleanup Script for Lynq E2E Tests ===${NC}"
echo ""

# Default values
CLUSTER_NAME="${KIND_CLUSTER:-lynq-test-e2e}"

# Check if kind is installed
if ! command -v kind &> /dev/null; then
    echo -e "${RED}Error: kind is not installed${NC}"
    exit 1
fi

# Check if cluster exists
if ! kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    echo -e "${YELLOW}Kind cluster '${CLUSTER_NAME}' does not exist${NC}"
    echo -e "${GREEN}Nothing to clean up${NC}"
    exit 0
fi

echo -e "${YELLOW}Found kind cluster '${CLUSTER_NAME}'${NC}"
echo ""

# Optional: Clean up resources before deleting cluster
if command -v kubectl &> /dev/null; then
    # Set context to the cluster
    if kubectl config use-context "kind-${CLUSTER_NAME}" &> /dev/null; then
        echo -e "${YELLOW}Cleaning up test namespaces...${NC}"
        
        # Delete common test namespaces
        for ns in policy-test lynq-test lynq-system; do
            if kubectl get namespace "$ns" &> /dev/null; then
                echo "  Deleting namespace: $ns"
                kubectl delete namespace "$ns" --wait=false --ignore-not-found=true 2>/dev/null || true
            fi
        done
        
        echo -e "${YELLOW}Cleaning up Lynq CRDs...${NC}"
        kubectl delete crd lynqnodes.operator.lynq.sh --ignore-not-found=true 2>/dev/null || true
        kubectl delete crd lynqforms.operator.lynq.sh --ignore-not-found=true 2>/dev/null || true
        kubectl delete crd lynqhubs.operator.lynq.sh --ignore-not-found=true 2>/dev/null || true
        
        echo -e "${GREEN}✓ Resources cleanup initiated${NC}"
    fi
fi

# Delete the kind cluster
echo ""
echo -e "${YELLOW}Deleting kind cluster '${CLUSTER_NAME}'...${NC}"
kind delete cluster --name "${CLUSTER_NAME}"
echo -e "${GREEN}✓ Kind cluster deleted${NC}"

# Clean up kubectl context
if command -v kubectl &> /dev/null; then
    echo -e "${YELLOW}Cleaning up kubectl context...${NC}"
    kubectl config delete-context "kind-${CLUSTER_NAME}" 2>/dev/null && echo -e "${GREEN}✓ Context removed${NC}" || echo -e "${YELLOW}Context not found or already removed${NC}"
    kubectl config delete-cluster "kind-${CLUSTER_NAME}" 2>/dev/null && echo -e "${GREEN}✓ Cluster config removed${NC}" || echo -e "${YELLOW}Cluster config not found or already removed${NC}"
fi

echo ""
echo -e "${GREEN}=== Cleanup Complete ===${NC}"
echo ""
echo "To create a new cluster, run:"
echo "  ./scripts/setup-kind.sh"
echo ""
