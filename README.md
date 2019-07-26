# Replicated Troubleshoot

Replicated Troubleshoot is a CLI tool and a Kubernetes Operator that provides pre-installation cluster conformance testing and validation (preflight checks) and post-installation troubleshooting and diagnostics (support bundles).

## Preflight Checks
Preflight checks are an easy-to-run set of conformance tests that can be written to verify that specific requirements in a cluster are met.

To run a sample preflight check from a sample application, [install the preflight kubectl plugin](https://help.replicated.com/docs/troubleshoot/kubernetes/preflight/running-as-kubectl-plugin/) and run:

```shell
kubectl preflight https://preflight.replicated.com
```

For a full description of the supported preflight checks, visit the [docs](https://help.replicated.com/docs/troubleshoot/kubernetes/analysis/analysis-phase/).

## Support Bundle
A support bundle is an archive that's created in-cluster, by collecting logs, cluster information and executing various commands. After creating a support bundle, the cluster operator will normally deliver it to the application vendor for analysis and remote debugging.

To collect a sample support bundle, [install the troubleshoot kubectl plugin](/docs/troubleshoot/kubernetes/troubleshoot/running-as-kubectl-plugin/) and run:

```shell
kubectl troubleshoot https://troubleshoot.replicated.com
```
