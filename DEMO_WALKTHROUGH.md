# LLM Analyzer Demo Walkthrough

This guide walks through demonstrating the new AI-powered LLM analyzer feature in Troubleshoot.sh.

## Prerequisites

Before starting the demo, ensure you have:
- A Kubernetes cluster (Kind, Minikube, or any cluster)
- kubectl configured to access your cluster
- The Troubleshoot project built locally (`make build`)
- An OpenAI API key

## Introduction (1 minute)

**What to say:**
> "Troubleshoot.sh now includes an AI-powered analyzer that uses Large Language Models to automatically identify issues in Kubernetes applications. Instead of writing complex rules or patterns, the LLM analyzer reads your logs, understands the context, and identifies root causes automatically."

**Key points:**
- No need to anticipate every failure mode
- AI understands context and correlations
- Works with any application, not just pre-configured scenarios
- Uses cost-effective models (gpt-4o-mini by default)

## Part 1: Setup (2 minutes)

**What to say:**
> "For this demo, I'm using the development build of Troubleshoot.sh with our new LLM analyzer. End users would install this as a kubectl plugin, but we're using the local binary to showcase the latest features."

### Build the project (if not already done)

```bash
# Build the Troubleshoot binaries
make build

# Verify the binaries exist
ls -la ./bin/support-bundle ./bin/analyze
```

### Set your OpenAI API key

```bash
# Set the API key as an environment variable
export OPENAI_API_KEY="sk-..."

# Or if you have a .env file
source .env

# Verify it's set
echo "API key configured: ${OPENAI_API_KEY:0:10}..."
```

### Create a test cluster (if needed)

```bash
# If you don't have a cluster, create one with Kind
kind create cluster --name demo-cluster
```

**What to say:**
> "That's all the setup required. The LLM analyzer is now ready to use."

## Part 2: Deploy a Failing Application (3 minutes)

**What to say:**
> "Let's deploy a sample application that has multiple issues - the kind of complex scenario that's difficult to debug manually."

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

**What to say:**
> "We have multiple pods failing with different issues. The web app can't connect to the database, and the database is being OOMKilled. In a real scenario, these could be spread across many namespaces with hundreds of log lines."

## Part 3: Collect & Analyze with LLM (4 minutes)

**What to say:**
> "Now let's use Troubleshoot.sh with the LLM analyzer to automatically identify these issues."

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
        fileName: "*"
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
        name: demo-app
        namespace: demo-app
        outcomes:
          - fail:
              when: "< 1"
              message: "Deployment has no ready replicas"
          - pass:
              message: "Deployment is running"
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

### Review the results

**Expected output:**
```
Analyzing support bundle...

FAIL: AI Diagnostic Analysis
AI Analysis Found Critical Issues:
Critical database and application failures detected. Database pod is experiencing 
OOMKilled events due to insufficient memory limits (10Mi), preventing it from 
starting. This causes the web application to fail as it cannot establish database 
connections.

Root Cause: Database container memory limit (10Mi) is insufficient for PostgreSQL 
startup, causing immediate OOMKill. This cascades to web app failures due to 
missing database dependency.
Affected Components: database-xxx, web-app-xxx, web-app-yyy

FAIL: Deployment has no ready replicas

Support bundle written to support-bundle-2024-12-10T120000.tar.gz
```

**What to say:**
> "Notice how the LLM analyzer not only identified both issues but also understood the relationship between them. It recognized that the database OOM is the root cause that triggers the cascade of web app failures. Traditional analyzers would only report symptoms."

## Part 4: Re-analyze an Existing Bundle (3 minutes)

**What to say:**
> "One powerful feature is the ability to re-analyze existing bundles with different contexts or questions."

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
        fileName: "*"
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

**Alternative approach with different question:**
```bash
# Create another analyzer focusing on different aspects
cat <<EOF > security-check.yaml
apiVersion: troubleshoot.sh/v1beta2
kind: Analyzer
metadata:
  name: security-analysis
spec:
  analyzers:
    - llm:
        checkName: "Security and Configuration Review"
        collectorName: "demo-logs"
        fileName: "*"
        problemDescription: "Are there any security misconfigurations or concerns?"
        outcomes:
          - warn:
              when: "issue_found"
              message: "Security considerations: {{.Summary}}"
          - pass:
              message: "No security issues identified"
EOF

./bin/analyze support-bundle-*.tar.gz \
  --analyzers security-check.yaml
```

**What to say:**
> "The same bundle can be re-analyzed from different perspectives. Support teams can ask specific questions without re-collecting data from the cluster."

## Cleanup (30 seconds)

```bash
# Remove the demo application
kubectl delete namespace demo-app

# Remove temporary files
rm demo-support-bundle.yaml reanalyze.yaml security-check.yaml
```

## Key Takeaways

**Summarize with these points:**

1. **Simple Setup**: Just need kubectl plugin and OpenAI API key
2. **No Configuration Required**: Works out of the box with sensible defaults
3. **Natural Language**: Describe problems in plain English
4. **Intelligent Analysis**: AI understands context and correlations
5. **Cost Effective**: Uses gpt-4o-mini by default (~$0.01 per analysis)
6. **Flexible**: Can re-analyze with different questions
7. **Powerful**: Identifies root causes, not just symptoms

**Closing statement:**
> "The LLM analyzer transforms Kubernetes troubleshooting from a manual, expertise-heavy process into an automated, intelligent analysis that anyone can use. It's like having a Kubernetes expert review all your logs and tell you exactly what's wrong."

## FAQ During Demo

**Q: What about sensitive data?**
A: Troubleshoot.sh already has redaction capabilities. Sensitive data is removed before sending to the LLM.

**Q: How much does it cost?**
A: With gpt-4o-mini, typically less than $0.01 per analysis. Most logs fit within a few thousand tokens.

**Q: Can it work with other AI providers?**
A: Currently OpenAI, but the architecture supports adding other providers.

**Q: What if I don't have internet access?**
A: You can use a proxy or private OpenAI deployment. Traditional analyzers still work offline.

**Q: How accurate is it?**
A: In testing, it identifies root causes that humans often miss, especially correlation between multiple issues.