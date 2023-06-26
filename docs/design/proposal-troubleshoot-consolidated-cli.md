# Consolidated Troubleshoot CLI

## Goals

- Consolidate all top level Troubleshoot commands (`support-bundle`, `preflight`, `analyze`, `redact`, and `sbctl`) into one exposing subcommands of all the functionality that was implemented by the previous commands.
- Ensure functional backward compatibility of newly introduces interfaces (CLI and Public APIs)

## Non-Goals

- Maintain backward compatibility of the CLI interface and the public APIs (new interfaces will be created)

## Background

- Top level troubleshoot commands (`support-bundle`, `preflight`, `analyze`, `redact`, and `sbctl`) are currently shipped standalone and require separate installation. This makes discovery and logistics of delivering these binaries an extra complexity that is unnecessary, albeit automatable. Consolidating these binaries into one addresses this problem quite well. Users are able to discover features that were otherwise hard to find such as serving a bundle using `sbctl` to run an API server.
- As Troubleshoot grows and gains new features that may lead to new subcommands, shipping these features would be hard if they lead to new binaries being created. If some of the new features target individual binaries e.g only `sbctl`, users who do not have this binary would not immediately have access to the new features.
- Troubleshoot top level commands have certain features that are quite similar. An example is the discussion about changing the behaviour of `preflight`, considering that preflights and support-bundles utilize the same specs - the same collectors and analyzers - and only differ in what is returned to the user post-analysis. For this design proposal, `support-bundle` and `preflight` may be condensed into a single `troubleshoot` binary.

### Previous related proposals
* https://github.com/replicatedhq/troubleshoot/blob/em/original-proposal-troubleshoot-cli/docs/design/proposal-sbctl-integration.md
* https://github.com/replicatedhq/troubleshoot/blob/em/original-proposal-troubleshoot-cli/docs/design/proposal-consolidate-collection.md


## High Level Design

Functions of `support-bundle`, `preflight`, `analyze`, `redact`, and `sbctl` binaries/tools should be rolled together into a single `troubleshoot` CLI that can perform all necessary functions.

`troubleshoot` CLI should be able to report on the version that is installed in the CLI, and any support bundle generated with `troubleshoot` should report its build/version inside the archive it generates.

## Detailed Design

In the interest of being able to work on this quickly without breaking existing use-cases, a new `troubleshoot` command should be created utilizing cobra and viper best practices from the cobra.dev docs.

To enable the new CLI to be written in a clean and DRY way, we should first address the need for a stable public API for troubleshoot.

A guiding principle of the design of the "Public API" should be that the we are defining a set of "artefacts" (i.e: a support bundle, a spec), and each of the defined public functions acts as an interface to interact with these artefacts, either creating, manipulating, or performing other actions on them. In this way we guarantee that each public function can be re-used on an artefact (support bundle) in any stage of it's existance.

For example, running a set of redactors or analyzers on an existing support bundle without having to re-run collection.

### Troubleshoot top-level commands to subcommands

All top level commands (`support-bundle`, `preflight`, `analyze`, `redact`, and `sbctl`) of the Troubleshoot project will now be subcommands e.g `sbctl` is planned to be `troubleshoot inspect`. Some flags and subcommands names may change to best fit completeness of the CLI interface e.g `--interactive=false` will be `--no-input`. Putting the CLI interface together should aim to follow best practises from well known guidelines such as https://clig.dev/ and other mature projects.

- `troubleshoot inspect` - This subcommand will be equivalent to the `sbctl` command.
- `troubleshoot collect` - This subcommand will be equivalent to the `support-bundle` command.
- `troubleshoot preflight` - This subcommand will be equivalent to the `preflight` command.
- `troubleshoot redact` - This subcommand will be equivalent to the `redact` command.
- `troubleshoot analyze` - This subcommand will be equivalent to the `analyze` command.

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
  * Testability of public APIs: With interfaces, we are able to test our contracts with ease because we are able to create stubs we can build our tests on.

The functionality we want to expose via this api is as follows.

_NOTE: These are not the actual APIs, rather pseudo code to better communicate intentions_

- `LoadSpecs()`
  - accepts a list of valid yaml documents and extracts all troubleshoot objects from the documents. This includes `Secret` and `ConfigMap` objects that have troubleshoot specs, and any other kinds that may be defined in the future.
  - validates that the loaded specs are valid troubleshoot specs. `CollectBundle()`, `AnalyzeBundle()` and `RedactBundle()` may perform the same validation done here for validate specs provided programmatically.
- `CollectBundle()`
  - is reponsible for running all provided collectors from a given preflight, support bundle or any other spec that contains collectors. Such would have been loaded by `LoadSpecs()` or programmatically provided. This includes host, remote or in-cluster collectors.
  - should provide as output a `bundle` object that exposes the collected bundle stored in memory or on disk. In this context `bundle` is be a programmable construct
- `LoadBundle()`
  - loads a bundle from disk, memory or a remote location into a `bundle` object that can be consumed by other APIs. The bundle _may_ be in various formats e.g archive, directory or some other future format we may define.
- `RedactBundle()`
  - redacts data from a bundle that was either collected by `CollectBundle()` or loaded by `LoadBundle()` from given redactors. Redactors would have been loaded by `LoadSpecs()` or programmatically provided.
- `AnalyzeBundle()`
  - is responsible for running analysers to analyse a `bundle` object. Analysers would have been loaded by `LoadSpecs()` or programmatically provided while the `bundle` would be been loaded by `LoadBundle()` or created by `CollectBundle()`
  - generates analysis results in a format that can be programatically consumed by upstream tools that can be used to render the results in UIs or store in a format that can be persisted e.g JSON, YAML etc
- `ArchiveBundle()`
  - generates an archive of the `bundle` object with a specified format e.g tar.
  - results in a generated archive that can be persisted, streamed or encoded into some other format e.g base64. Such behaviour will be driven by parameters provided to the API itself.
- `ServeBundle()`
  - serves a `bundle` object via a kubernetes API server for programmatic access by tools such as `kubectl` or the `client-go` library.

### CLI Usage patterns

This is a non-exhaustive list of usage patterns, but it should help guide how functionality from the current CLIs will be exposed in the new `troubleshoot` CLI.

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

  Alternatively the `--no-input` global flag will disable the spawning of a nested shell. This should behave much like the `sbctl serve` command does today, printing the path to it's temporary kubeconfig location to stdout. optionally taking flags to specify the kubeconfig location for use in advanced automation.

  `troubleshoot inspect --no-input support-bundle-12-12-2001.tar.gz`

  `troubleshoot inspect --no-input -o /path/to/kubeconfig support-bundle-12-12-2001.tar.gz`

- Redact an existing spec

  `troubleshoot redact support-bundle.tar.gz redactors.yaml...`

- Re-run analysers against an existing support bundle

  `troubleshoot analyze support-bundle.tar.gz spec.yaml... `

## Limitations

None

## Assumptions

- sbctl has no package naming conflicts with troubleshoot

## Testing

- As with all other CLIs in the project, this CLI will contain end-to-end tests
- The public APIs will have improved capability of testing scenarios that that were not possible thanks to usage of interfaces that allow creation of stubs where using external systems is not feasible.

## Documentation

- The public APIs will be documented in code with sample usage examples. The documentation needs to be optimised for consumption by `pkg.go.dev` documentation. The troubleshoot project resides https://pkg.go.dev/github.com/replicatedhq/troubleshoot
- A sample app with minimal functionality, but exercising most if not all of the APIs, will be created for testing purposes as well as for anyone willing to adopt the project.
- A section in troubleshoot.sh will be added where point users to our documentation. The section will have some introduction of the SDK and the CLI.

## Alternatives Considered

None

## Security Implications

None
