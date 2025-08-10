# Replicated Troubleshoot

Replicated Troubleshoot is a framework for collecting, redacting, and analyzing highly customizable diagnostic information about a Kubernetes cluster. Troubleshoot specs are created by 3rd-party application developers/maintainers and run by cluster operators in the initial and ongoing operation of those applications.

Troubleshoot provides two CLI tools as kubectl plugins (using [Krew](https://krew.dev)): `kubectl preflight` and `kubectl support-bundle`. Preflight provides pre-installation cluster conformance testing and validation (preflight checks) and support-bundle provides post-installation troubleshooting and diagnostics (support bundles).

To know more about troubleshoot, please visit: https://troubleshoot.sh/

## Preflight Checks
Preflight checks are an easy-to-run set of conformance tests that can be written to verify that specific requirements in a cluster are met.

To run a sample preflight check from a sample application, install the preflight kubectl plugin:

```
curl https://krew.sh/preflight | bash
```
 and run, where https://preflight.replicated.com provides an **example** preflight spec:

```
kubectl preflight https://preflight.replicated.com
```

**NOTE** this is an example. Do **not** use to validate real scenarios.

For more details on creating the custom resource files that drive preflight checks, visit [creating preflight checks](https://troubleshoot.sh/docs/preflight/introduction/).


## Support Bundle
A support bundle is an archive that's created in-cluster, by collecting logs and cluster information, and executing specified commands (including redaction of sensitive information). After creating a support bundle, the cluster operator will normally deliver it to the 3rd-party application vendor for analysis and disconnected debugging. Another Replicated project, [KOTS](https://github.com/replicatedhq/kots), provides k8s apps an in-cluster UI for processing support bundles and viewing analyzers (as well as support bundle collection).

To collect a sample support bundle, install the troubleshoot kubectl plugin:

```
curl https://krew.sh/support-bundle | bash
```
 and run, where https://support-bundle.replicated.com provides an **example** support bundle spec:

```
kubectl support-bundle https://support-bundle.replicated.com
```

**NOTE** this is an example. Do **not** use to validate real scenarios.

For more details on creating the custom resource files that drive support-bundle collection, visit [creating collectors](https://troubleshoot.sh/docs/collect/) and [creating analyzers](https://troubleshoot.sh/docs/analyze/).

And see our other tool [sbctl](https://github.com/replicatedhq/sbctl) that makes it easier to interact with support bundles using `kubectl` commands you already know

## LLM Analyzer (AI-Powered Analysis)

The LLM analyzer uses artificial intelligence to automatically analyze support bundle contents and identify issues without pre-written deterministic rules. This enables dynamic problem detection based on actual log patterns and system state.

### Prerequisites

- OpenAI API key set as environment variable: `export OPENAI_API_KEY=your-api-key`
- Supported models: `gpt-5`, `gpt-4o-mini`, `gpt-4o`, `gpt-4-turbo`, `gpt-3.5-turbo`

### Basic Usage

1. **Add LLM analyzer to your spec:**

```yaml
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: my-app-support
spec:
  collectors:
    - logs:
        name: pod-logs
        namespace: default
  analyzers:
    - llm:
        checkName: "AI Problem Detection"
        collectorName: "pod-logs"
        fileName: "*.log"
        # model defaults to gpt-4o-mini, override for specific needs:
        # model: "gpt-5"  # For complex issues requiring advanced reasoning
        # useStructuredOutput: true  # Default: true for supported models (gpt-4o, gpt-4o-mini)
        outcomes:
          - fail:
              when: "issue_found"
              message: "Issue detected: {{.Summary}}"
          - warn:
              when: "potential_issue"
              message: "Warning: {{.Summary}}"
          - pass:
              message: "No issues detected"
```

2. **Run with problem description:**

```bash
# Interactive mode (prompts for problem description)
kubectl support-bundle ./your-spec.yaml

# With problem description flag
kubectl support-bundle ./your-spec.yaml --problem-description "My pods keep crashing and restarting"
```

### Re-analyzing Existing Bundles

You can re-analyze previously collected support bundles with different problem descriptions:

```bash
# Interactive prompt for problem description
kubectl support-bundle analyze --bundle support-bundle-2024-01-20T10-30-00.tar.gz

# With specific problem description
kubectl support-bundle analyze --bundle support-bundle-2024-01-20T10-30-00.tar.gz \
  --problem-description "Application performance is degraded"
```

### Configuration Options

- **model**: AI model to use (default: gpt-4o-mini, others: gpt-5, gpt-4o, etc.)
- **maxFiles**: Maximum number of files to analyze (default: 20)
- **maxSize**: Maximum total size of files in KB (default: 1024KB/1MB)
- **fileName**: Pattern to match files (e.g., "*.log", "error-*")
- **collectorName**: Name of the collector to analyze files from
- **exclude**: Boolean to exclude this analyzer (default: false)

#### Smart File Selection (New)
- **priorityPatterns**: Keywords to prioritize (default: error, fatal, exception, panic, crash, OOM)
- **skipPatterns**: File patterns to skip (default: images and archives)
- **preferRecent**: Prioritize recent files based on timestamps (default: false)

#### Structured Output (New)
- **useStructuredOutput**: Use OpenAI's structured outputs for guaranteed valid JSON (default: true for gpt-4o/gpt-4o-mini, false for older models)

#### Advanced Configuration
- **problemDescription**: Set problem description in the analyzer spec instead of using CLI flag
- **apiEndpoint**: Override API endpoint for testing, proxies, or different regions (default: https://api.openai.com/v1/chat/completions)

### Model Selection Guide

- **gpt-4o-mini**: (Default) Cost-effective with 128K context window, recommended for most use cases ($0.15/1M input tokens)
- **gpt-5**: Most advanced model for complex issues requiring cutting-edge reasoning (pricing TBD)
- **gpt-4o**: Latest GPT-4 Omni model with 128K context, excellent for complex analysis ($2.50/1M input tokens)
- **gpt-4-turbo**: Previous generation, still very capable ($10/1M input tokens)
- **gpt-3.5-turbo**: Budget option for simple analysis ($0.50/1M input tokens) - Note: Does not support structured outputs

### Enhanced Output

The LLM analyzer now provides structured, actionable output including:
- **Root Cause Analysis**: Identified root cause of the problem
- **Recommended Commands**: kubectl commands to resolve issues
- **Affected Resources**: List of impacted pods and services
- **Next Steps**: Ordered action items
- **Documentation Links**: Relevant Kubernetes documentation
- **Related Issues**: Other potential problems found

Template variables available in outcome messages:
- `{{.Summary}}`, `{{.Issue}}`, `{{.Solution}}`, `{{.RootCause}}`
- `{{.Commands}}`, `{{.AffectedPods}}`, `{{.NextSteps}}`
- `{{.Severity}}`, `{{.Confidence}}`

### Examples

See [examples/analyzers/llm-analyzer.yaml](examples/analyzers/llm-analyzer.yaml) for complete examples including:
- Using LLM analyzer alongside traditional analyzers
- Re-analyzing existing bundles
- Different model configurations
- Smart file selection with priority patterns
- Enhanced output templates

# Community

For questions about using Troubleshoot, how to contribute and engaging with the project in any other way, please refer to the following resources and channels.

- [Replicated Community](https://help.replicated.com/community) forum
- [#app-troubleshoot channel in Kubernetes Slack](https://kubernetes.slack.com/channels/app-troubleshoot)
- [#Community meetings calendar](https://calendar.google.com/calendar/u/0?cid=Y19mMGx1aGhiZGtscGllOGo5dWpicXMwNnN1a0Bncm91cC5jYWxlbmRhci5nb29nbGUuY29t). This happen monthly but dates may change and would be kept upto date in the calendar.

# Software Bill of Materials
A signed SBOM  that includes Troubleshoot dependencies is included in each release.
- **troubleshoot-sbom.tgz** contains a software bill of materials for Troubleshoot.
- **troubleshoot-sbom.tgz.sig** is the digital signature for troubleshoot-sbom.tgz
- **key.pub** is the public key from the key pair used to sign troubleshoot-sbom.tgz

The following example illustrates using [cosign](https://github.com/sigstore/cosign) to verify that **troubleshoot-sbom.tgz** has
not been tampered with.
```sh
$ cosign verify-blob --key key.pub --signature troubleshoot-sbom.tgz.sig troubleshoot-sbom.tgz
Verified OK
```

If you were to get an error similar to the one below, it means you are verifying an SBOM signed using cosign `v1` using a newer `v2` of the binary. This version introduced [breaking changes](https://github.com/sigstore/cosign/blob/main/CHANGELOG.md#breaking-changes) which require an additional flag `--insecure-ignore-tlog=true` to successfully verify SBOMs like so.
```sh
$ cosign verify-blob --key key.pub --signature troubleshoot-sbom.tgz.sig troubleshoot-sbom.tgz --insecure-ignore-tlog=true
WARNING: Skipping tlog verification is an insecure practice that lacks of transparency and auditability verification for the blob.
Verified OK
```
