## preflight docs

Extract and display documentation from a preflight spec

### Synopsis

Extract all `docString` fields from enabled analyzers in one or more preflight YAML files. Templating is evaluated first using the provided values, so only documentation for analyzers that are enabled is emitted. The output is Markdown.

```
preflight docs [preflight-file...] [flags]
```

### Examples

```
# Extract docs with defaults
preflight docs ml-platform-preflight.yaml

# Multiple specs with values files (later values override earlier ones)
preflight docs spec1.yaml spec2.yaml \
  --values values-base.yaml --values values-prod.yaml

# Inline overrides (Helm-style --set)
preflight docs ml-platform-preflight.yaml \
  --set monitoring.enabled=true --set ingress.enabled=false

# Save to file
preflight docs ml-platform-preflight.yaml -o requirements.md
```

### Options

```
      --values stringArray   Path to YAML files containing template values (can be used multiple times)
      --set stringArray      Set template values on the command line (can be used multiple times)
  -o,  --output string       Output file (default: stdout)
```

### Behavior

- Accepts one or more preflight specs; all are rendered, and their docStrings are concatenated in input order.
- Values merge: deep-merged left-to-right across `--values` files. `--set` overrides win last.
- Rendering engine:
  - If a spec references `.Values`, it is rendered with the Helm engine; otherwise Go text/template is used. A fallback to the legacy engine is applied for mixed templates.
- Map normalization: values maps are normalized to `map[string]interface{}` before applying `--set` to avoid type errors.
- Markdown formatting:
  - The first line starting with `Title:` in a `docString` becomes a Markdown heading.
  - If no `Title:` is present, the analyzer (or requirement) name is used.
  - Sections are separated by blank lines.

### v1beta3 docString extraction

- v1beta3 layout uses `spec.analyzers: [...]`.
- Each analyzer may include a sibling `docString` string.
- The docs command extracts `spec.analyzers[*].docString` after rendering.
- Backward compatibility: legacy `requirements` blocks are still supported and extracted when present.

### SEE ALSO

* [preflight](preflight.md)  - Run and retrieve preflight checks in a cluster
