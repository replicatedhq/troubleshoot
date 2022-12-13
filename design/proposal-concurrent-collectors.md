# Run collectors concurrently
 
## Goals

Increase the speed at which collectors complete.

## Non Goals


## Background

Currently all collectors run sequentially, in the order in which they are specified in the spec.  This is implied, and has not been guaranteed.  With the addition of the ability to supply multiple specs, the order is no longer predicatble.

Although sequence is not guaranteed for any collectors, #768 ensures that the `clusterResources` collector runs first, so that the list of resources does not include those that are started during the Troubleshoot collection sequence.

The performance for support-bundle and preflight runs is important to the end user experience of the product.  There are examples where a support bundle takes multiple minutes to complete, and folks are left wondering if the process has hung or crashed.  Preflights that take too long will simply be skipped, which leads to problems with installation being difficult to identify.

## High-Level Design

Switch collectors and host collectors to use goroutines to collect (concurrently).  This should run the list of collectors of `clusterResources` type first, then the remainder concurrently when `clusterResources` is complete.

## Detailed Design

Tasks that can be split to separate merges (in order):

* Switch all collectors to use goroutines to collect (concurrently).
  * In `supportbundle.runCollectors()`, refactor to use a `go func()` pattern to run all the collectors that are not `clusterResources` concurrently.  Check that the first in the list is `clusterResources` and run that separately, prior to the others.
  * This task should introduce an optional concurrency limit field, default to, say, 10, which limits the number of goroutines/collectors that can run at any given time.
  * This task should improve the collection speed of in-cluster collectors and preflights

* Switch Host Collectors to use goroutines for concurrent collection.
  * In `supportbundle.runHostCollectors()`, refactor to use a `go func()` pattern for each collector.
  * No need to honor the same concurrency limit as host collectors do not load the Kubernetes API

* Switch Host Preflights to use goroutines - `preflight.CollectHost()` runs `go func()` for each collector.

* Switch in-cluster Preflights to use goroutines - `preflight.Collect()` runs `go func()` for each collector.
  * Ensure `clusterResources` runs in advance of the others so that pods started by collectors do not produce an error in the results.

## Limitations

Relies on splitting up the list of collectors into `clusterResources` and others.

Collisions between specs (e.g. if `runPod` has multiple specs with the same name) are not handled.  At the current stage with collectors running sequentially, if two collectors write to the same file the second overwrites the data from the first.  If collectors run concurrently, there can be issues creating multiple pods with the same name, and issues with concurrent access to the target file.  This issue is tracked in [#895](https://github.com/replicatedhq/troubleshoot/issues/895)

This proposal excludes Remote Collectors.
## Assumptions

* The Kubernetes API can handle the load of collectors running concurrently, including when the pod logs collector runs.

## Testing

Any new function will need unit tests.

The existing Collect functions will need unit tests altered (or added).

We will need to ensure that `clusterResources` is collected prior to others.

## Documentation

There should be a note in https://troubleshoot.sh/docs/collect/ ensuring that folks do not expect the collectors to run in any particular order.

## Alternatives Considered

None.
## Security Considerations

None identified.