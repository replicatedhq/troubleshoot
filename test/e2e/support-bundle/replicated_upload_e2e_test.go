package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

// makeSDKSecretResource creates a corev1.Secret matching the Replicated SDK
// Helm chart output, with a license embedded as a YAML string in config.yaml.
func makeSDKSecretResource(appName, namespace, licenseID, channelID string) *corev1.Secret {
	configYAML := fmt.Sprintf(
		"license: |\n  apiVersion: kots.io/v1beta1\n  kind: License\n  spec:\n    licenseID: %s\n    appSlug: %s\nchannelID: %q\nreplicatedAppEndpoint: \"https://replicated.app\"\n",
		licenseID, appName, channelID,
	)
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName + "-sdk",
			Namespace: namespace,
			Labels: map[string]string{
				"helm.sh/chart":                "replicated-1.19.2",
				"app.kubernetes.io/name":       appName + "-sdk",
				"app.kubernetes.io/instance":   appName,
				"app.kubernetes.io/managed-by": "Helm",
			},
		},
		Data: map[string][]byte{
			"config.yaml": []byte(configYAML),
		},
	}
}

// specWithAutoUpload is a minimal support bundle spec that collects nothing
// significant — used to test the upload path without slow collectors.
var specWithAutoUpload = `
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: upload-test
spec:
  collectors:
    - clusterResources:
        exclude: true
    - clusterInfo:
        exclude: true
`

func TestReplicatedUpload_SingleApp(t *testing.T) {
	feature := features.New("Auto-upload discovers single SDK secret").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			client, err := c.NewClient()
			require.NoError(t, err)

			secret := makeSDKSecretResource("testapp", c.Namespace(), "license-e2e-001", "chan-e2e")
			err = client.Resources(c.Namespace()).Create(ctx, secret)
			require.NoError(t, err)

			return ctx
		}).
		Assess("discovers SDK secret and attempts upload", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			cluster := getClusterFromContext(t, ctx, ClusterName)
			specFile, err := os.CreateTemp("", "upload-spec-*.yaml")
			require.NoError(t, err)
			defer os.Remove(specFile.Name())
			_, err = specFile.WriteString(specWithAutoUpload)
			require.NoError(t, err)
			specFile.Close()

			cmd := exec.CommandContext(ctx, sbBinary(),
				"--auto-upload",
				"--namespace", c.Namespace(),
				"--interactive=false",
				"--kubeconfig", cluster.GetKubeconfig(),
				specFile.Name(),
			)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			_ = cmd.Run() // May fail on actual upload to replicated.app (no real license), that's OK

			output := stderr.String() + stdout.String()
			// The key assertion: discovery succeeded and presigned URL upload was attempted.
			// The upload itself will fail (fake license), but reaching this message proves
			// the SDK secret was found and credentials were extracted.
			assert.Contains(t, output, "Uploading via Replicated SDK presigned URL")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			client, _ := c.NewClient()
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "testapp-sdk", Namespace: c.Namespace()}}
			_ = client.Resources(c.Namespace()).Delete(ctx, secret)
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

func TestReplicatedUpload_MultipleApps_NonInteractive(t *testing.T) {
	feature := features.New("Auto-upload lists multiple apps in non-interactive mode").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			client, err := c.NewClient()
			require.NoError(t, err)

			// Install two apps in the same namespace
			secret1 := makeSDKSecretResource("app-alpha", c.Namespace(), "license-alpha", "chan-alpha")
			err = client.Resources(c.Namespace()).Create(ctx, secret1)
			require.NoError(t, err)

			secret2 := makeSDKSecretResource("app-beta", c.Namespace(), "license-beta", "chan-beta")
			err = client.Resources(c.Namespace()).Create(ctx, secret2)
			require.NoError(t, err)

			return ctx
		}).
		Assess("lists both apps and suggests --app-slug", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			cluster := getClusterFromContext(t, ctx, ClusterName)
			specFile, err := os.CreateTemp("", "upload-spec-*.yaml")
			require.NoError(t, err)
			defer os.Remove(specFile.Name())
			_, err = specFile.WriteString(specWithAutoUpload)
			require.NoError(t, err)
			specFile.Close()

			cmd := exec.CommandContext(ctx, sbBinary(),
				"--auto-upload",
				"--namespace", c.Namespace(),
				"--interactive=false",
				"--kubeconfig", cluster.GetKubeconfig(),
				specFile.Name(),
			)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			_ = cmd.Run()

			output := stderr.String() + stdout.String()
			// Should list both apps
			assert.Contains(t, output, "app-alpha")
			assert.Contains(t, output, "app-beta")
			// Should suggest --app-slug
			assert.Contains(t, output, "--app-slug")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			client, _ := c.NewClient()
			for _, name := range []string{"app-alpha-sdk", "app-beta-sdk"} {
				secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: c.Namespace()}}
				_ = client.Resources(c.Namespace()).Delete(ctx, secret)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

func TestReplicatedUpload_MultipleApps_WithAppSlug(t *testing.T) {
	feature := features.New("Auto-upload selects correct app via --app-slug").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			client, err := c.NewClient()
			require.NoError(t, err)

			secret1 := makeSDKSecretResource("app-alpha", c.Namespace(), "license-alpha", "chan-alpha")
			err = client.Resources(c.Namespace()).Create(ctx, secret1)
			require.NoError(t, err)

			secret2 := makeSDKSecretResource("app-beta", c.Namespace(), "license-beta", "chan-beta")
			err = client.Resources(c.Namespace()).Create(ctx, secret2)
			require.NoError(t, err)

			return ctx
		}).
		Assess("uses --app-slug to select the right app", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			cluster := getClusterFromContext(t, ctx, ClusterName)
			specFile, err := os.CreateTemp("", "upload-spec-*.yaml")
			require.NoError(t, err)
			defer os.Remove(specFile.Name())
			_, err = specFile.WriteString(specWithAutoUpload)
			require.NoError(t, err)
			specFile.Close()

			cmd := exec.CommandContext(ctx, sbBinary(),
				"--auto-upload",
				"--app-slug", "app-beta",
				"--namespace", c.Namespace(),
				"--interactive=false",
				"--kubeconfig", cluster.GetKubeconfig(),
				specFile.Name(),
			)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			_ = cmd.Run()

			output := stderr.String() + stdout.String()
			// Discovery succeeded and selected app-beta via --app-slug
			assert.Contains(t, output, "Uploading via Replicated SDK presigned URL")
			// Should NOT prompt for selection
			assert.NotContains(t, output, "multiple SDK secrets found")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			client, _ := c.NewClient()
			for _, name := range []string{"app-alpha-sdk", "app-beta-sdk"} {
				secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: c.Namespace()}}
				_ = client.Resources(c.Namespace()).Delete(ctx, secret)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

func TestReplicatedUpload_MultipleApps_AcrossNamespaces_NonInteractive(t *testing.T) {
	ns1 := envconf.RandomName("upload-ns1", 16)
	ns2 := envconf.RandomName("upload-ns2", 16)

	feature := features.New("Auto-upload lists apps across namespaces in non-interactive mode").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			client, err := c.NewClient()
			require.NoError(t, err)

			// Create two namespaces
			for _, ns := range []string{ns1, ns2} {
				nsObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
				err = client.Resources().Create(ctx, nsObj)
				require.NoError(t, err)
			}

			// App 1 in ns1
			secret1 := makeSDKSecretResource("app-gamma", ns1, "license-gamma", "chan-gamma")
			err = client.Resources(ns1).Create(ctx, secret1)
			require.NoError(t, err)

			// App 2 in ns2
			secret2 := makeSDKSecretResource("app-delta", ns2, "license-delta", "chan-delta")
			err = client.Resources(ns2).Create(ctx, secret2)
			require.NoError(t, err)

			return ctx
		}).
		Assess("lists both apps from different namespaces", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			cluster := getClusterFromContext(t, ctx, ClusterName)
			specFile, err := os.CreateTemp("", "upload-spec-*.yaml")
			require.NoError(t, err)
			defer os.Remove(specFile.Name())
			_, err = specFile.WriteString(specWithAutoUpload)
			require.NoError(t, err)
			specFile.Close()

			// Use a namespace that has no SDK secret to trigger cross-namespace search
			cmd := exec.CommandContext(ctx, sbBinary(),
				"--auto-upload",
				"--namespace", c.Namespace(),
				"--interactive=false",
				"--kubeconfig", cluster.GetKubeconfig(),
				specFile.Name(),
			)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			_ = cmd.Run()

			output := stderr.String() + stdout.String()
			// Should find both apps across namespaces
			assert.Contains(t, output, "app-gamma")
			assert.Contains(t, output, "app-delta")
			// Should suggest --app-slug for selection
			assert.Contains(t, output, "--app-slug")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			client, _ := c.NewClient()
			for _, ns := range []string{ns1, ns2} {
				nsObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
				_ = client.Resources().Delete(ctx, nsObj)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

func TestReplicatedUpload_MultipleApps_AcrossNamespaces_WithAppSlug(t *testing.T) {
	ns1 := envconf.RandomName("upload-ns1", 16)
	ns2 := envconf.RandomName("upload-ns2", 16)

	feature := features.New("Auto-upload selects correct app across namespaces via --app-slug").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			client, err := c.NewClient()
			require.NoError(t, err)

			for _, ns := range []string{ns1, ns2} {
				nsObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
				err = client.Resources().Create(ctx, nsObj)
				require.NoError(t, err)
			}

			secret1 := makeSDKSecretResource("app-gamma", ns1, "license-gamma", "chan-gamma")
			err = client.Resources(ns1).Create(ctx, secret1)
			require.NoError(t, err)

			secret2 := makeSDKSecretResource("app-delta", ns2, "license-delta", "chan-delta")
			err = client.Resources(ns2).Create(ctx, secret2)
			require.NoError(t, err)

			return ctx
		}).
		Assess("selects app-delta via --app-slug without prompting", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			cluster := getClusterFromContext(t, ctx, ClusterName)
			specFile, err := os.CreateTemp("", "upload-spec-*.yaml")
			require.NoError(t, err)
			defer os.Remove(specFile.Name())
			_, err = specFile.WriteString(specWithAutoUpload)
			require.NoError(t, err)
			specFile.Close()

			cmd := exec.CommandContext(ctx, sbBinary(),
				"--auto-upload",
				"--app-slug", "app-delta",
				"--namespace", c.Namespace(),
				"--interactive=false",
				"--kubeconfig", cluster.GetKubeconfig(),
				specFile.Name(),
			)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			_ = cmd.Run()

			output := stderr.String() + stdout.String()
			// Discovery succeeded and selected app-delta via --app-slug
			assert.Contains(t, output, "Uploading via Replicated SDK presigned URL")
			// Should NOT list multiple or prompt
			assert.NotContains(t, output, "multiple SDK secrets found")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			client, _ := c.NewClient()
			for _, ns := range []string{ns1, ns2} {
				nsObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
				_ = client.Resources().Delete(ctx, nsObj)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

func TestReplicatedUpload_MultipleApps_AcrossNamespaces_WithSdkNamespace(t *testing.T) {
	ns1 := envconf.RandomName("upload-ns1", 16)
	ns2 := envconf.RandomName("upload-ns2", 16)

	feature := features.New("Auto-upload selects correct namespace via --sdk-namespace").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			client, err := c.NewClient()
			require.NoError(t, err)

			for _, ns := range []string{ns1, ns2} {
				nsObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
				err = client.Resources().Create(ctx, nsObj)
				require.NoError(t, err)
			}

			// Only one SDK secret per namespace, but --sdk-namespace skips cross-ns search
			secret1 := makeSDKSecretResource("app-gamma", ns1, "license-gamma", "chan-gamma")
			err = client.Resources(ns1).Create(ctx, secret1)
			require.NoError(t, err)

			secret2 := makeSDKSecretResource("app-delta", ns2, "license-delta", "chan-delta")
			err = client.Resources(ns2).Create(ctx, secret2)
			require.NoError(t, err)

			return ctx
		}).
		Assess("uses --sdk-namespace to target specific namespace", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			cluster := getClusterFromContext(t, ctx, ClusterName)
			specFile, err := os.CreateTemp("", "upload-spec-*.yaml")
			require.NoError(t, err)
			defer os.Remove(specFile.Name())
			_, err = specFile.WriteString(specWithAutoUpload)
			require.NoError(t, err)
			specFile.Close()

			cmd := exec.CommandContext(ctx, sbBinary(),
				"--auto-upload",
				"--sdk-namespace", ns1,
				"--interactive=false",
				"--kubeconfig", cluster.GetKubeconfig(),
				specFile.Name(),
			)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			_ = cmd.Run()

			output := stderr.String() + stdout.String()
			// Should find the SDK secret directly and attempt upload (no cross-ns search)
			assert.Contains(t, output, "Uploading via Replicated SDK presigned URL")
			assert.NotContains(t, output, "searching all namespaces")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			client, _ := c.NewClient()
			for _, ns := range []string{ns1, ns2} {
				nsObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
				_ = client.Resources().Delete(ctx, nsObj)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

func TestReplicatedUpload_NoSDKSecret(t *testing.T) {
	feature := features.New("Auto-upload shows helpful hints when no SDK found").
		Assess("shows --app-slug and --sdk-namespace hints", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			cluster := getClusterFromContext(t, ctx, ClusterName)
			specFile, err := os.CreateTemp("", "upload-spec-*.yaml")
			require.NoError(t, err)
			defer os.Remove(specFile.Name())
			_, err = specFile.WriteString(specWithAutoUpload)
			require.NoError(t, err)
			specFile.Close()

			cmd := exec.CommandContext(ctx, sbBinary(),
				"--auto-upload",
				"--namespace", c.Namespace(),
				"--interactive=false",
				"--kubeconfig", cluster.GetKubeconfig(),
				specFile.Name(),
			)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			_ = cmd.Run()

			output := stderr.String() + stdout.String()
			// Should suggest flags
			assert.True(t,
				strings.Contains(output, "--app-slug") || strings.Contains(output, "--sdk-namespace"),
				"should suggest --app-slug or --sdk-namespace in output: %s", output,
			)
			// Bundle should still be saved locally
			assert.Contains(t, output, "support-bundle-")

			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}
