# Replicated Troubleshoot

Replicated Troubleshoot is a framework for collecting, redacting, and analyzing highly customizable diagnostic information about a Kubernetes cluster. Troubleshoot specs are created by 3rd-party application developers/maintainers and run by cluster operators in the initial and ongoing operation of those applications.

Troubleshoot provides two CLI tools as kubectl plugins (using [Krew](https://krew.dev)): `kubectl preflight` and `kubectl support-bundle`. Preflight provides pre-installation cluster conformance testing and validation (preflight checks) and support-bundle provides post-installation troubleshooting and diagnostics (support bundles).

To know more about troubleshoot, please visit: https://troubleshoot.sh/

## Preflight Checks
Preflight checks are an easy-to-run set of conformance tests that can be written to verify that specific requirements in a cluster are met.

To run a sample preflight check from a sample application, install the preflight kubectl plugin:

```
curl https://krew.sh/preflight | bash
```
 and run, where https://preflight.replicated.com provides an **example** preflight spec:

```
kubectl preflight https://preflight.replicated.com
```

**NOTE** this is an example. Do **not** use to validate real scenarios.

For more details on creating the custom resource files that drive preflight checks, visit [creating preflight checks](https://troubleshoot.sh/docs/preflight/introduction/).


## Support Bundle
A support bundle is an archive that's created in-cluster, by collecting logs and cluster information, and executing specified commands (including redaction of sensitive information). After creating a support bundle, the cluster operator will normally deliver it to the 3rd-party application vendor for analysis and disconnected debugging. Another Replicated project, [KOTS](https://github.com/replicatedhq/kots), provides k8s apps an in-cluster UI for processing support bundles and viewing analyzers (as well as support bundle collection).

To collect a sample support bundle, install the troubleshoot kubectl plugin:

```
curl https://krew.sh/support-bundle | bash
```
 and run, where https://support-bundle.replicated.com provides an **example** support bundle spec:

```
kubectl support-bundle https://support-bundle.replicated.com
```

**NOTE** this is an example. Do **not** use to validate real scenarios.

For more details on creating the custom resource files that drive support-bundle collection, visit [creating collectors](https://troubleshoot.sh/docs/collect/) and [creating analyzers](https://troubleshoot.sh/docs/analyze/).

And see our other tool [sbctl](https://github.com/replicatedhq/sbctl) that makes it easier to interact with support bundles using `kubectl` commands you already know

# Community

For questions about using Troubleshoot, how to contribute and engaging with the project in any other way, please refer to the following resources and channels.

- [Replicated Community](https://help.replicated.com/community) forum
- [#app-troubleshoot channel in Kubernetes Slack](https://kubernetes.slack.com/channels/app-troubleshoot)
- [#Community meetings calendar](https://calendar.google.com/calendar/u/0?cid=Y19mMGx1aGhiZGtscGllOGo5dWpicXMwNnN1a0Bncm91cC5jYWxlbmRhci5nb29nbGUuY29t). This happen monthly but dates may change and would be kept upto date in the calendar.

# Software Bill of Materials
A signed SBOM  that includes Troubleshoot dependencies is included in each release.
- **troubleshoot-sbom.tgz** contains a software bill of materials for Troubleshoot.
- **troubleshoot-sbom.tgz.bundle** contains the signature and transparency log material used by Cosign.
- **key.pub** is the public key from the key pair used to sign troubleshoot-sbom.tgz

The following example illustrates using [cosign](https://github.com/sigstore/cosign) to verify that **troubleshoot-sbom.tgz** has
not been tampered with. Install [Cosign v3](https://github.com/sigstore/cosign/releases).
```sh
$ cosign verify-blob --key key.pub --bundle troubleshoot-sbom.tgz.bundle troubleshoot-sbom.tgz
Verified OK
```

# Build Provenance

SLSA build provenance attestations are generated for the `preflight` and `support-bundle` archives in each release. Download the archive and the corresponding `troubleshoot_<version>_provenance.sigstore.json` bundle, then use [Cosign v3.1.3 or later](https://github.com/sigstore/cosign/releases) to verify that it was built by this repository's release workflow:

```sh
cosign verify-blob-attestation \
  --bundle troubleshoot_v0.135.0_provenance.sigstore.json \
  --type https://slsa.dev/provenance/v1 \
  --certificate-identity "https://github.com/replicatedhq/troubleshoot/.github/workflows/release.yaml@refs/tags/v0.135.0" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  preflight_linux_amd64.tar.gz
```

Replace `v0.135.0` and `preflight_linux_amd64.tar.gz` with the release tag and downloaded archive being verified. The same provenance bundle verifies both `preflight` and `support-bundle` archives from that release.
