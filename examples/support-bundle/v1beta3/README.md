# v1beta3 Support Bundle Examples

This directory contains example Support Bundle specs using the v1beta3 API, which introduces `StringOrValueFrom` support for securely referencing Kubernetes Secrets and ConfigMaps in collector fields.

## Features

### StringOrValueFrom Pattern

The v1beta3 API introduces a Kubernetes-native pattern for referencing sensitive values:

```yaml
uri:
  valueFrom:
    secretKeyRef:
      name: my-secret
      key: connection-uri
```

or

```yaml
uri: "postgresql://localhost:5432/db"  # Literal value
```

### Supported Collectors

Currently, v1beta3 supports `StringOrValueFrom` for:

- **Database collectors**: `postgres`, `mysql`, `redis`, `mssql`
  - `uri` field - Connection strings from secrets
  - `tls` fields - CA cert, client cert, and client key from secrets

## Examples

### 1. postgres-with-secret.yaml
Basic PostgreSQL collector with connection URI from a secret.

**Use case**: Securely store database credentials without hardcoding them in the spec.

```bash
kubectl apply -f postgres-with-secret.yaml
```

### 2. postgres-with-tls.yaml
PostgreSQL with TLS configuration from secrets.

**Use case**: Secure database connections with mutual TLS, storing certificates in secrets.

```bash
kubectl apply -f postgres-with-tls.yaml
```

### 3. multiple-databases.yaml
Multiple database collectors (PostgreSQL, MySQL, Redis, MSSQL) with various configurations.

**Use case**: Collect diagnostics from multiple databases in your application stack.

```bash
kubectl apply -f multiple-databases.yaml
```

### 4. cross-namespace-secrets.yaml
Accessing secrets from different namespaces.

**Use case**: Centralized credential management in a shared namespace.

```bash
kubectl apply -f cross-namespace-secrets.yaml
```

**RBAC Requirements**: The support bundle service account needs `get` permission on secrets in the referenced namespaces.

### 5. optional-secrets.yaml
Using the `optional` field for graceful degradation.

**Use case**: Collect diagnostics even when some credentials are unavailable (e.g., optional secondary databases).

```bash
kubectl apply -f optional-secrets.yaml
```

### 6. configmap-example.yaml
Using ConfigMaps for non-sensitive configuration.

**Use case**: Store non-sensitive connection strings (e.g., development databases) in ConfigMaps.

```bash
kubectl apply -f configmap-example.yaml
```

## Key Concepts

### Secret vs ConfigMap

- **Secrets**: Use for sensitive data (passwords, tokens, certificates)
- **ConfigMaps**: Use for non-sensitive configuration (development endpoints, feature flags)

### Optional Field

```yaml
uri:
  valueFrom:
    secretKeyRef:
      name: my-secret
      key: uri
      optional: true  # Returns empty string if secret/key doesn't exist
```

- `optional: false` (default): Collection fails if secret is missing
- `optional: true`: Returns empty string if secret/key is missing

### Cross-Namespace Access

```yaml
uri:
  valueFrom:
    secretKeyRef:
      name: shared-secret
      key: uri
      namespace: other-namespace  # Access secrets in different namespaces
```

If `namespace` is not specified, uses the support bundle's namespace.

### Backward Compatibility

v1beta3 maintains backward compatibility with v1beta2 TLS configuration:

```yaml
tls:
  secret:  # v1beta2 style
    name: tls-secret
    namespace: default
```

## RBAC Configuration

Support bundles need appropriate RBAC permissions to read secrets:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: troubleshoot-secret-reader
rules:
- apiGroups: [""]
  resources: ["secrets"]
  resourceNames: ["postgres-connection", "redis-creds"]  # Restrict to specific secrets
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: troubleshoot-secret-reader-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: troubleshoot-secret-reader
subjects:
- kind: ServiceAccount
  name: troubleshoot
  namespace: default
```

## Migration from v1beta2

### Before (v1beta2):
```yaml
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
spec:
  collectors:
    - postgres:
        uri: "postgresql://user:password@host:5432/db"  # Hardcoded
```

### After (v1beta3):
```yaml
apiVersion: troubleshoot.sh/v1beta3
kind: SupportBundle
spec:
  collectors:
    - postgres:
        uri:
          valueFrom:
            secretKeyRef:
              name: postgres-connection
              key: connection-uri
```

## Limitations

1. **No value composition**: Cannot combine multiple secrets into a single value
   ```yaml
   # NOT SUPPORTED
   uri: "postgresql://$(USERNAME):$(PASSWORD)@host:5432/db"
   ```
   Store the complete connection string in a single secret key.

2. **Collector scope**: Only database collectors support `StringOrValueFrom` initially
   - Future versions will extend to HTTP, Data, and other collectors

3. **No templating**: The entire field value comes from one source

## Security Best Practices

1. **Use resourceNames in RBAC**: Restrict access to specific secrets
2. **Separate secrets**: Don't reuse secrets across applications
3. **Rotate credentials**: Update secrets regularly
4. **Audit access**: Monitor secret access logs
5. **Redact output**: Ensure connection strings are redacted in bundle output

## Troubleshooting

### Error: "failed to get secret default/my-secret"

- **Cause**: Secret doesn't exist or RBAC denied access
- **Solution**: Verify secret exists: `kubectl get secret my-secret`
- **Solution**: Check RBAC: `kubectl auth can-i get secret/my-secret`

### Error: "key 'uri' not found in secret"

- **Cause**: Secret exists but doesn't contain the specified key
- **Solution**: Check secret keys: `kubectl get secret my-secret -o jsonpath='{.data}'`

### Error: "cannot specify both 'value' and 'valueFrom'"

- **Cause**: Both literal value and secret reference provided
- **Solution**: Use only one: either `value: "string"` or `valueFrom: {...}`

## Additional Resources

- [Troubleshoot Documentation](https://troubleshoot.sh)
- [v1beta3 API Reference](https://troubleshoot.sh/docs/v1beta3/)
- [Kubernetes Secrets](https://kubernetes.io/docs/concepts/configuration/secret/)
- [RBAC Authorization](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
