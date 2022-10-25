# Run collectors concurrently
 
## Goals

Increase the speed at which collectors complete.

Ensure that the cluster-resources collector runs prior to any others, so that things like pod status and pod logs only include the pods from the application rather than the ones that Troubleshoot started as part of collectors.
## Non Goals


## Background

Currently all collectors run sequentially, in the order in which they are specified in the spec.  This is implied, and has not been guaranteed.  With the addition of the ability to supply multiple specs, the order is no longer predicatble.

Although sequence is not guaranteed for any collectors, #768 ensures that the `clusterResources` collector runs first, so that the list of resources does not include those that are started during the Troubleshoot collection sequence.

## High-Level Design

Switch all collectors to use goroutines to collect (concurrently).  This should run the list of collectors of `clusterResources` type first, then the remainder concurrently when `clusterResources` is complete.

## Detailed Design

Tasks that can be split to separate merges (in order):

* Switch all collectors to use goroutines to collect (concurrently).

## Limitations
 
## Assumptions

* The Kubernetes API can handle the load of collectors running concurrently, including when the pod logs collector runs
 
## Testing

Any new function will need unit tests.

The existing Collect functions will need unit tests altered (or added).

We will need to ensure that `clusterResources` is collected prior to others.

## Documentation

There should be a note in https://troubleshoot.sh/docs/collect/ ensuring that folks do not expect the collectors to run in any particular order.

## Alternatives Considered

## Security Considerations

None identified.