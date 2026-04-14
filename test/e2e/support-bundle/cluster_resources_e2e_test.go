package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"golang.org/x/exp/slices"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterResources(t *testing.T) {
	const (
		mutatingWebhookName   = "e2e-mutating-webhook-config"
		validatingWebhookName = "e2e-validating-webhook-config"
	)

	tests := []struct {
		paths      []string
		expectType string
	}{
		{
			paths: []string{
				"clusterroles.json",
				"volumeattachments.json",
				"nodes.json",
				"pvs.json",
				"resources.json",
				"custom-resource-definitions.json",
				"groups.json",
				"priorityclasses.json",
				"namespaces.json",
				"clusterrolebindings.json",
				"storage-classes.json",
				"mutating-webhook-configurations.json",
				"validating-webhook-configurations.json",
			},
			expectType: "file",
		},
		{
			paths: []string{
				"cronjobs",
				"limitranges",
				"daemonsets",
				"deployments",
				"pvcs",
				"leases",
				"auth-cani-list",
				"services",
				"roles",
				"events",
				"rolebindings",
				"replicasets",
				"jobs",
				"serviceaccounts",
				"configmaps",
				"statefulsets",
				"endpoints",
				"network-policy",
				"resource-quota",
				"ingress",
				"pods",
				"pod-disruption-budgets",
			},
			expectType: "folder",
		},
	}

	feature := features.New("Cluster Resources Test").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			client := clusterClientset(t, ctx)

			_, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(ctx, newMutatingWebhookConfiguration(mutatingWebhookName), metav1.CreateOptions{})
			require.NoError(t, err)

			_, err = client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(ctx, newValidatingWebhookConfiguration(validatingWebhookName), metav1.CreateOptions{})
			require.NoError(t, err)

			return ctx
		}).
		Assess("check support bundle catch cluster resources", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			var out bytes.Buffer
			supportBundleName := "cluster-resources"
			tarPath := fmt.Sprintf("%s.tar.gz", supportBundleName)
			targetFolder := fmt.Sprintf("%s/cluster-resources/", supportBundleName)
			cmd := exec.CommandContext(ctx, sbBinary(), "spec/clusterResources.yaml", "--interactive=false", fmt.Sprintf("-o=%s", supportBundleName))
			cmd.Stdout = &out
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}

			defer func() {
				err := os.Remove(fmt.Sprintf("%s.tar.gz", supportBundleName))
				if err != nil {
					t.Fatal("Error remove file:", err)
				}
			}()

			files, folders, err := readFilesAndFoldersFromTar(tarPath, targetFolder)

			if err != nil {
				t.Fatal(err)
			}

			for _, test := range tests {
				if test.expectType == "file" {
					for _, path := range test.paths {
						if !slices.Contains(files, path) {
							t.Fatalf("Expected file %s not found", path)
						}
					}
				} else if test.expectType == "folder" {
					for _, path := range test.paths {
						if !slices.Contains(folders, path) {
							t.Fatalf("Expected folder %s not found", path)
						}
					}
				}
			}

			assertWebhookConfigurationCollected(t, tarPath, supportBundleName, "mutating-webhook-configurations.json", mutatingWebhookName)
			assertWebhookConfigurationCollected(t, tarPath, supportBundleName, "validating-webhook-configurations.json", validatingWebhookName)

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			client := clusterClientset(t, ctx)

			err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(ctx, mutatingWebhookName, metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				t.Fatal(err)
			}

			err = client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Delete(ctx, validatingWebhookName, metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				t.Fatal(err)
			}

			return ctx
		}).
		Feature()
	testenv.Test(t, feature)
}

func clusterClientset(t *testing.T, ctx context.Context) *kubernetes.Clientset {
	t.Helper()

	cluster := getClusterFromContext(t, ctx, ClusterName)
	restConfig, err := clientcmd.BuildConfigFromFlags("", cluster.GetKubeconfig())
	require.NoError(t, err)

	client, err := kubernetes.NewForConfig(restConfig)
	require.NoError(t, err)

	return client
}

func newMutatingWebhookConfiguration(name string) *admissionregistrationv1.MutatingWebhookConfiguration {
	sideEffects := admissionregistrationv1.SideEffectClassNone
	failurePolicy := admissionregistrationv1.Ignore
	url := "https://example.com/mutate"

	return &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Webhooks: []admissionregistrationv1.MutatingWebhook{
			{
				Name:                    fmt.Sprintf("%s.example.com", name),
				AdmissionReviewVersions: []string{"v1"},
				SideEffects:             &sideEffects,
				FailurePolicy:           &failurePolicy,
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL: &url,
				},
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"configmaps"},
						},
					},
				},
			},
		},
	}
}

func newValidatingWebhookConfiguration(name string) *admissionregistrationv1.ValidatingWebhookConfiguration {
	sideEffects := admissionregistrationv1.SideEffectClassNone
	failurePolicy := admissionregistrationv1.Ignore
	url := "https://example.com/validate"

	return &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name:                    fmt.Sprintf("%s.example.com", name),
				AdmissionReviewVersions: []string{"v1"},
				SideEffects:             &sideEffects,
				FailurePolicy:           &failurePolicy,
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL: &url,
				},
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"secrets"},
						},
					},
				},
			},
		},
	}
}

func assertWebhookConfigurationCollected(t *testing.T, tarPath string, supportBundleName string, fileName string, expectedName string) {
	t.Helper()

	type webhookList struct {
		Items []struct {
			Metadata metav1.ObjectMeta `json:"metadata"`
		} `json:"items"`
	}

	targetFile := fmt.Sprintf("%s/cluster-resources/%s", supportBundleName, fileName)
	payload, err := readFileFromTar(tarPath, targetFile)
	require.NoError(t, err)

	var configs webhookList
	err = json.Unmarshal(payload, &configs)
	require.NoError(t, err)

	found := false
	for _, item := range configs.Items {
		if item.Metadata.Name == expectedName {
			found = true
			break
		}
	}

	assert.True(t, found, "expected webhook configuration %q in %s", expectedName, fileName)
}
