#!/bin/bash
set -e

echo "========================================"
echo "Setting up Troubleshoot Test Cluster"
echo "========================================"

# Check if kind is installed
if ! command -v kind &> /dev/null; then
    echo "Installing Kind..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        brew install kind
    else
        echo "Please install Kind manually: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
        exit 1
    fi
fi

# Check if helm is installed
if ! command -v helm &> /dev/null; then
    echo "Installing Helm..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        brew install helm
    else
        echo "Please install Helm manually: https://helm.sh/docs/intro/install/"
        exit 1
    fi
fi

# Delete existing cluster if it exists
kind delete cluster --name troubleshoot-test 2>/dev/null || true

# Create new cluster
echo "Creating Kind cluster..."
kind create cluster --name troubleshoot-test --config kind-config.yaml

# Wait for cluster to be ready
echo "Waiting for cluster to be ready..."
kubectl wait --for=condition=Ready nodes --all --timeout=60s

# Add helm repositories
echo "Adding Helm repositories..."
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update

# Install PostgreSQL
echo "Installing PostgreSQL..."
helm install postgres bitnami/postgresql \
  --set auth.postgresPassword=correctpass \
  --set auth.database=myapp \
  --set primary.persistence.size=1Gi \
  --wait --timeout 2m

# Install Redis
echo "Installing Redis..."
helm install redis bitnami/redis \
  --set auth.enabled=false \
  --set master.persistence.size=1Gi \
  --wait --timeout 2m

# Install nginx ingress (optional, but useful for testing)
echo "Installing nginx-ingress..."
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=90s

# Deploy test scenarios
echo "Deploying test scenarios..."
kubectl apply -f test-scenarios/oom-pod.yaml
kubectl apply -f test-scenarios/crash-loop.yaml
kubectl apply -f test-scenarios/connection-issue.yaml

# Wait a bit for pods to start failing
echo "Waiting for test scenarios to manifest issues..."
sleep 20

# Show status
echo ""
echo "========================================"
echo "Test Cluster Setup Complete!"
echo "========================================"
echo ""
echo "Cluster Status:"
kubectl get nodes
echo ""
echo "Test Namespaces:"
kubectl get namespaces | grep test-
echo ""
echo "Problem Pods:"
kubectl get pods -n test-oom
kubectl get pods -n test-crash
kubectl get pods -n test-connection
echo ""
echo "To collect a support bundle, run:"
echo "  kubectl support-bundle collector-spec.yaml"
echo ""
echo "To delete the cluster, run:"
echo "  ./cleanup.sh"