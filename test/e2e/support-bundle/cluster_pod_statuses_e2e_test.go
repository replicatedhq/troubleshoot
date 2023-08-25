package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/replicatedhq/troubleshoot/pkg/convert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestPendingPod(t *testing.T) {
	supportBundleName := "pod-deployment"
	deploymentName := "test-pending-deployment"
	containerName := "curl"
	feature := features.New("Pending Pod Test").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			deployment := newDeployment(c.Namespace(), deploymentName, 1, containerName)
			client, err := c.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			if err = client.Resources().Create(ctx, deployment); err != nil {
				t.Fatal(err)
			}

			return ctx
		}).
		Assess("check support bundle catch pending pod", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			var out bytes.Buffer
			var results []*convert.Result

			tarPath := fmt.Sprintf("%s.tar.gz", supportBundleName)
			targetFile := fmt.Sprintf("%s/analysis.json", supportBundleName)

			cmd := exec.Command("../../../bin/support-bundle", "spec/pod.yaml", "--interactive=false", fmt.Sprintf("-o=%s", supportBundleName))
			cmd.Stdout = &out
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}

			analysisJSON, err := readFileFromTar(tarPath, targetFile)
			if err != nil {
				t.Fatal(err)
			}

			err = json.Unmarshal(analysisJSON, &results)
			if err != nil {
				t.Fatal(err)
			}

			for _, result := range results {
				if strings.Contains(result.Insight.Detail, deploymentName) {
					return ctx
				}
			}

			t.Fatal("Pending pod not found")
			defer func() {
				err := os.Remove(fmt.Sprintf("%s.tar.gz", supportBundleName))
				if err != nil {
					t.Fatal("Error remove file:", err)
				}
			}()
			return ctx
		}).Feature()
	testenv.Test(t, feature)
}

func newDeployment(namespace string, name string, replicas int32, containerName string) *appsv1.Deployment {
	labels := map[string]string{"app": "pending-test"}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: containerName, Image: "nginx", Command: []string{"wge", "-O", "/work-dir/index.html", "https://www.wikipedia.org"}}}},
			},
		},
	}
}
