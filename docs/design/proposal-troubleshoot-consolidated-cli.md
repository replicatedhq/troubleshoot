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

- TODO: specify the public APIs we expect users of Troubleshoot to consume, e.g.
  - collect()
  - redact()
  - analyze()

In the interest of being able to work on this quickly without breaking existing use-cases, a new `troubleshoot` command should be created. Utilizing cobra and viper best practices from the cobra.dev docs.

sbctl should be migrated to the troubleshoot repository in a "lift and shift" operation to start with, allowing for it's methods and functions to be called by the new `troubleshoot` command.

### Public APIs

The existing package entrypoint we use from the support-bundle command is `supportbundle.CollectSupportBundleFromSpec()`
which is an end-to-end function that runs collectors, redactors and then analysis.

The Consolidated cli should be responsible for handling the end-to-end running of this process in the `cmd` leaving the importable `pkg` free to focus on defining the individual collect, redact analyze steps.

this allows us to move functions out from behind `CollectSupportBundleFromSpec` so they can be easier called from other projects importing troubleshoot.

those functions could be then called as:

- `troubleshoot.Collect()`
- `troubleshoot.Redact()`
- `troubleshoot.Analyze()`

This is a breaking change for existing upstream projects, many of which will directly call `CollectSupportBundleFromSpec`

To proceed with this change without breaking impacting the exinsting CLI the new `troubleshoot` command should target the underlying functions, essentially replicating the functionality of `CollectSupportBundleFromSpec`. which can be deprecated once we have a stable replacement.

### Usage patterns

- generate a support bundle

  `troubleshoot collect supportbundle.yaml`

  `troubleshoot collect supportbundle.yaml secrets/default/kotsadm-appslug-supportbundle`

  `troubleshoot collect https://kots.io`

- use a spec to return a go/no-go preflight outcome

  `troubleshoot preflight spec.yaml`

  This should not only clearly state any reasons for failing, but also use standardised exit codes that can be used by automation tools.

- Interact with an existing support bundle with kubectl.

  `troubleshoot inspect support-bundle-12-12-2001.tar.gz`

  This should set the `KUBECONFIG` environment variable, as well as a `TROUBLESHOOT_SHELL` environment variable before spawning a subshell.
  This is to allow prompts and shell environments to be able to detect that they're running in a troubleshoot spawned shell, much like tmux et-al.

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

Use "troubleshoot [command] --help" for more information about a command.
```

Collect command:

```
Collect a support bundle, which is an archive of files, output, metrics and state
from a server that can be used to assist when troubleshooting a Kubernetes cluster.

The collected information is analyzed and private information redacted according to the spec.

If a URL is not provided, you must either specify --load-cluster-specs, or use "-" to
load the spec from stdin.

Usage:
  troubleshoot collect [urls...] [flags] [-]

Flags:
      --redact                         enable/disable default redactions (default true)
      --redactors strings              names of the additional redactors to use
      --load-cluster-specs             enable/disable loading additional troubleshoot specs found within the cluster. required when no specs are provided on the command line
      --since string                   force pod logs collectors to return logs newer than a relative duration like 5s, 2m, or 3h.
      --since-time string              force pod logs collectors to return logs after a specific date (RFC3339)
  -l, --spec-labels strings               selector to filter on for loading additional support bundle specs found in secrets within the cluster (default [troubleshoot.io/kind=support-bundle])

Global Flags:
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
  -o, --output string                  specify the output file path for the support bundle
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                  The address and port of the Kubernetes API server
      --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
  -v, --v Level                        number for the log level verbosity
```

Preflight:

```
A preflight check is a set of validations that can and should be run to ensure
that a cluster meets the requirements to run an application.

Usage:
  troubleshoot preflight [urls...] [flags]

Flags:
      --node-selector string           selector (label query) to filter remote collection nodes on.

Global Flags:
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
  -o, --output string                  specify the output file path for the support bundle
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                  The address and port of the Kubernetes API server
      --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
  -v, --v Level                        number for the log level verbosity
```

## Limitations



## Assumptions

- sbctl has no package naming conflicts with troubleshoot

## Testing

## Documentation

## Alternatives Considered

## Security Implications