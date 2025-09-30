package preflight

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/loader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const v1beta3Template = `apiVersion: troubleshoot.sh/v1beta3
kind: Preflight
metadata:
  name: templated-from-v1beta2
spec:
  analyzers:
    {{- if .Values.kubernetes.enabled }}
    - docString: |
        Title: Kubernetes Control Plane Requirements
        Requirement:
          - Version:
            - Minimum: {{ .Values.kubernetes.minVersion }}
            - Recommended: {{ .Values.kubernetes.recommendedVersion }}
          - Docs: https://kubernetes.io
        These version targets ensure that required APIs and default behaviors are
          available and patched. Moving below the minimum commonly removes GA APIs
          (e.g., apps/v1 workloads, storage and ingress v1 APIs), changes admission
          defaults, and lacks critical CVE fixes. Running at or above the recommended
          version matches what is exercised most extensively in CI and receives the
          best operational guidance for upgrades and incident response.
      clusterVersion:
        checkName: Kubernetes version
        outcomes:
          - fail:
              when: '< {{ .Values.kubernetes.minVersion }}'
              message: This application requires at least Kubernetes {{ .Values.kubernetes.minVersion }}, and recommends {{ .Values.kubernetes.recommendedVersion }}.
              uri: https://www.kubernetes.io
          - warn:
              when: '< {{ .Values.kubernetes.recommendedVersion }}'
              message: Your cluster meets the minimum version of Kubernetes, but we recommend you update to {{ .Values.kubernetes.recommendedVersion }} or later.
              uri: https://kubernetes.io
          - pass:
              when: '>= {{ .Values.kubernetes.recommendedVersion }}'
              message: Your cluster meets the recommended and required versions of Kubernetes.
    {{- end }}
    {{- if .Values.ingress.enabled }}
    - docString: |
        Title: Required CRDs and Ingress Capabilities
        Requirement:
          - Ingress Controller: Contour
          - CRD must be present:
            - Group: heptio.com
            - Kind: IngressRoute
            - Version: v1beta1 or later served version
        The ingress layer terminates TLS and routes external traffic to Services.
          Contour relies on the IngressRoute CRD to express host/path routing, TLS
          configuration, and policy. If the CRD is not installed and served by the
          API server, Contour cannot reconcile desired state, leaving routes
          unconfigured and traffic unreachable.
      {{- if eq .Values.ingress.type "Contour" }}
      customResourceDefinition:
        checkName: Contour IngressRoute CRD
        customResourceDefinitionName: ingressroutes.contour.heptio.com
        outcomes:
          - fail:
              message: Contour IngressRoute CRD not found; required for ingress routing
          - pass:
              message: Contour IngressRoute CRD present
      {{- end }}
    {{- end }}
    {{- if .Values.runtime.enabled }}
    - docString: |
        Title: Container Runtime Requirements
        Requirement:
          - Runtime: containerd (CRI)
          - Kubelet cgroup driver: systemd
          - CRI socket path: /run/containerd/containerd.sock
        containerd (via the CRI) is the supported runtime for predictable container
          lifecycle management. On modern distros (cgroup v2), kubelet and the OS must
          both use the systemd cgroup driver to avoid resource accounting mismatches
          that lead to unexpected OOMKills and throttling. The CRI socket path must
          match kubelet configuration so the node can start and manage pods.
      containerRuntime:
        outcomes:
          - pass:
              when: '== containerd'
              message: containerd runtime detected
          - fail:
              message: Unsupported container runtime; containerd required
    {{- end }}
    {{- if .Values.storage.enabled }}
    - docString: |
        Title: Default StorageClass Requirements
        Requirement:
          - A StorageClass named "{{ .Values.storage.className }}" must exist (cluster default preferred)
          - AccessMode: ReadWriteOnce (RWO) required (RWX optional)
          - VolumeBindingMode: WaitForFirstConsumer preferred
          - allowVolumeExpansion: true recommended
        A default StorageClass enables dynamic PVC provisioning without manual
          intervention. RWO provides baseline persistence semantics for stateful pods.
          WaitForFirstConsumer defers binding until a pod is scheduled, improving
          topology-aware placement (zonal/az) and reducing unschedulable PVCs.
          AllowVolumeExpansion permits online growth during capacity pressure
          without disruptive migrations.
      storageClass:
        checkName: Default StorageClass
        storageClassName: '{{ .Values.storage.className }}'
        outcomes:
          - fail:
              message: Default StorageClass not found
          - pass:
              message: Default StorageClass present
    {{- end }}
    {{- if .Values.distribution.enabled }}
    - docString: |
        Title: Kubernetes Distribution Support
        Requirement:
          - Unsupported: docker-desktop, microk8s, minikube
          - Supported: eks, gke, aks, kurl, digitalocean, rke2, k3s, oke, kind
        Development or single-node environments are optimized for local testing and
          omit HA control-plane patterns, cloud integration, and production defaults.
          The supported distributions are validated for API compatibility, RBAC
          expectations, admission behavior, and default storage/networking this
          application depends on.
      distribution:
        outcomes:
          - fail:
              when: '== docker-desktop'
              message: The application does not support Docker Desktop Clusters
          - fail:
              when: '== microk8s'
              message: The application does not support Microk8s Clusters
          - fail:
              when: '== minikube'
              message: The application does not support Minikube Clusters
          - pass:
              when: '== eks'
              message: EKS is a supported distribution
          - pass:
              when: '== gke'
              message: GKE is a supported distribution
          - pass:
              when: '== aks'
              message: AKS is a supported distribution
          - pass:
              when: '== kurl'
              message: KURL is a supported distribution
          - pass:
              when: '== digitalocean'
              message: DigitalOcean is a supported distribution
          - pass:
              when: '== rke2'
              message: RKE2 is a supported distribution
          - pass:
              when: '== k3s'
              message: K3S is a supported distribution
          - pass:
              when: '== oke'
              message: OKE is a supported distribution
          - pass:
              when: '== kind'
              message: Kind is a supported distribution
          - warn:
              message: Unable to determine the distribution of Kubernetes
    {{- end }}
    {{- if .Values.nodeChecks.count.enabled }}
    - docString: |
        Title: Node count requirement
        Requirement:
          - Node count: Minimum {{ .Values.cluster.minNodes }} nodes, Recommended {{ .Values.cluster.recommendedNodes }} nodes
        Multiple worker nodes provide scheduling capacity, tolerance to disruptions,
          and safe rolling updates. Operating below the recommendation increases risk
          of unschedulable pods during maintenance or failures and reduces headroom
          for horizontal scaling.
      nodeResources:
        checkName: Node count
        outcomes:
          - fail:
              when: 'count() < {{ .Values.cluster.minNodes }}'
              message: This application requires at least {{ .Values.cluster.minNodes }} nodes.
              uri: https://kurl.sh/docs/install-with-kurl/adding-nodes
          - warn:
              when: 'count() < {{ .Values.cluster.recommendedNodes }}'
              message: This application recommends at least {{ .Values.cluster.recommendedNodes }} nodes.
              uri: https://kurl.sh/docs/install-with-kurl/adding-nodes
          - pass:
              message: This cluster has enough nodes.
    {{- end }}
    {{- if .Values.nodeChecks.cpu.enabled }}
    - docString: |
        Title: Cluster CPU requirement
        Requirement:
          - Total CPU: Minimum {{ .Values.cluster.minCPU }} vCPU
        Aggregate CPU must cover system daemons, controllers, and application pods.
          Insufficient CPU causes prolonged scheduling latency, readiness probe
          failures, and throughput collapse under load.
      nodeResources:
        checkName: Cluster CPU total
        outcomes:
          - fail:
              when: 'sum(cpuCapacity) < {{ .Values.cluster.minCPU }}'
              message: The cluster must contain at least {{ .Values.cluster.minCPU }} cores
              uri: https://kurl.sh/docs/install-with-kurl/system-requirements
          - pass:
              message: There are at least {{ .Values.cluster.minCPU }} cores in the cluster
    {{- end }}
    {{- if .Values.nodeChecks.memory.enabled }}
    - docString: |
        Title: Per-node memory requirement
        Requirement:
          - Per-node memory: Minimum {{ .Values.node.minMemoryGi }} GiB; Recommended {{ .Values.node.recommendedMemoryGi }} GiB
        Nodes must reserve memory for kubelet/system components and per-pod overhead.
          Below the minimum, pods will frequently be OOMKilled or evicted. The
          recommended capacity provides headroom for spikes, compactions, and
          upgrades without destabilizing workloads.
      nodeResources:
        checkName: Per-node memory requirement
        outcomes:
          - fail:
              when: 'min(memoryCapacity) < {{ .Values.node.minMemoryGi }}Gi'
              message: All nodes must have at least {{ .Values.node.minMemoryGi }} GiB of memory.
              uri: https://kurl.sh/docs/install-with-kurl/system-requirements
          - warn:
              when: 'min(memoryCapacity) < {{ .Values.node.recommendedMemoryGi }}Gi'
              message: All nodes are recommended to have at least {{ .Values.node.recommendedMemoryGi }} GiB of memory.
              uri: https://kurl.sh/docs/install-with-kurl/system-requirements
          - pass:
              message: All nodes have at least {{ .Values.node.recommendedMemoryGi }} GiB of memory.
    {{- end }}
    {{- if .Values.nodeChecks.ephemeral.enabled }}
    - docString: |
        Title: Per-node ephemeral storage requirement
        Requirement:
          - Per-node ephemeral storage: Minimum {{ .Values.node.minEphemeralGi }} GiB; Recommended {{ .Values.node.recommendedEphemeralGi }} GiB
        Ephemeral storage backs image layers, writable container filesystems, logs,
          and temporary data. When capacity is low, kubelet enters disk-pressure
          eviction and image pulls fail, causing pod restarts and data loss for
          transient files.
      nodeResources:
        checkName: Per-node ephemeral storage requirement
        outcomes:
          - fail:
              when: 'min(ephemeralStorageCapacity) < {{ .Values.node.minEphemeralGi }}Gi'
              message: All nodes must have at least {{ .Values.node.minEphemeralGi }} GiB of ephemeral storage.
              uri: https://kurl.sh/docs/install-with-kurl/system-requirements
          - warn:
              when: 'min(ephemeralStorageCapacity) < {{ .Values.node.recommendedEphemeralGi }}Gi'
              message: All nodes are recommended to have at least {{ .Values.node.recommendedEphemeralGi }} GiB of ephemeral storage.
              uri: https://kurl.sh/docs/install-with-kurl/system-requirements
          - pass:
              message: All nodes have at least {{ .Values.node.recommendedEphemeralGi }} GiB of ephemeral storage.
    {{- end }}`

const valuesV1Beta3Minimal = `# Minimal values for v1beta3 template
kubernetes:
  enabled: false
storage:
  enabled: false
  className: "default"
runtime:
  enabled: false
distribution:
  enabled: false
ingress:
  enabled: false
nodeChecks:
  count:
    enabled: false
  cpu:
    enabled: false
  memory:
    enabled: false
  ephemeral:
    enabled: false`

const valuesV1Beta3Full = `# Full values for v1beta3 template
kubernetes:
  enabled: true
  minVersion: "1.22.0"
  recommendedVersion: "1.29.0"

storage:
  enabled: true
  className: "default"

cluster:
  minNodes: 3
  recommendedNodes: 5
  minCPU: 4

node:
  minMemoryGi: 8
  recommendedMemoryGi: 32
  minEphemeralGi: 40
  recommendedEphemeralGi: 100

ingress:
  enabled: true
  type: "Contour"

runtime:
  enabled: true

distribution:
  enabled: true

nodeChecks:
  count:
    enabled: true
  cpu:
    enabled: true
  memory:
    enabled: true
  ephemeral:
    enabled: true`

const valuesV1Beta3_1 = `# Values file 1 for testing precedence
kubernetes:
  enabled: true
  minVersion: "1.21.0"
  recommendedVersion: "1.28.0"`

const valuesV1Beta3_3 = `# Values file 3 for testing precedence - should override kubernetes.enabled
kubernetes:
  enabled: false`

// createTempFile creates a temporary file with the given content and returns its path
func createTempFile(t *testing.T, content string, filename string) string {
	t.Helper()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, filename)
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)
	return filePath
}

// repoPath returns a path relative to the repository root from within pkg/preflight tests
func repoPath(rel string) string {
	if rel == "v1beta3.yaml" {
		// Use an existing v1beta3 example file for testing
		return filepath.Join("..", "..", "examples", "preflight", "simple-v1beta3.yaml")
	}
	return filepath.Join("..", "..", rel)
}

func TestDetectAPIVersion_V1Beta3(t *testing.T) {
	t.Parallel()
	api := detectAPIVersion(v1beta3Template)
	assert.Equal(t, "troubleshoot.sh/v1beta3", api)
}

func TestRender_V1Beta3_MinimalValues_YieldsNoAnalyzers(t *testing.T) {
	t.Parallel()

	valuesFile := createTempFile(t, valuesV1Beta3Minimal, "values-v1beta3-minimal.yaml")
	vals, err := loadValuesFile(valuesFile)
	require.NoError(t, err)

	rendered, err := RenderWithHelmTemplate(v1beta3Template, vals)
	require.NoError(t, err)

	kinds, err := loader.LoadSpecs(context.Background(), loader.LoadOptions{RawSpec: rendered, Strict: true})
	require.NoError(t, err)
	require.Len(t, kinds.PreflightsV1Beta2, 1)
	pf := kinds.PreflightsV1Beta2[0]
	assert.Len(t, pf.Spec.Analyzers, 0)
}

func TestRender_V1Beta3_FullValues_ContainsExpectedAnalyzers(t *testing.T) {
	t.Parallel()

	valuesFile := createTempFile(t, valuesV1Beta3Full, "values-v1beta3-full.yaml")
	vals, err := loadValuesFile(valuesFile)
	require.NoError(t, err)

	rendered, err := RenderWithHelmTemplate(v1beta3Template, vals)
	require.NoError(t, err)

	kinds, err := loader.LoadSpecs(context.Background(), loader.LoadOptions{RawSpec: rendered, Strict: true})
	require.NoError(t, err)
	require.Len(t, kinds.PreflightsV1Beta2, 1)
	pf := kinds.PreflightsV1Beta2[0]

	var hasStorageClass, hasCRD, hasRuntime, hasDistribution bool
	nodeResourcesCount := 0
	for _, a := range pf.Spec.Analyzers {
		if a.StorageClass != nil {
			hasStorageClass = true
			assert.Equal(t, "Default StorageClass", a.StorageClass.CheckName)
			assert.Equal(t, "default", a.StorageClass.StorageClassName)
		}
		if a.CustomResourceDefinition != nil {
			hasCRD = true
			assert.Equal(t, "Contour IngressRoute CRD", a.CustomResourceDefinition.CheckName)
			assert.Equal(t, "ingressroutes.contour.heptio.com", a.CustomResourceDefinition.CustomResourceDefinitionName)
		}
		if a.ContainerRuntime != nil {
			hasRuntime = true
		}
		if a.Distribution != nil {
			hasDistribution = true
		}
		if a.NodeResources != nil {
			nodeResourcesCount++
		}
	}

	assert.True(t, hasStorageClass, "expected StorageClass analyzer present")
	assert.True(t, hasCRD, "expected CustomResourceDefinition analyzer present")
	assert.True(t, hasRuntime, "expected ContainerRuntime analyzer present")
	assert.True(t, hasDistribution, "expected Distribution analyzer present")
	assert.Equal(t, 4, nodeResourcesCount, "expected 4 NodeResources analyzers (count, cpu, memory, ephemeral)")
}

func TestRender_V1Beta3_MergeMultipleValuesFiles_And_SetPrecedence(t *testing.T) {
	t.Parallel()

	// Create temporary files for each values set
	minimalFile := createTempFile(t, valuesV1Beta3Minimal, "values-v1beta3-minimal.yaml")
	file1 := createTempFile(t, valuesV1Beta3_1, "values-v1beta3-1.yaml")
	file3 := createTempFile(t, valuesV1Beta3_3, "values-v1beta3-3.yaml")

	// Merge minimal + 1 + 3 => kubernetes.enabled should end up false due to last wins in file 3
	vals := map[string]interface{}{}
	for _, f := range []string{minimalFile, file1, file3} {
		m, err := loadValuesFile(f)
		require.NoError(t, err)
		vals = mergeMaps(vals, m)
	}

	// First render without --set; expect NO kubernetes analyzer
	rendered, err := RenderWithHelmTemplate(v1beta3Template, vals)
	require.NoError(t, err)
	kinds, err := loader.LoadSpecs(context.Background(), loader.LoadOptions{RawSpec: rendered, Strict: true})
	require.NoError(t, err)
	require.Len(t, kinds.PreflightsV1Beta2, 1)
	pf := kinds.PreflightsV1Beta2[0]
	assert.False(t, containsAnalyzer(pf.Spec.Analyzers, "clusterVersion"))

	// Apply --set kubernetes.enabled=true and re-render; expect kubernetes analyzer present
	require.NoError(t, applySetValue(vals, "kubernetes.enabled=true"))
	rendered2, err := RenderWithHelmTemplate(v1beta3Template, vals)
	require.NoError(t, err)
	kinds2, err := loader.LoadSpecs(context.Background(), loader.LoadOptions{RawSpec: rendered2, Strict: true})
	require.NoError(t, err)
	require.Len(t, kinds2.PreflightsV1Beta2, 1)
	pf2 := kinds2.PreflightsV1Beta2[0]
	assert.True(t, containsAnalyzer(pf2.Spec.Analyzers, "clusterVersion"))
}

func containsAnalyzer(analyzers []*troubleshootv1beta2.Analyze, kind string) bool {
	for _, a := range analyzers {
		switch kind {
		case "clusterVersion":
			if a.ClusterVersion != nil {
				return true
			}
		case "storageClass":
			if a.StorageClass != nil {
				return true
			}
		case "customResourceDefinition":
			if a.CustomResourceDefinition != nil {
				return true
			}
		case "containerRuntime":
			if a.ContainerRuntime != nil {
				return true
			}
		case "distribution":
			if a.Distribution != nil {
				return true
			}
		case "nodeResources":
			if a.NodeResources != nil {
				return true
			}
		}
	}
	return false
}

func TestRender_V1Beta3_CLI_ValuesAndSetFlags(t *testing.T) {
	t.Parallel()

	// Start with minimal values (no analyzers enabled)
	valuesFile := createTempFile(t, valuesV1Beta3Minimal, "values-v1beta3-minimal.yaml")
	vals, err := loadValuesFile(valuesFile)
	require.NoError(t, err)

	// Test: render with minimal values - should have no analyzers
	rendered, err := RenderWithHelmTemplate(v1beta3Template, vals)
	require.NoError(t, err)
	kinds, err := loader.LoadSpecs(context.Background(), loader.LoadOptions{RawSpec: rendered, Strict: true})
	require.NoError(t, err)
	require.Len(t, kinds.PreflightsV1Beta2, 1)
	pf := kinds.PreflightsV1Beta2[0]
	assert.Len(t, pf.Spec.Analyzers, 0, "minimal values should produce no analyzers")

	// Test: simulate CLI --set flag to enable kubernetes checks
	err = applySetValue(vals, "kubernetes.enabled=true")
	require.NoError(t, err)
	rendered, err = RenderWithHelmTemplate(v1beta3Template, vals)
	require.NoError(t, err)
	kinds, err = loader.LoadSpecs(context.Background(), loader.LoadOptions{RawSpec: rendered, Strict: true})
	require.NoError(t, err)
	require.Len(t, kinds.PreflightsV1Beta2, 1)
	pf = kinds.PreflightsV1Beta2[0]
	assert.True(t, containsAnalyzer(pf.Spec.Analyzers, "clusterVersion"), "kubernetes analyzer should be present after --set kubernetes.enabled=true")

	// Test: simulate CLI --set flag to override specific values
	err = applySetValue(vals, "kubernetes.minVersion=1.25.0")
	require.NoError(t, err)
	err = applySetValue(vals, "kubernetes.recommendedVersion=1.27.0")
	require.NoError(t, err)
	rendered, err = RenderWithHelmTemplate(v1beta3Template, vals)
	require.NoError(t, err)
	kinds, err = loader.LoadSpecs(context.Background(), loader.LoadOptions{RawSpec: rendered, Strict: true})
	require.NoError(t, err)
	require.Len(t, kinds.PreflightsV1Beta2, 1)
	pf = kinds.PreflightsV1Beta2[0]

	// Verify the overridden values appear in the rendered spec
	var clusterVersionAnalyzer *troubleshootv1beta2.ClusterVersion
	for _, a := range pf.Spec.Analyzers {
		if a.ClusterVersion != nil {
			clusterVersionAnalyzer = a.ClusterVersion
			break
		}
	}
	require.NotNil(t, clusterVersionAnalyzer, "cluster version analyzer should be present")

	// Check that our --set values are used in the rendered outcomes
	foundMinVersion := false
	foundRecommendedVersion := false
	for _, outcome := range clusterVersionAnalyzer.Outcomes {
		if outcome.Fail != nil && strings.Contains(outcome.Fail.When, "1.25.0") {
			foundMinVersion = true
		}
		if outcome.Warn != nil && strings.Contains(outcome.Warn.When, "1.27.0") {
			foundRecommendedVersion = true
		}
	}
	assert.True(t, foundMinVersion, "should find --set minVersion in rendered spec")
	assert.True(t, foundRecommendedVersion, "should find --set recommendedVersion in rendered spec")

	// Test: multiple --set flags to enable multiple analyzer types
	err = applySetValue(vals, "storage.enabled=true")
	require.NoError(t, err)
	err = applySetValue(vals, "runtime.enabled=true")
	require.NoError(t, err)
	rendered, err = RenderWithHelmTemplate(v1beta3Template, vals)
	require.NoError(t, err)
	kinds, err = loader.LoadSpecs(context.Background(), loader.LoadOptions{RawSpec: rendered, Strict: true})
	require.NoError(t, err)
	require.Len(t, kinds.PreflightsV1Beta2, 1)
	pf = kinds.PreflightsV1Beta2[0]

	assert.True(t, containsAnalyzer(pf.Spec.Analyzers, "clusterVersion"), "kubernetes analyzer should remain enabled")
	assert.True(t, containsAnalyzer(pf.Spec.Analyzers, "storageClass"), "storage analyzer should be enabled")
	assert.True(t, containsAnalyzer(pf.Spec.Analyzers, "containerRuntime"), "runtime analyzer should be enabled")
}

func TestRender_V1Beta3_InvalidTemplate_ErrorHandling(t *testing.T) {
	t.Parallel()

	// Test: malformed YAML syntax (actually, this should pass template rendering but fail YAML parsing later)
	invalidYaml := `apiVersion: troubleshoot.sh/v1beta3
kind: Preflight
metadata:
  name: invalid-yaml
spec:
  analyzers:
    - this is not valid yaml
      missing proper structure:
        - and wrong indentation
`
	vals := map[string]interface{}{}
	rendered, err := RenderWithHelmTemplate(invalidYaml, vals)
	require.NoError(t, err, "template rendering should succeed even with malformed YAML")

	// But loading the spec should fail due to invalid YAML structure
	_, err = loader.LoadSpecs(context.Background(), loader.LoadOptions{RawSpec: rendered, Strict: true})
	assert.Error(t, err, "loading malformed YAML should produce an error")

	// Test: invalid Helm template syntax
	invalidTemplate := `apiVersion: troubleshoot.sh/v1beta3
kind: Preflight
metadata:
  name: invalid-template
spec:
  analyzers:
    {{- if .Values.invalid.syntax with unclosed brackets
    - clusterVersion:
        outcomes:
          - pass:
              message: "This should fail"
`
	_, err = RenderWithHelmTemplate(invalidTemplate, vals)
	assert.Error(t, err, "invalid template syntax should produce an error")

	// Test: template referencing undefined values with proper conditional check
	templateWithUndefined := `apiVersion: troubleshoot.sh/v1beta3
kind: Preflight
metadata:
  name: undefined-values
spec:
  analyzers:
    {{- if and .Values.nonexistent (ne .Values.nonexistent.field nil) }}
    - clusterVersion:
        checkName: "Version: {{ .Values.nonexistent.version }}"
        outcomes:
          - pass:
              message: "Should not appear"
    {{- end }}
`
	rendered, err = RenderWithHelmTemplate(templateWithUndefined, vals)
	require.NoError(t, err, "properly guarded undefined values should not cause template error")
	kinds2, err := loader.LoadSpecs(context.Background(), loader.LoadOptions{RawSpec: rendered, Strict: true})
	require.NoError(t, err)
	require.Len(t, kinds2.PreflightsV1Beta2, 1)
	pf2 := kinds2.PreflightsV1Beta2[0]
	assert.Len(t, pf2.Spec.Analyzers, 0, "undefined values should result in no analyzers")

	// Test: template that directly accesses undefined field (should error)
	templateWithDirectUndefined := `apiVersion: troubleshoot.sh/v1beta3
kind: Preflight
metadata:
  name: direct-undefined
spec:
  analyzers:
    - clusterVersion:
        checkName: "{{ .Values.nonexistent.field }}"
        outcomes:
          - pass:
              message: "Should fail"
`
	_, err = RenderWithHelmTemplate(templateWithDirectUndefined, vals)
	assert.Error(t, err, "directly accessing undefined nested values should cause template error")

	// Test: template with missing required value (should error during template rendering)
	templateMissingRequired := `apiVersion: troubleshoot.sh/v1beta3
kind: Preflight
metadata:
  name: missing-required
spec:
  analyzers:
    - storageClass:
        checkName: "Storage Test"
        storageClassName: {{ .Values.storage.className }}
        outcomes:
          - pass:
              message: "Storage is good"
`
	valsWithoutStorage := map[string]interface{}{
		"other": map[string]interface{}{
			"field": "value",
		},
	}
	_, err = RenderWithHelmTemplate(templateMissingRequired, valsWithoutStorage)
	assert.Error(t, err, "template rendering should fail when accessing undefined nested values")

	// Test: circular reference in values (this would be a user config error)
	circularVals := map[string]interface{}{
		"test": map[string]interface{}{
			"field": "{{ .Values.test.field }}", // This would create infinite loop if processed
		},
	}
	templateWithCircular := `apiVersion: troubleshoot.sh/v1beta3
kind: Preflight
metadata:
  name: circular-test
spec:
  analyzers:
    - data:
        name: test.json
        data: |
          {"value": "{{ .Values.test.field }}"}
`
	// Helm template engine should handle this gracefully (it doesn't recursively process string values)
	rendered, err = RenderWithHelmTemplate(templateWithCircular, circularVals)
	require.NoError(t, err, "circular reference in values should not crash template engine")
	assert.Contains(t, rendered, "{{ .Values.test.field }}", "circular reference should render as literal string")
}
