# Troubleshoot Design Principles

This document captures design principles that the Troubleshoot project abides by. This is intended to provide new contributors with guidance on how to approach problems and a better understanding of what to consider and address while implementing features.

## Client not Cluster based

Troubleshoot has to communicate with the Kubernetes API server to gather information. However, Troubleshoot should interact with the cluster as little as reasonably possible and has no intention of having a persistent in-cluster presence. There are several reasons for this approach, in no particular order those include:

* Users experiencing an issue in their cluster may discover Troubleshoot after they have a problem and requiring installed components may exclude them from solving their problem.
* A user may have limited cluster access and requiring cluster wide installation like a CRD or Operator can prevent them for using tools to recover from their errors.
* By definition Troubleshoot is being used because there is an issue with the cluster, to prevent exasperating the issue Troubleshoot should avoid writing to the cluster.

## Tools not specs

The Troubleshoot project should include tools which can be used to diagnose issues but is not attempting to include Specs built into the project itself. The number of issues and projects which could benefit from Troubleshoot is very large and we intended to enable those projects to better support their project. It is unreasonable and undesirable for Troubleshoot to be the source of truth for all possible cluster and software issues.

## Fail forward

Clusters with issues can be unpredictable, the scope of error conditions is likely unbounded. Troubleshoot should keep this in mind when doing error handling and fail forward proceeding with as much of the intended operation as possible while logging the errors. For example if a collector fails to collect information that should not prevent other collectors from running. Any condition that causes Troubleshoot to hang or not complete a run is considered a bug.

## Provide a predictable user experience

When things go wrong people can be stressed and stressed people aren't likely to thoroughly read documentation. Whenever possible default values, command line flags, etc should be set to provide a user the best experience possible with the input provided. Some examples of this include:

* When accepting flags for things like Namespaces, as much as possible, use the same syntax that `kubectl` accepts which users are likely to try instinctively.
* Default to gathering more information not less, rarely is it easier to diagnose an issue with less information.

