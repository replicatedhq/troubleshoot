package e2e

import (
	"bytes"
	"context"
	"os/exec"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestCrashPod(t *testing.T) {
	deploymentName := "test-crash-deployment"
	containerName := "curl"
	feature := features.New("Crashloop Pod Test").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			deployment := newDeployment(c.Namespace(), deploymentName, 1, containerName)
			client, err := c.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			if err = client.Resources().Create(ctx, deployment); err != nil {
				t.Fatal(err)
			}
			// err = wait.For(conditions.New(client.Resources()).DeploymentConditionMatch(deployment, appsv1.DeploymentAvailable, corev1.ConditionTrue), wait.WithTimeout(time.Minute*5))
			// if err != nil {
			// 	t.Fatal(err)
			// }

			return ctx
		}).
		Assess("check support bundle catch crashloop pod", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			var out bytes.Buffer
			cmd := exec.Command("../../../bin/support-bundle", "spec/crashloopPod.yaml", "--interactive=false")
			cmd.Stdout = &out
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).Feature()
	testenv.Test(t, feature)
}

func newDeployment(namespace string, name string, replicas int32, containerName string) *appsv1.Deployment {
	labels := map[string]string{"app": "crash-loop-test"}
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
