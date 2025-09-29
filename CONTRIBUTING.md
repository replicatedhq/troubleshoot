# Contributing to Troubleshoot

Thank you for your interest in Troubleshoot, we welcome your participation. There are a number of ways to participate in Troubleshoot as outlined below:

# Community

For discussions about developing Troubleshoot, there's an [#app-troubleshoot channel in Kubernetes Slack](https://kubernetes.slack.com/channels/app-troubleshoot).

## Issues

- [Request a New Feature](https://github.com/replicatedhq/troubleshoot/issues/new?assignees=&labels=feature&template=feature_enhancement.md) Create an issue to add functionality that addresses a problem or adds an enhancement.
- [Report a Bug](https://github.com/replicatedhq/troubleshoot/issues/new?assignees=&labels=bug&template=bug_report.md) Report a problem or unexpected behaviour with Troubleshoot.

## Design Principles

When implementing a new feature please review the [design principles](./docs/design/design-principles.md) to help guide the approach.

## Development Environment

To get started we recommend:

1. Go (v1.24 or later)
2. For cluster-based collectors, you will need access to a Kubernetes cluster
3. Fork and clone repo
4. Run `make clean build` to generate binaries
5. You can now run `./bin/preflight` and/or `./bin/support-bundle` to use the code you've been writing

> Note: to cross-compile a Linux binary from MacOS:
> `GOOS=linux GOARCH=amd64 make clean build`

### Testing

To run the tests locally run the following:

```bash
make test
make test RUN=TestClusterResources_Merge
```

Additionally, e2e tests can be run with:

```bash
make support-bundle preflight e2e-test
```

A running Kubernetes cluster as well as `jq` are required to run e2e tests.

### Profiling

You are able to collect CPU & memory runtime properties and store the data for analysis in a file. To do so, pass in the file paths using `--cpuprofile` and `--memprofile` flags in the CLI. Once you have your data collected, you can analyse it using [pprof visualization tool](https://github.com/google/pprof/blob/main/doc/README.md). Here is how

Run support bundle and with CPU & memory profile flags

```sh
./bin/support-bundle examples/support-bundle/sample-supportbundle.yaml --cpuprofile=cpu.prof --memprofile=mem.prof
```

Visualize using [pprof](https://github.com/google/pprof/blob/main/doc/README.md)

```sh
go tool pprof -http=":8000" cpu.prof

go tool pprof -http=":8001" mem.prof
```

**Additional flags for memory profiling**
- `inuse_space`: Amount of memory allocated and not released yet (default).
- `inuse_objects`: Amount of objects allocated and not released yet.
- `alloc_space`: Total amount of memory allocated (regardless of released).
- `alloc_objects`: Total amount of objects allocated (regardless of released).

More on profiling please visit https://go.dev/doc/diagnostics#profiling

## Contribution workflow

We'd love to talk before you dig into a a large feature. 
