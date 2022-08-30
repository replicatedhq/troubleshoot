# Provide a facility to automatically obtain updated specs

## Background

Troubleshoot is limited in that when folks write an update or a new spec for data collection and analysis, that upgrade is not available when Troubleshoot is invoked if specs for it are stored in a secret or a file which was deployed by an application.  Updating the local spec is simple enough, but may require an upgrade to the application (e.g. KOTS) in order to collect updates to the spec supplied by that application.  This update requirement delays folks from accessing updated Troubleshoot specs and therefore discourages people from writing new/updated specs.

## Goals

* Provide a means for a spec to optionally specify a location for a spec that can replace it if successfully reached.

## Other requirements

* Maintain the ability for Troubleshoot to run in an airgapped environment, though with some feature limitations.
* Maintain compatilbility with existing Troubleshoot specs.

## High-Level Design

Add a new field to the Troubleshoot spec definition, which includes a URI used to locate a replacement spec.

If the field is populated, Troubleshoot is to attempt to collect the spec from the location provided, and if successful, ignore the remainder of the spec provided and use the spec listed in the new field.

If the additional spec is not found at the location (or, if there is no network access to that location), Troubleshoot is to continue processing the remainder of the spec provided, with a log message describing the failure to process the URI.

Add a CLI flag (e.g. `--no-updates`) to disable the URI specified location from being accessed.  This would be useful in airgap environments and/or when we simply do not want to have the external spec used, without having to update the yaml, secret, or remote source.

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

We could add a new type, `specURI`, which contains one or more URIs from which to obtain specs.  If the spec is retrieved successfully then the remainder of the provided spec is ignored, and replaced with the one downloaded:

```yaml
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: default
spec:
  uri: https://raw.githubusercontent.com/replicatedhq/troubleshoot-specs/main/in-cluster/default.yaml
  collectors:
    - clusterInfo: {}
    - clusterResources: {}
  analyzers:
    - cephStatus: {}
    - longhorn: {}
```

When a spec is parsed from the initial call to Troubleshoot, the content should be read and if a `specURI` exists which is not blank, the code should attempt to download that URI.  Should that be successful, the collectors and analyzers sections of the downloaded content should be used to replace the remainder of the spec provided via the initial call.

## Impact on kURL and KOTS

This section is included because KOTS is a significant consumer of the Troubleshoot codebase, and as such deserves consideration when significant changes are made.

The code changes proposed in this document do not require any changes to kURL or KOTS, and any changes to those repos are excluded from the scope of work described here.  The following notes are suggestions for how the kURL and KOTS repos could benefit from the changes in this document, and could relate to any project which calls Troubleshoot or includes Troubleshoot specs.

For kURL, each component/addon installed could include adding an individual spec with a URI pointing at a specific spec for that component, which could be updated independently of other components.  Invoking troubleshoot with all the individual specs required would allow updated specs to be collected without needing to upgrade any particular component in the cluster.

KOTS currently provides a spec to Troubleshoot which is a merge of a default spec provided by the KOTS codebase, and the spec provided by an application vendor.  Should either of those provide a URI, that would mean the downloaded spec would replace the entire spec.  This is undesirable since the URI would need to contain the application vendor's updated spec as well as the generic cluster-wide collectors and analyzers, which is a significant maintenance challenge.  To resolve this, it is desirable that the following is completed before KOTS uses the URI field:

* Troubleshoot should accept multiple specs (see #650)
* KOTS would need to be modified to provide the application vendor and the default specs independently of one another.

## Limitations

Redactors are currently excluded from this proposal, though it would be desirable to add a similar feature in the future.

If a URI provided includes a spec which includes another URI, it is possible to get into some kind of recursion if the downloaded spec contains another URI.  For this reason, and to ensure that the results are predictable, we should only parse the URI field once per spec for specs passed to Troubleshoot on initiation via the CLI or entrypoint.

## Assumptions

## Testing

## Alternatives Considered

* Some mechanism to trigger a process to update secrets stored in Kubernetes.  This was rejected due to the need to run a manual update spec for something installed in the cluster.
* Altering KOTS to provide a URL for the spec rather than storing it in code.  This was rejected to allow airgap compatibility.

## Security Considerations

This change will make additional network calls with default usage of Troubleshoot. To ensure a user has control of limiting these calls an argument to disable this feature entirely will be provided. This allows users with different security postures to select what's appropriate for their environment.
