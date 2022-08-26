# Provide a facility to automatically obtain updated specs

## Background

Troubleshoot is limited in that when folks write an update or a new spec for data collection and analysis, that upgrade is not available when Troubleshoot is invoked from within the kots application manager, or from specs stored in a file.  Updating the local file is simple enough, but updating the specs supplied by kots requires an upgrade to kots in order to collect updates to the default spec supplied by kots, and/or an update to the application which is deployed if the spec supplied by the application is changed.  This update requirement delays folks from accessing updated Troubleshoot specs and therefore discourages people from writing new/updated specs.

It is also difficult to justify the tight coupling of Troubleshoot configuration, which is intended to be an entirely client-side concept, and the application manager which is installed to a Kubernetes environment.

## Goals

* Provide the ability to automatically obtain a spec from a source that receives updates, whenever Troubleshoot is run
* Decouple the provision of default specs from the kots application manager, so that kots upgrades are not required in order to get access to updated specs

## Non Goals

* Maintain the ability for Troubleshoot to run in an airgapped environment
* maintain compatilbility with existing Troubleshoot specs

## High-Level Design

Add a new field to the Troubleshoot spec definition, which includes a URI used to locate the current spec online.

If the field is populated, Troubleshoot is to attempt to download the additional spec(s) from the source online, and add to the spec already provided.

Add a CLI flag to prevent any attmept to download, for use in airgap environments and/or when we simply do not want to download.

Update the [default spec included with kots](https://github.com/replicatedhq/kots/blob/main/pkg/supportbundle/defaultspec/spec.yaml) to include a URI pointing at [the troubleshoot-specs repo](https://raw.githubusercontent.com/replicatedhq/troubleshoot-specs/main/in-cluster/default.yaml).

URI's provided may be any of the URI types already accepted by Troubleshoot, including web addresses (https://), secrets, or files.

## Detailed Design

Current spec format is like this example:

```yaml
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: default
spec:
  collectors:
    - clusterInfo: {}
    - clusterResources: {}
  analyzers:
    - cephStatus: {}
    - longhorn: {}
```

We could add a new type, `specURIs`, which contains a list of URIs from which to obtain specs:

```yaml
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: default
spec:
  specURIs:
    - https://raw.githubusercontent.com/replicatedhq/troubleshoot-specs/main/in-cluster/default.yaml
  collectors:
    - clusterInfo: {}
    - clusterResources: {}
  analyzers:
    - cephStatus: {}
    - longhorn: {}
```

## Impact on kURL and kots

For kURL, each component/addon installed could include adding a URI pointing at a specific spec that 

## Limitations

Redactors are currently excluded from this proposal, though it would be desirable to add a similar feature in the future.

If a URI provided includes a spec which includes another URI, it is possible to get into some kind of recursion.  We should count the number of URI calls and limit that.

## Assumptions

Troubleshooot is able to accept multiple specs on invocation.

## Testing

## Alternatives Considered

* Some mechanism to trigger a process to update secrets stored in Kubernetes.  This was rejected due to the need to run a manual update spec for something installed in the cluster.
* Altering kots to provide a URL for the spec rather than storing it in code.  This was rejected to allow airgap compatibility.

## Security Considerations

Troubleshoot currently allows downloading from URL, and that pattern is frequently used from the CLI when customers are directed to do so.  If we made this change, customers would need to ensure that their firewall allows outbound connections to the URL target from the kots container running Troubleshoot.