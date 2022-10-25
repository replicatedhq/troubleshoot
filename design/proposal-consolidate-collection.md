# Consolidate collector code
 
## Goals

Reduce code maintenance needs.

Improve consistency between preflight and support-bundle.

## Non Goals

## Background

In the current Troubleshoot code base, there are three separate paths to running collectors.  These include preflights, support-bundle, and the collect package.  The three have diverged over time and are different to one another, but do not appear to have any need to be separate. This is confusing and likely to introduce errors in the future.

## High-Level Design

* Add a `collect` package public API that can be called to run the collect logic, from any other package
* change the preflight, support-bundle and collect binaries to call that API rather than their own collect routines

## Detailed Design

TODO:
* API design, inputs, outputs
* check if host collectors really need to be separate from collectors
* decide if we wish to maintain the `collect` binary or deprecate it in favor of `support-bundle`.

## Limitations
 
## Assumptions

* there is no need to run collectors differently between preflight and support-bundle
 
## Testing

Any new function will need unit tests.

Existing tests will need to be altered, and possibly consolidated.

## Documentation

The public function to call collection should be documented and at that point regarded stable.

## Alternatives Considered

## Security Considerations

None identified.

## Related changes

TODO:
* make an API for analyze and redact
* create an API that calls collect, redact, and analyze - and decide if that can be called by preflight and support-bundle