# LLM Analyzer Demo Walkthrough

This guide walks through demonstrating the new AI-powered LLM analyzer feature in Troubleshoot.sh.

## Prerequisites

Before starting the demo, ensure you have:
- A Kubernetes cluster (Kind, Minikube, or any cluster)
- kubectl configured to access your cluster
- The Troubleshoot project built locally (`make build`)
- An OpenAI API key in a `.env` file

## Introduction (1 minute)

**Key points:**
- No need to anticipate every failure mode
- AI understands context and correlations
- Works with any application, not just pre-configured scenarios
- Uses cost-effective models (gpt-4o-mini by default)

## Part 1: Setup (2 minutes)

### Build the project (if not already done)

```bash
# Build the Troubleshoot binaries
make build

# Verify the binaries exist
ls -la ./bin/support-bundle ./bin/analyze
```

### Set your OpenAI API key (if not already done)

Troubleshoot.sh now automatically reads `.env` files in the current directory, following modern CLI tool conventions.

### Create a test cluster (if needed)

```bash
# If you don't have a cluster, create one with Kind
kind create cluster --name demo-cluster
```

## Part 2: Deploy a Failing Application (3 minutes)

### Deploy the demo application

```bash
# Run the deployment script
./demo-app-deploy.sh
```

This script will:
- Create a namespace called `demo-app`
- Deploy a web application that fails to connect to the database
- Deploy a database that gets OOMKilled due to memory limits
- Show the failing pods and recent events

### Verify the problems

```bash
# Check pod status - you'll see CrashLoopBackOff
kubectl get pods -n demo-app

# Optional: Check logs to see the errors
kubectl logs -n demo-app -l app=web --tail=10
kubectl logs -n demo-app -l app=db --tail=10
```

## Part 3: Collect & Analyze with LLM (4 minutes)

### Create the support bundle specification

```bash
cat <<EOF > demo-support-bundle.yaml
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: demo-app-troubleshoot
spec:
  collectors:
    - clusterInfo: {}
    - clusterResources: {}
    - logs:
        name: demo-logs
        namespace: demo-app
        limits:
          maxLines: 1000
    - events:
        namespace: demo-app

  analyzers:
    # AI-Powered Analyzer
    - llm:
        checkName: "AI Diagnostic Analysis"
        collectorName: "demo-logs"
        fileName: "**/*.log"
        model: "gpt-4o-mini"
        maxFiles: 10
        priorityPatterns:
          - "error"
          - "fatal"
          - "failed"
          - "OOM"
        outcomes:
          - fail:
              when: "issue_found"
              message: |
                AI Analysis Found Critical Issues:
                {{.Summary}}

                Root Cause: {{.RootCause}}
                Affected Components: {{.AffectedPods}}
          - pass:
              message: "No critical issues detected"

    # Traditional analyzer for comparison
    - deploymentStatus:
        name: web-app
        namespace: demo-app
        outcomes:
          - fail:
              when: "< 1"
              message: "Web app deployment has no ready replicas"
          - pass:
              message: "Web app deployment is running"
EOF
```

### Run the support bundle collection

```bash
# Run with a problem description
./bin/support-bundle demo-support-bundle.yaml \
  --problem-description "Application is not starting and keeps crashing"
```

**Alternative: Interactive mode**
```bash
# Or use interactive mode to be prompted
./bin/support-bundle demo-support-bundle.yaml --interactive
# When prompted, type: "My application won't start and the database keeps restarting"
```

**Note:** End users would use `kubectl support-bundle` after installing the plugin, but we're using the local binary.

## Part 4: Re-analyze an Existing Bundle (3 minutes)

### Create an analyzer-only specification

```bash
cat <<EOF > reanalyze.yaml
apiVersion: troubleshoot.sh/v1beta2
kind: Analyzer
metadata:
  name: focused-reanalysis
spec:
  analyzers:
    - llm:
        checkName: "Memory Issue Deep Dive"
        collectorName: "demo-logs"
        fileName: "**/*.log"
        model: "gpt-4o-mini"
        problemDescription: "Focus on memory and resource issues only"
        outcomes:
          - fail:
              when: "issue_found"
              message: |
                Memory Analysis Results:
                {{.Summary}}

                Recommendations: {{.Solution}}
          - pass:
              message: "No memory issues found"
EOF
```

### Re-analyze the existing bundle

```bash
# Use the bundle we just created (adjust filename as needed)
./bin/analyze support-bundle-*.tar.gz \
  --analyzers reanalyze.yaml
```

## Cleanup (30 seconds)

```bash
# Remove the demo application
kubectl delete namespace demo-app

# Remove temporary files
rm demo-support-bundle.yaml reanalyze.yaml security-check.yaml

# Delete the Kind cluster
kind delete cluster --name demo-cluster
```
