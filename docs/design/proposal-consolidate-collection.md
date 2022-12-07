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

Package `preflight`:
* Remove `CollectHost`, `Collect` and `CollectRemote`

Package `supportbundle`:
* Remove `runCollectors`, `CollectSupportBundleFromSpec`, and associated code

Package `collect`:
* Add a replacement for `runCollectors`, `CollectSupportBundleFromSpec` taken from `supportbundle`
* Add replacements for `CollectHost`, `Collect` and `CollectRemote` taken from `preflight`
* Where the above duplicate functionality, alter the `supportbundle` or `preflight` packages to ensure that a single new function handles the requirement

CLI packages:
* Alter to use the public functions from `collect`

## Limitations

Breaking change - KOTS at least imports `CollectSupportBundleFromSpec` plus potentially others.

This does not affect the analysis and redaction portions of code.

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