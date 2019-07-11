package preflightjob

import (
	"context"
	"fmt"
	"time"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ReconcilePreflightJob) createPreflightServer(instance *troubleshootv1beta1.PreflightJob) error {
	name := fmt.Sprintf("%s-%s", instance.Name, "preflight")
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: instance.Namespace,
	}

	found := &corev1.Pod{}
	err := r.Get(context.Background(), namespacedName, found)
	if err == nil || !kuberneteserrors.IsNotFound(err) {
		return err
	}

	imageName := "replicatedhq/troubleshoot:latest"
	imagePullPolicy := corev1.PullAlways

	if instance.Spec.Image != "" {
		imageName = instance.Spec.Image
	}
	if instance.Spec.ImagePullPolicy != "" {
		imagePullPolicy = corev1.PullPolicy(instance.Spec.ImagePullPolicy)
	}

	podLabels := make(map[string]string)
	podLabels["preflight"] = instance.Name
	podLabels["troubleshoot-role"] = "preflight"

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
			Labels:    podLabels,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Image:           imageName,
					ImagePullPolicy: imagePullPolicy,
					Name:            "preflight",
					Command:         []string{"preflight"},
					Args:            []string{"server"},
					Ports: []corev1.ContainerPort{
						{
							Name:          "http",
							ContainerPort: 8000,
						},
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(instance, &pod, r.scheme); err != nil {
		return err
	}

	if err := r.Create(context.Background(), &pod); err != nil {
		return err
	}

	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		Spec: corev1.ServiceSpec{
			Selector: podLabels,
			Type:     corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       8000,
					TargetPort: intstr.FromInt(8000),
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(instance, &service, r.scheme); err != nil {
		return err
	}

	if err := r.Create(context.Background(), &service); err != nil {
		return err
	}

	instance.Status.ServerPodName = name
	instance.Status.ServerPodNamespace = instance.Namespace
	instance.Status.ServerPodPort = 8000
	instance.Status.IsServerReady = true

	// wait for the server to be ready
	// TODO
	time.Sleep(time.Second * 5)

	if err := r.Update(context.Background(), instance); err != nil {
		return err
	}

	return nil
}
