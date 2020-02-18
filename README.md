# Replicated Troubleshoot

Replicated Troubleshoot is a framework for collecting, redacting and analyzing highly customizable diagnostic information about a Kubernetes cluster. Troubleshoot specs are created by 3rd-party application developers/maintainers and run by cluster operators in the initial and ongoing operation of those applications.

Troubleshoot provides two CLI tools as kubectl plugins (using [Krew](https://krew.dev)) `kubectl preflight` and `kubectl support-bundle`. Preflight provides pre-installation cluster conformance testing and validation (preflight checks) and support-bundle provides post-installation troubleshooting and diagnostics (support bundles).

# Development Environment Setup

1. Ensure that you have `go` installed
2. Ensure that your PATH is set to include the GOPATH in your `.bashrc` file. For example: 
```
export GOPATH=/home/username/go
export PATH="${KREW_ROOT:-$HOME/.krew}/bin:$GOPATH:$PATH"
```

# Testing Troubleshoot Locally

1. Run `make support-bundle`
2. Copy `./bin/support-bundle` to `kotsamd/operator` folder
3. In docker.skaffold, uncomment # COPY ./support-bundle /root/.krew/bin/kubectl-support_bundle

After these steps, the operator will restart (enabling local verification). 

# Using Replicated Troubleshoot (Non-Development Guide)

## Preflight Checks
Preflight checks are an easy-to-run set of conformance tests that can be written to verify that specific requirements in a cluster are met.

To run a sample preflight check from a sample application, install the preflight kubectl plugin:

```shell
curl https://krew.sh/preflight | bash
```
 and run:
 
```shell
kubectl preflight https://preflight.replicated.com
```

For a details on creating the custom resource files that drive preflight checks, visit [creating preflight checks](https://troubleshoot.sh/docs/preflight/creating/).


## Support Bundle
A support bundle is an archive that's created in-cluster, by collecting logs, cluster information and executing specified commands (including redaction of sensitive information). After creating a support bundle, the cluster operator will normally deliver it to the 3rd-party application vendor for analysis and disconnected debugging. Another Replicated project, [Kotsadm](https://github.com/replicatedhq/kotsadm), provides cluster operators with in-cluster UI for processing support bundles and viewing analyzers (as well as support bundle collection).

To collect a sample support bundle, install the troubleshoot kubectl plugin:

```shell
curl https://krew.sh/support-bundle | bash
```
 and run:
 
```shell
kubectl support-bundle https://troubleshoot.replicated.com
```
For details on creating the custom resource files that drive support-bundle collection [creating collectors](https://troubleshoot.sh/reference/collectors/overview/) and for analyzers [creating analyzers](https://troubleshoot.sh/reference/analyzers/overview/).

# Community

For questions about using Troubleshoot, there's a [Replicated Community](https://help.replicated.com/community) forum, and a [#app-troubleshoot channel in Kubernetes Slack](https://kubernetes.slack.com/channels/app-troubleshoot).




