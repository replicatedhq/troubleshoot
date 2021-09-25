# Replicated Troubleshoot

Replicated Troubleshoot is a framework for collecting, redacting, and analyzing highly customizable diagnostic information about a Kubernetes cluster. Troubleshoot specs are created by 3rd-party application developers/maintainers and run by cluster operators in the initial and ongoing operation of those applications.

Troubleshoot provides two CLI tools as kubectl plugins (using [Krew](https://krew.dev)): `kubectl preflight` and `kubectl support-bundle`. Preflight provides pre-installation cluster conformance testing and validation (preflight checks) and support-bundle provides post-installation troubleshooting and diagnostics (support bundles).

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

For more details on creating the custom resource files that drive preflight checks, visit [creating preflight checks](https://troubleshoot.sh/docs/preflight/introduction/).


## Support Bundle
A support bundle is an archive that's created in-cluster, by collecting logs and cluster information, and executing specified commands (including redaction of sensitive information). After creating a support bundle, the cluster operator will normally deliver it to the 3rd-party application vendor for analysis and disconnected debugging. Another Replicated project, [KOTS](https://github.com/replicatedhq/kots), provides k8s apps an in-cluster UI for processing support bundles and viewing analyzers (as well as support bundle collection).

To collect a sample support bundle, install the troubleshoot kubectl plugin:

```shell
curl https://krew.sh/support-bundle | bash
```
 and run:
 
```shell
kubectl support-bundle https://support-bundle.replicated.com
```
For more details on creating the custom resource files that drive support-bundle collection, visit [creating collectors](https://troubleshoot.sh/docs/collect/) and [creating analyzers](https://troubleshoot.sh/docs/analyze/).

# Community

For questions about using Troubleshoot, there's a [Replicated Community](https://help.replicated.com/community) forum, and a [#app-troubleshoot channel in Kubernetes Slack](https://kubernetes.slack.com/channels/app-troubleshoot).

# Software Bill of Materials 
A signed SBOM  that includes Troubleshoot dependencies is included in each release. 
- **troubleshoot-sbom.tgz** contains a software bill of materials for Troubleshoot. 
- **troubleshoot-sbom.tgz.sig** is the digital signature for troubleshoot-sbom.tgz
- **key.pub** is the public key from the key pair used to sign troubleshoot-sbom.tgz

The following example illustrates using [cosign](https://github.com/sigstore/cosign) to verify that **troubleshoot-sbom.tgz** has
not been tampered with.
```shell
$ cosign verify-blob -key key.pub -signature troubleshoot-sbom.tgz.sig troubleshoot-sbom.tgz
Verified OK
```