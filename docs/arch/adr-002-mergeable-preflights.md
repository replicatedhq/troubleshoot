# ADR 002: Mergeable preflight specs

When the `preflight` binary, or pacakge, is called, it supports only one spec definition at a time.

Recent changes in Troubleshoot allow the `support-bundle` binary to be called with multiple specs at a time.  This allows cluster components to contribute independant Troubleshoot specs for their scope, and have Troubleshoot assemble them at run time.

Tools such as kURL have components which are maintained with a degree of separation to one another.  It would be helpful to the maintainers of such projects to be able to call `preflight` specifying a number of specs at runtime, allowing Troubleshoot to assemble them into one spec for collection/analysis.

Currently if the `preflight` binary is called with multiple specs, it simply ignores all after the first.

## Decision

Modify the `preflight` CLI and package to be able to read multiple args rather than just one.

Introduce a merge mechanism in the same way that the `support-bundle` binary runs, to merge and deduplicate Preflight specs.

## Status

Proposed

## Consequences

There are no backward compatibility consequences or breaking changes in this proposal.

The project benefits:
* Folks maintaining kURL add-ons can contribute unique preflight specs for their add-on (and the same for other projects simlarly structured)
* Folks using `preflight` from the CLI without other applications (e.g. for a Helm install) are able to specify a list of preflights for their application rather than having to assemble one spec for each environment.

## Design notes

This proposal does not include adding the `uri:` field to `kind: Preflight`.

The file `cmd/preflight/cli/root.go` calls `preflight.RunPreflights` with `args[0]` which is likely to need to change to just `args`.

Func `RunPreflights` takes a single string arg (`arg string`) for the spec definition.  This is likely to need to change to `arg []string`.

File `cmd/troubleshoot/cli/run.go` loops through the list of args, concatenating them together.  A similar process is suitable for this change.