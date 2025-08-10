#!/bin/bash

# Deploy a failing demo application for LLM analyzer demonstration
set -e

echo "========================================="
echo "Deploying Demo Application with Issues"
echo "========================================="
echo

# Create namespace
echo "Creating namespace 'demo-app'..."
kubectl create namespace demo-app --dry-run=client -o yaml | kubectl apply -f -

# Deploy web application with database connection issues
echo "Deploying web application (will fail due to missing database)..."
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
  namespace: demo-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
      - name: app
        image: busybox
        command: ["/bin/sh", "-c"]
        args:
        - |
          echo "Starting web application..."
          sleep 2
          echo "Connecting to database at postgres://db:5432"
          sleep 1
          echo "ERROR: Connection refused - database unreachable"
          echo "FATAL: Cannot start without database"
          exit 1
EOF

# Deploy database with memory issues
echo "Deploying database (will OOMKill due to memory limits)..."
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: database
  namespace: demo-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: db
  template:
    metadata:
      labels:
        app: db
    spec:
      containers:
      - name: postgres
        image: busybox
        command: ["/bin/sh", "-c"]
        args:
        - |
          echo "PostgreSQL 14.2 starting..."
          echo "Allocating shared memory..."
          echo "ERROR: Cannot allocate memory"
          echo "HINT: Container memory limit too low"
          echo "FATAL: OOMKilled"
          exit 137
        resources:
          limits:
            memory: "10Mi"
          requests:
            memory: "5Mi"
EOF

echo
echo "Waiting for pods to start failing..."
sleep 5

echo
echo "Current pod status:"
kubectl get pods -n demo-app

echo
echo "Recent events:"
kubectl get events -n demo-app --sort-by='.lastTimestamp' | head -10

echo
echo "========================================="
echo "Demo app deployed with issues:"
echo "- Web app: Cannot connect to database"
echo "- Database: OOMKilled due to low memory"
echo "========================================="