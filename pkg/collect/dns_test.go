package collect

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetKubernetesClusterIP(t *testing.T) {
	k8sSvcIp := "10.0.0.1"
	client := fake.NewSimpleClientset()
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: k8sSvcIp,
		},
	}

	// Add the service to the fake clientset
	_, err := client.CoreV1().Services("default").Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("error injecting service into fake clientset: %v", err)
	}

	// Call the function
	clusterIP, err := getKubernetesClusterIP(client, context.TODO())
	if err != nil {
		t.Fatalf("error getting cluster IP: %v", err)
	}

	// Check the result
	if clusterIP != k8sSvcIp {
		t.Errorf("expected %s, got %s", k8sSvcIp, clusterIP)
	}
}
