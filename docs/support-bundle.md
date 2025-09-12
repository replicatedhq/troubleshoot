## support-bundle

Generate a support bundle from a Kubernetes cluster or specified sources

### Synopsis

Generate a support bundle, an archive containing files, output, metrics, and cluster state to aid in troubleshooting Kubernetes clusters.

If no arguments are provided, specs are automatically loaded from the cluster by default.

**Argument Types**:
1. **Secret**: Load specs from a Kubernetes Secret. Format: "secret/namespace-name/secret-name[/data-key]"
2. **ConfigMap**: Load specs from a Kubernetes ConfigMap. Format: "configmap/namespace-name/configmap-name[/data-key]"
3. **File**: Load specs from a local file. Format: Local file path
4. **Standard Input**: Read specs from stdin. Format: "-"
5. **URL**: Load specs from a URL. Supports HTTP and OCI registry URLs.

```
support-bundle [urls...] [flags]
```

### Auto-update

Support-bundle includes a built-in self-updater.

- Default behavior: On startup, it checks the latest release on `replicatedhq/troubleshoot`. If a newer version exists, it downloads and atomically replaces the running binary. Dev builds without a version skip auto-update.
- Control via flag: `--auto-update` (default true). Disable with `--auto-update=false`.
- Control via env: `TROUBLESHOOT_AUTO_UPDATE=true|false` (empty = default true).
- Logging: use `--v=1` to see updater messages (e.g., "Updating …", "Update complete.").
- Airgap/network errors: if the release API or download is unreachable, it continues without updating.

### Options

```
      --as string                      Username to impersonate for the operation. User could be a regular user or a service account in a namespace.
      --as-group stringArray           Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --as-uid string                  UID to impersonate for the operation.
      --cache-dir string               Default cache directory (default "$HOME/.kube/cache")
      --certificate-authority string   Path to a cert file for the certificate authority
      --client-certificate string      Path to a client certificate file for TLS
      --client-key string              Path to a client key file for TLS
      --cluster string                 The name of the kubeconfig cluster to use
      --collect-without-permissions    always generate a support bundle, even if it some require additional permissions (default true)
      --context string                 The name of the kubeconfig context to use
      --cpuprofile string              File path to write cpu profiling data
      --debug                          enable debug logging. This is equivalent to --v=0
      --disable-compression            If true, opt-out of response compression for all requests to the server
      --dry-run                        print support bundle spec without collecting anything
  -h, --help                           help for support-bundle
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --interactive                    enable/disable interactive mode (default true)
      --kubeconfig string              Path to the kubeconfig file to use for CLI requests.
      --load-cluster-specs             enable/disable loading additional troubleshoot specs found within the cluster. This is the default behavior if no spec is provided as an argument
      --memprofile string              File path to write memory profiling data
  -n, --namespace string               If present, the namespace scope for this CLI request
      --no-uri                         When this flag is used, Troubleshoot does not attempt to retrieve the spec referenced by the uri: field`
  -o, --output string                  specify the output file path for the support bundle
      --redact                         enable/disable default redactions (default true)
      --redactors strings              names of the additional redactors to use
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -l, --selector strings               selector to filter on for loading additional support bundle specs found in secrets within the cluster (default [troubleshoot.sh/kind=support-bundle])
  -s, --server string                  The address and port of the Kubernetes API server
      --since string                   force pod logs collectors to return logs newer than a relative duration like 5s, 2m, or 3h.
      --since-time string              force pod logs collectors to return logs after a specific date (RFC3339)
      --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
  -v, --v Level                        number for the log level verbosity
```

### SEE ALSO

* [support-bundle analyze](support-bundle_analyze.md)	 - analyze a support bundle
* [support-bundle redact](support-bundle_redact.md)	 - Redact information from a generated support bundle archive
* [support-bundle version](support-bundle_version.md)	 - Print the current version and exit

###### Auto generated by spf13/cobra on 23-Aug-2024
