#!/bin/bash
set -e

echo "========================================"
echo "Cleaning up Troubleshoot Test Cluster"
echo "========================================"

# Delete the Kind cluster
echo "Deleting Kind cluster 'troubleshoot-test'..."
kind delete cluster --name troubleshoot-test

echo ""
echo "Cluster deleted successfully!"
echo ""
echo "To recreate the cluster, run:"
echo "  ./setup.sh"