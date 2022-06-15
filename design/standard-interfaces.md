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

While using the information gathered in a support bundle users were finding it hard to find information while manually reviewing the varous files collected in the bundle. Users have to understand the folder structure, files structure, and process JSON files to find information about the cluster. The [sbctl](https://github.com/replicatedhq/sbctl) project was created to prove out the utility of providing api based access so that users could use existing tools which they already understood. This has been a very successful experiment with feedback being that most users now use this utility as their primary, or only, interface to the support bundle information.

There are some drawbacks to the current approach. The `sbctl` project is tightly coupled to troubleshoot on-disk formats, each kubernetes API must be implemented in `sbctl` individually and will require being kept up to date as APIs change, and `sbctl` has no plan today to provide similar API based access to information other than the standard kubernetes api.

This proposal is meant to take the learnings from `sbctl` and consider implementing it as a first class feature of troubleshoot while attempting to address the current maintenance and expandability limitations.

## High-Level Design

To help address standard access to API data, troubleshoot will start an api server backed by etcd which collectors can then use to store collected information rather than writting to custom on-disk locations. This will allow collectors to use the standard API to both collect (from the live API server) and store (to the ephemeral troubleshoot API server) data. Storing data directly in etcd will allow troubleshoot to later serve up this same information by again starting an api-server and etcd using the previously collected etcd data store. This should remove almsot all maitnance from troubleshoot for implementing API calls to access the collected information.

There will also be information that is desired to be stored that doesn't have any native representation in the api-server today. Examples could include customer collectors which execute into containers and extensions like [metrics server](https://github.com/kubernetes-sigs/metrics-server). These components can be addressed using the built in [Kubernetes API Aggregation Layer](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/apiserver-aggregation/) which is how `metrics server` itself works to extend and provide api based access to node information. By registering additional API extensions troubleshoot plugins can implement both a collection and an API for retrieving custom information which is accessible and revisioned in the same fashion as the rest of the api-server.

The additional benefit to this, which can be demonstrated with `metrics-server`, is that collectors can be written for any other extension API and be compatible with existing tools the same way using the api-server provides compatibility with `kubectl`. Using `metrics-server` as an example, a collector can be written to collect node information which can be stored locally. The collector can then implement the standard [GET Endpoints](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/apiserver-aggregation/) from the metrics-server and reply to them with the results it collected. By doing so, all tools that work with metrics-server today, like `kubectl top` will also work with information served from a support bundle. Additionally, the collector implementation handles any custom file formats without exposing that to any other tool making it easy to maintain.

Any other access to the filesystem directly will be modified to instead use the provided api-server. This means analyzers will not directly reference files on disk and will instead run against the api-server to analyze the collected information. This decouples analyzers from how collectors store data providing clean separation of concerns for maintaining both collectors and analyzers. This could also allow analyzers to run against existing clusters providing a use for analyzers independent of support bundle collection. This could encourage additional community contributions of analyzers.

## Detailed Design

TBD

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
