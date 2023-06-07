# Consolidated Troubleshoot CLI

## Goals

As Troubleshoot grows and gains new features, some of which involves flags for setting options, and to make it easier to add additional subcommands that don't belong underneath either `support-bundle` or `preflight` binaries, we would like to consolidate all of the Troubleshoot commands under one binary.

There is discussion about changing the behaviour of `preflight`, considering that preflights and support-bundles utilize the same specs - the same collectors and analyzers - and only differ in what is returned to the user post-analysis.  For this design proposal, `support-bundle` and `preflight` may be condensed into a single `troubleshoot` binary.

For the purposes of this design we'll talk about troubleshoot as a standalone binary. It will also be available, and likley most commonly installed as, a krew plugin for kubectl.

## Non-Goals

## Background

## High Level Design

Functions of `support-bundle`, `preflight`, `analyze`, `redact`, and `sbctl` binaries/tools should be rolled together into a single `troubleshoot` CLI that can perform all necessary functions of the same.

`troubleshoot` CLI should be able to report on the version that is installed in the CLI, and any support bundle generated with `troubleshoot` should report its build/version inside the archive it generates.

## Detailed Design

In the interest of being able to work on this quickly without breaking existing use-cases, a new `troubleshoot` command should be created. Utilizing cobra and viper best practices from the cobra.dev docs.

A guiding principle of this redesign should be that the we are defining a set of "artefacts" (i.e: a support bundle, a spec), and each of the defined public functions acts as an interface to interact with these artefacts. either creating, manipulating, or performing other actions on them. in this way we guarantee that each public function can be re-used on an artefact (support bundle) in any stage of it's existance.

For example, running a set of redactors or analyzers on an existing support bundle without having to re-run collection.

### sbctl → troubleshoot inspect

sbctl should be migrated to the troubleshoot repository in a "lift and shift" operation to start with.

It should continue to be built as a standalone binary to enable continued use until a stable replacement exists. as such it should me integrated in a way that preserves the existing `cmd` and it's package requirements.

Once integrated into the repository, we can begin using functions from it's packages from the from the new `troubleshoot` command to enable the `inspect` subcommand.

### Public APIs

The existing package entrypoint we use from the support-bundle command is `supportbundle.CollectSupportBundleFromSpec()`
which is an end-to-end function that runs collectors, redactors and then analysis.

The Consolidated cli should be responsible for handling the end-to-end running of this process in the `cmd` leaving the importable packages free to focus on defining the individual collect, redact analyze steps.

To this end we should create a new `pkg` that targets the functionality provided by the `collect` and `analyze` packages such as `collect.runHostCollectors()` and provide a stable API to be used by the CLI and other projects. At the same time we can unpublish functionality that should not be exposed (such as the individual collector and analyser functions) and mark code for deprecation.

This new package should be kept as minimal as possible. and serve only as an interface to private functions in the other packages.

Once the stable API is ready we can instruct projects like kurl to target that and work on removing code marked for deprecation and migrate non-public functions to an `internal` package to make it clear that it's not intended to be imported.

The functionality we want to expose via this api is:

- `CollectBundle(context.Context, opt CollectOptions) (bundlePath string, error)`
  - collect a support bundle from a spec.
  - takes a parsed spec struct as an parameter.
  - returns a path to the bundle directory and errors.
  - to minimise IO and increase collection speed. redactors should be run inline, redacting data in memory before it's saved to a bundle.
- `RedactBundle(context.Context, opt RedactOptions) error`
  - redact an already collected bundle, takes a path to the bundle as an parameter.
  - returns any errors
- `AnalyzeBundle(context.Context, opt AnalyzeOptions) (results{},error)`
  - run analysers from the spec and return the analysis results struct
  - returns analysis struct and errors
- `ArchiveBundle(context.Context, opt ArchiveOptions) error`
  - generates a tar archive of the bundle directory at the specified path with optional compression
  - takes bundle path, compression method and destination as parameters.
  - returns errors
- `ServeBundle(context.Context, opt ServeOptions)`
  - starts a sbctl server using the specified bundle and port, outputting a kubeconfig at a specified location.
- `LoadSpecs(context.Context, opt LoadOptions) ([]TroubleShootKind,err)`
  - Takes a loadOptions struct and returns a list of parsed troubleshoot kinds.


Some examples of the options structs.

```go
type LoadOptions struct {
  RawSpecs []string // list of locations for specs to load
  RawSpec string // a single spec to parse
  FilePaths []string // list of filepaths to check, can be globbed
  URIs []string // list of URIs to retrieve specs from
  SearchCluster bool // toggle for searching cluster from context for troubleshoot objects
}
```

```go
type CollectOptions struct {
  Specs []TroubleshootKind // list of specs to extract collectors and redactors from
}
```

```go
type RedactOptions struct {
  Specs []troubleshootKind // list of specs to extract redactors from
}
```
Note: this is almost identical to `CollectOptions` for now but remains separate to enable easier addition of redact specific options at a later date

```go
type ServeOptions struct {
  Address string // address to listen on including port (0.0.0.0:8080)
  ConfigPath string // optional path to store generated kubeconfig
}
```


### Usage patterns

- generate a support bundle

  `troubleshoot collect supportbundle.yaml`

  `troubleshoot collect supportbundle.yaml secrets/default/kotsadm-appslug-supportbundle`

  `troubleshoot collect https://kots.io`

- if no bundle URI/location is specified, search for a spec in-cluster

  `troubleshoot collect`

- use a spec to return a go/no-go preflight outcome

  `troubleshoot preflight spec.yaml`

  This should not only clearly state any reasons for failing, but also use standardised exit codes that can be used by automation tools.

  We should also continue to accept standard in, accepting any preflights from a yaml multidoc.

  `helm template | troubleshoot preflight -`

- Interact with an existing support bundle with kubectl.

  `troubleshoot inspect support-bundle-12-12-2001.tar.gz`

  This should set the `KUBECONFIG` environment variable, as well as a `TROUBLESHOOT_SHELL` environment variable before spawning a subshell.
  This is to allow prompts and shell environments to be able to detect that they're running in a troubleshoot spawned shell, much like tmux et-al.

  Alternatively the `--interactive=false` global flag will disable the spawning of a nested shell.
  this should behave much like the `sbctl serve` command does today,
  printing the path to it's temporary kubeconfig location to stdout. optionally taking flags to specify the kubeconfig location for use in advanced automation.

  `troubleshoot inspect --interactive=false support-bundle-12-12-2001.tar.gz`

  `troubleshoot inspect --interactive=false -o /path/to/kubeconfig support-bundle-12-12-2001.tar.gz`

- Redact an existing spec

  `troubleshoot redact -f redactors.yaml support-bundle.tar.gz`

- Re-run analysers against an existing support bundle

  `troubleshoot analyze -f spec.yaml support-bundle.tar.gz`

### Example help text

Overall top level command:

```
Kubernetes diagnostics assistant

Usage:
  troubleshoot [command]

Available Commands:
  help        This help text
  collect     Run collectors, redactors and analyzers, store the result
  completion  Generate the autocompletion script for the specified shell
  analyze     Analyze an existing support bundle
  redact      Run redactors across an existing support bundle
  preflight   Run collectors, and analyzers, and provide a pass/fail preflight result with explanation
  inspect     Open an interactive shell to inspect an existing support bundle with kubectl.
  version     Print the current version and exit
```

### global flags:

```
      --as string                      Username to impersonate for the operation. User could be a regular user or a service account in a namespace.
      --as-group stringArray           Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --as-uid string                  UID to impersonate for the operation.
      --cache-dir string               Default cache directory (default "/Users/xavpaice/.kube/cache")
      --certificate-authority string   Path to a cert file for the certificate authority
      --client-certificate string      Path to a client certificate file for TLS
      --client-key string              Path to a client key file for TLS
      --cluster string                 The name of the kubeconfig cluster to use
      --collect-without-permissions    always generate a support bundle, even if it some require additional permissions (default true)
      --context string                 The name of the kubeconfig context to use
      --cpuprofile string              File path to write cpu profiling data
      --debug                          enable debug logging. This is equivalent to --v=0
      --disable-compression            If true, opt-out of response compression for all requests to the server
  -h, --help                           help for troubleshoot
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --interactive                    enable/disable interactive mode (default true)
      --kubeconfig string              Path to the kubeconfig file to use for CLI requests.
      --memprofile string              File path to write memory profiling data
  -n, --namespace string               If present, the namespace scope for this CLI request
      --no-uri                         When this flag is used, Troubleshoot does not attempt to retrieve the bundle referenced by the uri: field in the spec.`
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                  The address and port of the Kubernetes API server
      --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
  -v, --v Level
```

### Collect command:

```
Collect a support bundle, which is an archive of files, output, metrics and state
from a server that can be used to assist when troubleshooting a Kubernetes cluster.

The collected information is analyzed and private information redacted according to the spec.

If a URL is not provided, you must either specify --load-cluster-specs, or use "-" to
load the spec from stdin.

Usage:
  troubleshoot collect [urls...] [flags] [-]

Flags:
      -o, --output string              specify the output file path for the support bundle
      --redact                         enable/disable default redactions (default true)
      --redactors strings              names of the additional redactors to use
      --load-cluster-specs             enable/disable loading additional troubleshoot specs found within the cluster. required when no specs are provided on the command line
      --since string                   force pod logs collectors to return logs newer than a relative duration like 5s, 2m, or 3h.
      --since-time string              force pod logs collectors to return logs after a specific date (RFC3339)
  -l, --spec-labels strings            selector to filter on for loading additional support bundle specs found in secrets within the cluster (default [troubleshoot.io/kind=support-bundle])
```

### Preflight command:

```
A preflight check is a set of validations that can and should be run to ensure
that a cluster meets the requirements to run an application.

Usage:
  troubleshoot preflight [urls...] [flags]

Flags:
      --node-selector string           selector (label query) to filter remote collection nodes on.
```

### Analyze command:

positional arguments:

1. bundle location

flags:

- -f      path to spec file

### Inspect command:

Positional arguments:

1. bundle location

flags:

- -p --port         port to listen on
- -o --kubeconfig   path for generated kubeconfig


## Limitations

## Assumptions

- sbctl has no package naming conflicts with troubleshoot

## Testing

## Documentation

## Alternatives Considered

## Security Implications
