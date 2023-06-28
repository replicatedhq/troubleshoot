## Parse troubleshoot specs from a helm chart

This is an example of using troubleshoot's `loader.LoadSpecs` API to load specs from rendered manifests in a helm chart. The manifests are rendered with the equivalent of `helm template --values values.yaml` where the output is a YAML multidoc. `loader.LoadSpecs` will take the YAML multidoc as an input and extract troubleshoot specs inside kuberenetes `Secrets` and `ConfigMap`s, and troubleshoot custom resources.

This application always uses the local version of troubleshoot so as to build using the latest version of the library. This ensures that the examples is kept in sync with new features.

### Running

```go
go mod tidy     # sync go modules
go run main.go  # run the application. This should print out a YAML multidoc of the loaded troubleshoot specs
```
