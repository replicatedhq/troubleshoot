# LLM Analyzer Demo Setup

## Prerequisites
1. OpenAI API Key from https://platform.openai.com/api-keys
2. Built binaries (`make build`)

## Quick Demo (No Kubernetes Required)

### 1. Create a Test Support Bundle
```bash
# Create a simple test bundle with OOM errors
./demo/quick-demo.sh --create-bundle-only
```

### 2. Set Your API Key
```bash
export OPENAI_API_KEY="sk-your-actual-key-here"
```

### 3. Run Analysis
```bash
# Analyze the test bundle
./bin/analyze test-bundle.tar.gz \
  --analyzers examples/analyzers/llm-analyzer.yaml
```

## Real Kubernetes Cluster Demo

### 1. Set Your API Key
```bash
export OPENAI_API_KEY="sk-your-actual-key-here"
```

### 2. Create and Analyze Support Bundle
```bash
# This collects from your cluster and analyzes in one step
./bin/support-bundle examples/analyzers/llm-analyzer.yaml \
  --problem-description "My pods are crashing with OOM errors" \
  --interactive=false
```

## What You'll See

With a valid API key, you'll get output like:
```
Fail: AI-Powered Problem Analysis
 Critical issue detected: Memory exhaustion in Java application
 Root Cause: Heap space insufficient for workload
 Solution: Increase memory limits to 2Gi
 Commands: kubectl patch deployment...
```

## Troubleshooting

1. **No output**: Check that OPENAI_API_KEY is set correctly
2. **API errors**: Verify your API key has credits/is active
3. **Build errors**: Run `make build` first

## Cost
Each analysis costs approximately $0.01-$0.10 depending on log size.