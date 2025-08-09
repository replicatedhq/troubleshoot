# Troubleshoot LLM Analyzer Test Cluster

This directory contains scripts and configurations to set up a Kubernetes test cluster with intentionally broken applications to test the LLM analyzer.

## Prerequisites

- Docker Desktop or Docker Engine
- kubectl
- Kind (will be installed by setup script on macOS)
- Helm (will be installed by setup script on macOS)

## Quick Start

1. **Create the test cluster:**
   ```bash
   ./setup.sh
   ```

2. **Collect a support bundle:**
   ```bash
   kubectl support-bundle collector-spec.yaml
   ```

3. **Clean up when done:**
   ```bash
   ./cleanup.sh
   ```

## Test Scenarios

The cluster includes three failing applications:

### 1. OOMKilled Pod (test-oom namespace)
- Deployment: `memory-hog`
- Issue: Container uses more memory than its limit (50Mi)
- Expected: Pod will be OOMKilled repeatedly

### 2. CrashLoopBackOff (test-crash namespace)
- Deployment: `crash-loop-app`
- Issue: Application exits with error code 1 after 5 seconds
- Expected: Pods will be in CrashLoopBackOff state

### 3. Connection Issues (test-connection namespace)
- Deployment: `connection-test-app`
- Issue: Wrong database credentials in ConfigMap
- Expected: Continuous connection error logs

## Files

- `kind-config.yaml` - Kind cluster configuration
- `collector-spec.yaml` - Troubleshoot collector specification
- `setup.sh` - Creates cluster and deploys test scenarios
- `cleanup.sh` - Deletes the cluster
- `test-scenarios/` - YAML files for broken applications

## Testing the LLM Analyzer

Once the LLM analyzer is implemented, you can test it against this cluster:

1. Run the setup script to create the cluster with issues
2. Use the troubleshoot CLI with the LLM analyzer enabled
3. Provide problem descriptions like:
   - "My pods keep restarting"
   - "Application can't connect to database"
   - "Pods are being killed"
4. Verify the LLM correctly identifies the issues

## Troubleshooting

If the setup script fails:
- Ensure Docker is running
- Check that ports 80 and 443 are not in use
- Try running `./cleanup.sh` first to remove any existing cluster