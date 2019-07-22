package preflight

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type PreflightServerOptions struct {
	ImageName  string
	PullPolicy string

	Name      string
	Namespace string

	OwnerReference metav1.Object
}

func CreatePreflightServer(client client.Client, scheme *runtime.Scheme, options PreflightServerOptions) (*corev1.Pod, *corev1.Service, error) {
	name := fmt.Sprintf("%s-%s", options.Name, "preflight")
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: options.Namespace,
	}

	found := &corev1.Pod{}
	err := client.Get(context.Background(), namespacedName, found)
	if err == nil || !kuberneteserrors.IsNotFound(err) {
		return nil, nil, err
	}

	imageName := "replicated/troubleshoot:latest"
	imagePullPolicy := corev1.PullAlways

	if options.ImageName != "" {
		imageName = options.ImageName
	}
	if options.PullPolicy != "" {
		imagePullPolicy = corev1.PullPolicy(options.PullPolicy)
	}

	podLabels := make(map[string]string)
	podLabels["preflight"] = options.Name
	podLabels["troubleshoot-role"] = "preflight"

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: options.Namespace,
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

	if scheme != nil {
		if err := controllerutil.SetControllerReference(options.OwnerReference, &pod, scheme); err != nil {
			return nil, nil, err
		}
	}

	if err := client.Create(context.Background(), &pod); err != nil {
		return nil, nil, err
	}

	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: options.Namespace,
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

	if scheme != nil {
		if err := controllerutil.SetControllerReference(options.OwnerReference, &service, scheme); err != nil {
			return nil, nil, err
		}
	}

	if err := client.Create(context.Background(), &service); err != nil {
		return nil, nil, err
	}

	// wait for the server to be ready
	// TODO
	time.Sleep(time.Second * 5)

	return &pod, &service, nil
}
