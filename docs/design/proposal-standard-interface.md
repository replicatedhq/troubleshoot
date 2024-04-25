# Provide an extendable API for accessing bundle information

## Goals

* Provide API based access to collected information, decoupling other projects from troubleshoot on-disk format
* Reuse existing APIs when they exist to make the bundle compatible with existing tools without modification
* Plan for extensibility for accessing information beyond just the standard kubernetes api

## Non Goals

* Compatibility with previous on-disk formats
* Compatibility with existing collectors without modification
  * There should be a plan to allow the implementation of existing collectors

## Background

While using the information gathered in a support bundle users were finding it hard to find information while manually reviewing the various files collected in the bundle. Users have to understand the folder structure, files structure, and process JSON files to find information about the cluster. The [sbctl](https://github.com/replicatedhq/sbctl) project was created to prove out the utility of providing api based access so that users could use existing tools which they already understood. This has been a very successful experiment with feedback being that most users now use this utility as their primary, or only, interface to the support bundle information.

There are some drawbacks to the current approach. The `sbctl` project is tightly coupled to troubleshoot on-disk formats, each kubernetes API must be implemented in `sbctl` individually and will require being kept up to date as APIs change, and `sbctl` has no plan today to provide similar API based access to information other than the standard kubernetes api.

This proposal is meant to take the learnings from `sbctl` and consider implementing it as a first class feature of troubleshoot while attempting to address the current maintenance and expandability limitations.

## High-Level Design

To help address standard access to API data, troubleshoot will start an etcd instance which collectors can then use to store collected information rather than writing to custom on-disk locations. Collectors that only need to collect Kubernetes API information then do not need to serve up data as the API server will be used to provide access to the collected data. Storing data directly in etcd will allow troubleshoot to later serve up this same information by again starting an api-server and etcd using the previously collected etcd data store. This should remove almost all maintenance from troubleshoot for implementing API calls to access the collected information.

There will also be information that is desired to be stored that doesn't have any native representation in the api-server today. Examples could include custom collectors which execute into containers and extensions like [metrics server](https://github.com/kubernetes-sigs/metrics-server). These components can be addressed using the built in [Kubernetes API Aggregation Layer](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/apiserver-aggregation/) which is how `metrics server` itself works to extend and provide api based access to node information. By registering additional API extensions troubleshoot plugins can implement both a collection and an API for retrieving custom information which is accessible in the same fashion as the rest of the api-server.

The additional benefit to this, which can be demonstrated with `metrics-server`, is that collectors can be written for any other extension API and be compatible with existing tools the same way using the api-server provides compatibility with `kubectl`. Using `metrics-server` as an example, a collector can be written to collect node information which can be stored locally. The collector can then implement the standard [GET Endpoints](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/apiserver-aggregation/) from the metrics-server and reply to them with the results it collected. By doing so, all tools that work with metrics-server today, like `kubectl top` will also work with information served from a support bundle. Additionally, the collector implementation handles any custom file formats without exposing that to any other tool making it easy to maintain.

Any other access to the filesystem directly will be modified to instead use the provided api-server. This means analyzers will not directly reference files on disk and will instead run against the api-server to analyze the collected information. This decouples analyzers from how collectors store data providing clean separation of concerns for maintaining both collectors and analyzers. Decoupling these also creates a natural way to implement analyzers that use data from multiple collectors. This could also allow analyzers to run against existing clusters providing a use for analyzers independent of support bundle collection. This could encourage additional community contributions of analyzers.

An initially unintended benefit of using the Aggregation Layer is that any HostCollector using this implementation would be very close to an implementation of an extension which you could install in clusters. This could make HostCollectors also useful to install as a service in a live cluster for operations information about hosts.

## Detailed Design

### Outstanding design questions

1. How reasonable is it to start an api-server as part of troubleshoot? Consider the following known implementations that do something like this:

* [envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest) - requires binaries present on the machine
* [microk8s implementation](https://github.com/canonical/microk8s/blob/master/build-scripts/patches/0000-Kubelite-integration.patch) - bundles slightly modified binaries
* [k0s uses upstream binaries statically compiled](https://docs.k0sproject.io/v1.23.8+k0s.0/architecture/) - bundles statically compiled binaries that self extract and uses a process monitor to run them

2. Can you in fact push metadata like "Status" into an api-server or do we have to write directly to etcd?

* If we can't push to the api-server is just writing the information directly into etcd something we can do and have a reasonable expectation of compatibility?

3. Is the overhead to write an Aggregation API going to add an unnecessary burden to writing new collector plugins? Can these be templated into a reasonably to ease collector creation?

## Limitations

Using the actual API server will provide limitations on the version skew which can be collected/displayed. This could be addressed by including multiple versions of the kubernetes-api server to allow serving a wide range of support bundles. This limitation likely already exists today but would exist in the tooling that is trying to collect, analyzer, or server the data.

## Assumptions

* Serving logs hasn't been designed yet, and presumably can be addressed in the detailed design to provide logs to the api-server in place of kubelet. Ideally this can be done using the standard upstream kubernetes-api server it is undesirable to fork it.
* Running the api-server, etcd, and any other supporting services (like kubelet) as go routines while adding some overhead to the collection process won't cause a significant burden on ram or cpu to collect support bundles.

## Testing

## Alternatives Considered

### Keep sbctl separate

The current `sbctl` project could be left to run it's course independent of this project. This leaves the troubleshoot project reliant on a separate project to provide a good user experience for anything other than analyzers.

## Security Considerations

Consideration to how Redactors are implemented needs to be considered.

## References

Original PR discussion found [here](https://github.com/replicatedhq/troubleshoot/pull/611)
