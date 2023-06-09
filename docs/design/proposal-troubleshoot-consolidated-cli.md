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

The core of this api will be exposed via new troubleshoot `types`

#### Conventions
* Interfaces instead of concrete types: Where possible, prefer using interfaces that concrete types implement. This abstraction allows us to
  * Create clear API contracts and separation of concerns e.g A `collect.Collect` interface only collects information from various sources into a bundle. The implementor (can implement several interfaces) has to abide to this contract and the consumer expects the contract to be held.
  * Extensibility: Extending or even replacing concrete type implementations is now possible when the public APIs are defined as interfaces. This gives us the ability to maintain the project with minimal risk of introducing breaking changes. It also allows us to have the flexibility of introducing concepts such as plugins in the future.
  * Testability of public APIs: With interfaces, we are able to test our contracts with ease cause we are able to create stubs we can build our tests on.

Below are the proposed APIs

*Types*
```go
// TroubleshootKinds wraps all kinds defined by troubleshoot
type TroubleshootKinds struct {
    AnalyzersV1Beta2        []troubleshootv1beta2.Analyzer
    CollectorsV1Beta2       []troubleshootv1beta2.Collector
    HostCollectorsV1Beta2   []troubleshootv1beta2.HostCollector
    HostPreflightsV1Beta2   []troubleshootv1beta2.HostPreflight
    PreflightsV1Beta2       []troubleshootv1beta2.Preflight
    RedactorsV1Beta2        []troubleshootv1beta2.Redactor
    RemoteCollectorsV1Beta2 []troubleshootv1beta2.RemoteCollector
    SupportBundlesV1Beta2   []troubleshootv1beta2.SupportBundle
    // Any future kinds e.g AnalyzersV1 would end up here
}

// To Allow for forward compatibility while reducing the impact of potentially breaking changes, options will be passed into these methods via options structs.
type LoadOptions struct {
    RawSpecs []string // list of locations for specs to load
    RawSpec string // a single spec to parse
    FilePaths []string // list of filepaths to check, can be globbed
    URIs []string // list of URIs to retrieve specs from
    SearchCluster bool // toggle for searching cluster from context for troubleshoot objects
    ProgressChan chan // a channel to write progress information to
}

type LoadBundleOptions struct {
    Path string // Path to archive or directory of bundle files
}

type CollectOptions struct {
    Specs *TroubleshootKinds // list of specs to extract collectors and redactors from
    ProgressChan chan // a channel to write progress information to
}

type RedactOptions struct {
    Specs *TroubleshootKinds // list of specs to extract redactors from
    ProgressChan chan // a channel to write progress information to
}
// Note: this is almost identical to `CollectOptions` for now but remains separate to enable easier addition of redact specific options at a later date

type AnalyzeOptions struct {
    ProgressChan chan // a channel to write progress information to
}

type ServeOptions struct {
    Address string // address to listen on including port (0.0.0.0:8080)
    ConfigPath string // optional path to store generated kubeconfig
}

type SaveAnalysisOptions struct {
    Format string // format to save analysis in: json/yaml/csv/plaintext
    Path string // where to save the analysis output (default to Bundle.FilePath)
}

type AnalysisResults struct {
    Bundle Bundle // include bundle metadata
}
```

*Interfaces*
```go
// Bundler interface implements all functionality related to managing troubleshoot bundles
type Bundler interface {
    // Collect runs collections defined in TroubleshootKinds passed through CollectOptions
    Collect(CollectOptions) (error) {}

    // We need to expose the bundle data collected in some form of structure as well
    // We have https://github.com/replicatedhq/troubleshoot/blob/620fa75eb593247a07c4dc39ea96fc6a059be111/pkg/collect/result.go#L15 at the moment

    // Analyze runs analysis defined in TroubleshootKinds passed through AnalyzeOptions
    Analyze(AnalyzeOptions) (AnalysisResults, error) {}

    // AnalyzeResults contains the analysis results that the bundle has
    AnalyzeResults() AnalysisResults

    // Redact runs redaction defined in TroubleshootKinds passed through RedactOptions
    Redact(RedactOptions) error {}

    // Archive produces an archive from a bundle on disk with options passed in ArchiveOptions
    Archive(ArchiveOptions) error {}

    // Load loads a bundle from a directory or archive served from disk or a remote location like a URL
    Load(LoadBundleOptions) (error) {}
    // it's worth noting that while this may appear inefficient now
    // it allows us to extend what we include in the Bundle struct in future without having to continuously extend the load function.

    // Serve starts an sbctl-like server with options defined in ServeOptions
    Serve(ServeOptions) error {}
}

// These interfaces already exist in some form. We would need to review them

// https://github.com/replicatedhq/troubleshoot/blob/620fa75eb593247a07c4dc39ea96fc6a059be111/pkg/collect/host_collector.go#L7
type HostCollector interface {
    Title() string
    IsExcluded() (bool, error)
    ObjectTyper

    // Belongs in the Bundler interface
    // Collect(progressChan chan<- interface{}) (map[string][]byte, error)
}

// https://github.com/replicatedhq/troubleshoot/blob/620fa75eb593247a07c4dc39ea96fc6a059be111/pkg/collect/collector.go#L18
type Collector interface {
    Title() string
    IsExcluded() (bool, error)
    ObjectTyper

    // This belong elsewhere and, if need be, should be composed (golang's interface composition) into this interface. Retaining them for review
    // GetRBACErrors() []error
    // HasRBACErrors() bool
    // CheckRBAC(ctx context.Context, c Collector, collector *troubleshootv1beta2.Collect, clientConfig *rest.Config, namespace string) error

    // Belongs in the Bundler interface
    // Collect(progressChan chan<- interface{}) (CollectorResult, error)
}

// https://github.com/replicatedhq/troubleshoot/blob/620fa75eb593247a07c4dc39ea96fc6a059be111/pkg/analyze/analyzer.go#LL173C1-L177C2
type Analyzer interface {
    Title() string
    IsExcluded() (bool, error)
    ObjectTyper

    // Belongs in the Bundler interface
    // Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error)
}

// https://github.com/replicatedhq/troubleshoot/blob/620fa75eb593247a07c4dc39ea96fc6a059be111/pkg/analyze/host_analyzer.go#L5
type HostAnalyzer interface {
    Title() string
    IsExcluded() (bool, error)
    ObjectTyper

    // Belongs in the Bundler interface
    Analyze(getFile func(string) ([]byte, error)) ([]*AnalyzeResult, error)
}

// https://github.com/replicatedhq/troubleshoot/blob/620fa75eb593247a07c4dc39ea96fc6a059be111/pkg/redact/redact.go#LL31C1-L33C2
type Redactor interface {
    Redact(input io.Reader, path string) io.Reader
    ObjectTyper
}

// ObjectTyper interface exposes an internal object and its type for a consumer of an interface to get the concrete type implementing that interface
// This interface is meant for collectors, analysers and redactors so as to get back the object created from a spec. It should not be used with other
// interfaces that have internal implementation that's bound to change such as Bundler.
// TODO: Is this the best name for this? I'm just following go's recommendation - https://go.dev/doc/effective_go#interface-names
type ObjectTyper interface {
    // TODO: Is there a simpler way?
    // TODO: Maybe we should limit this interface usage to collectors/analysers/redactor. Call it KindCaster? ObjectKinder?? SpecTyper??
    Object() interface{}  // typeless object that was created e.g troubleshootv1beta2.Ceph collector, or redact.MultiLineRedactor redactor
    Type() string // type information that can be used to cast the object back to its original concrete implementation. e.g troubleshootv1beta2.Ceph
                  // NOTE: The concrete type exposed here needs to be a public type
}
```

*functions*

```go
// Load loads specs defined by the LoadOptions struct and returns a TroubleshootKinds object
func Load(LoadOptions) (TroubleshootKinds, error){}
```

With the above definitions, an example of generating a supoprt bundle:

```go
// convenience function so we don't have to type "if err != nil" a thousand times
func check(err error) {
  if err != nil {
    log.Panic(err)
  }
}

// define where we want our specs to be loaded from
loadOptions := LoadOptions{
  URIs: ["https://some.url/spec.yaml"],
  SearchCluster: true,
}

// load our specs and parse them into TroubleshootKinds
kinds, err := Load(LoadOptions)
check(err)

// Add defaults that the application implements (e.g support-bundle, preflights, some other project)
// Pre-populated somewhere in the application's project code. Constant? Dynamically loaded from config ??
// KOTS for example has default specs in its codebase - https://github.com/replicatedhq/kots/tree/main/pkg/supportbundle
// Troubleshoot has default redactors - https://github.com/replicatedhq/troubleshoot/blob/620fa75eb593247a07c4dc39ea96fc6a059be111/pkg/redact/redact.go#L161
defaults := TroubleshootKinds{}
defaults.CollectorsV1Beta2 = append(kinds.CollectorsV1Beta2, other.CollectorsV1Beta2...)
defaults.AnalyzersV1Beta2 = append(kinds.CollectorsV1Beta2, other.AnalyzersV1Beta2...)
defaults.RedactorsV1Beta2 = append(kinds.CollectorsV1Beta2, other.RedactorsV1Beta2...)

// Add them to the loaded specs
kinds := TroubleshootKinds{}
kinds.Add(defaults)

// define our options for collection
collectOptions := CollectOptions{
    Specs := *kinds,
}

// Implements the Bundler interface which
type bundle struct {
  FilePath string
}

// declare our new bundle
var newBundle bundle
// run collection for our bundle
err := newBundle.Collect(collectOptions)
check(err)

// now we can run analysis on our bundle
AnalysisResults,err := newBundle.Analyze(nil) // a nil AnalyzeOptions dictates that we want to adhere to the passed spec and _only_ to the spec. (no defaults)
// from here we can format and display the results however we want or create an archive of the bundle to share / download.
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
