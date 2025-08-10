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

The LLM analyzer uses OpenAI to automatically analyze Kubernetes logs and identify issues. It understands context, finds root causes, and correlates problems across multiple components.

### What's Different

- **No rules to write** - AI understands logs and errors automatically
- **Finds root causes** - Identifies why problems occur, not just symptoms  
- **Correlates issues** - Understands relationships (e.g., DB crash → app failures)
- **Natural language** - Describe problems in plain English

### Setup

1. **Get an OpenAI API key** from [platform.openai.com](https://platform.openai.com)
2. **Create a `.env` file**:
   ```bash
   echo 'OPENAI_API_KEY=sk-...' > .env
   ```
   The tool automatically loads `.env` files.

### How to Use

Add the `llm` analyzer to your spec:

```yaml
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
spec:
  collectors:
    - logs:
        name: app-logs
        namespace: default
  analyzers:
    - llm:
        checkName: "AI Analysis"
        collectorName: "app-logs"
        fileName: "**/*.log"  # Use **/* for nested dirs
        model: "gpt-4o-mini"  # Cost-effective, ~$0.01 per analysis
        outcomes:
          - fail:
              when: "issue_found"
              message: "Found: {{.Summary}}"
          - pass:
              message: "No issues detected"
```

Run with problem description:
```bash
./bin/support-bundle spec.yaml --problem-description "App keeps crashing"
```

Or re-analyze existing bundles:
```bash
./bin/analyze bundle.tar.gz --analyzers spec.yaml
```

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
