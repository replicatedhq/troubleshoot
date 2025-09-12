## preflight template

Render a templated preflight spec with values

### Synopsis

Process a templated preflight YAML file, substituting variables and removing conditional sections based on provided values. Supports multiple values files and inline overrides. Outputs the fully-resolved YAML (no conditional logic remains).

```
preflight template [template-file] [flags]
```

### Examples

```
# Render with defaults only
preflight template sample-preflight-templated.yaml

# Render with multiple values files (later files override earlier ones)
preflight template sample-preflight-templated.yaml \
  --values values-base.yaml --values values-prod.yaml

# Inline overrides (Helm-style --set)
preflight template sample-preflight-templated.yaml \
  --set kubernetes.minVersion=v1.24.0 --set storage.enabled=true

# Save to file
preflight template sample-preflight-templated.yaml -o rendered.yaml
```

### Options

```
      --values stringArray   Path to YAML files containing template values (can be used multiple times)
      --set stringArray      Set template values on the command line (can be used multiple times)
  -o,  --output string       Output file (default: stdout)
```

### Behavior

- Values merge: deep-merged left-to-right across multiple `--values` files. `--set` overrides win last.
- Rendering engine:
  - v1beta3 specs (Helm-style templates using `.Values.*`) are rendered with the Helm engine.
  - Legacy templates are rendered with Go text/template; mixed templates are supported.
- Map normalization: values files are normalized to `map[string]interface{}` before applying `--set` (avoids type errors when merging Helm `strvals`).

### v1beta3 spec decisions

- Layout aligns with v1beta2: `spec.analyzers: [...]`.
- Each analyzer accepts an optional `docString` used by `preflight docs`.
- Templating style is Helm-oriented (`.Values.*`).
- Modularity via conditional analyzers is supported, e.g. `{{- if .Values.ingress.enabled }}`.

### SEE ALSO

* [preflight](preflight.md)  - Run and retrieve preflight checks in a cluster
