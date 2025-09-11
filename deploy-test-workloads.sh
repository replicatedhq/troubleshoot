#!/bin/bash

echo "🚀 Deploying Test Workloads to K3s Cluster"
echo "=========================================="

# Check if we're connected to a cluster
if ! kubectl cluster-info &>/dev/null; then
    echo "❌ No Kubernetes cluster connection found"
    echo "💡 Make sure you're in the replicated cluster shell"
    exit 1
fi

echo "✅ Connected to cluster:"
kubectl get nodes

echo ""
echo "📦 Deploying problematic workloads..."

# Deploy memory pressure app (will cause OOMKills)
echo "1. 💾 Deploying memory-hungry app (will cause OOMKills)..."
kubectl apply -f test-workloads/memory-pressure-app.yaml
if [ $? -eq 0 ]; then
    echo "   ✅ Memory pressure app deployed"
else
    echo "   ❌ Failed to deploy memory pressure app"
fi

# Deploy crashloop app (missing dependencies)
echo "2. 🔄 Deploying backend API app (will be in CrashLoopBackOff)..."
kubectl apply -f test-workloads/crashloop-app.yaml
if [ $? -eq 0 ]; then
    echo "   ✅ Backend API app deployed"
else
    echo "   ❌ Failed to deploy backend API app"
fi

# Deploy CPU intensive app (scheduling issues)
echo "3. 🔥 Deploying CPU-intensive app (will have scheduling issues)..."
kubectl apply -f test-workloads/cpu-intensive-app.yaml
if [ $? -eq 0 ]; then
    echo "   ✅ CPU-intensive app deployed"
else
    echo "   ❌ Failed to deploy CPU-intensive app"
fi

echo ""
echo "⏳ Waiting for pods to start (and fail)..."
sleep 30

echo ""
echo "📊 Current pod status:"
kubectl get pods -o wide

echo ""
echo "📋 Recent events:"
kubectl get events --sort-by=.metadata.creationTimestamp -o custom-columns="TIME:.metadata.creationTimestamp,TYPE:.type,REASON:.reason,MESSAGE:.message" | tail -10

echo ""
echo "🎯 Expected issues to observe:"
echo "   1. 💾 memory-hungry-app pods: OOMKilled due to memory limits"
echo "   2. 🔄 backend-api pods: CrashLoopBackOff due to missing dependencies" 
echo "   3. 🔥 cpu-intensive-app pods: Some pending due to resource constraints"

echo ""
echo "⏰ Wait 2-3 minutes for issues to manifest, then collect support bundle:"
echo "   kubectl support-bundle real-cluster-troubleshoot.yaml"

echo ""
echo "🧠 Or use our intelligent analysis:"
echo "   ./run-real-cluster-analysis.sh"
