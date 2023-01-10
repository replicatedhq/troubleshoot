# Consolidated Troubleshoot CLI

## Goals

As Troubleshoot grows and gains new features, some of which involves flags for setting options, and to make it easier to add additional subcommands that don't belong underneath either `support-bundle` or `preflight` binaries, we would like to consolidate all of the Troubleshoot commands under one binary/plugin.

There is discussion about changing the behaviour of `preflight`, considering that preflights and support-bundles utilize the same specs - the same collectors and analyzers - and only differ in what is returned to the user post-analysis.  For this design proposal, `support-bundle` and `preflight` may be condensed into a single `troubleshoot` binary/plugin.

## Non-Goals


## Background




## High Level Design

Functions of `support-bundle`, `preflight`, `analyze`, `redact`, and `sbctl` binaries/tools should be rolled together into a single `troubleshoot` CLI plugin that can perform all necessary functions of the same.

`troubleshoot` CLI plugin should be able to report on the version that is installed in the CLI, and any support bundle generated with `troubleshoot` should report its build/version inside the archive it generates.

## Detailed Design

- generate a support bundle

  `kubectl troubleshoot supportbundle.yaml`

  `kubectl troubleshoot supportbundle.yaml secrets/default/kotsadm-appslug-supportbundle`

  `kubectl troubleshoot https://kots.io`

- use a spec to return a go/no-go preflight outcome

  `kubectl troubleshoot --preflight spec.yaml`

- use a support bundle tarball to execute `sbctl` and shell into a support bundle

  `kubectl troubleshoot --shell support-bundle-12-12-2001.tar.gz`

## Limitations

## Assumptions

## Testing

## Documentation

## Alternatives Considered

## Security Implications
