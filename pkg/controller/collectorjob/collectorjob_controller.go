/*
Copyright 2019 Replicated, Inc..

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package collectorjob

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	troubleshootclientv1beta1 "github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/typed/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller")

// Add creates a new CollectorJob Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileCollectorJob{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("collectorjob-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to CollectorJob
	err = c.Watch(&source.Kind{Type: &troubleshootv1beta1.CollectorJob{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &troubleshootv1beta1.CollectorJob{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileCollectorJob{}

// ReconcileCollectorJob reconciles a CollectorJob object
type ReconcileCollectorJob struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a CollectorJob object and makes changes based on the state read
// and what is in the CollectorJob.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=troubleshoot.replicated.com,resources=collectorjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=troubleshoot.replicated.com,resources=collectorjobs/status,verbs=get;update;patch
func (r *ReconcileCollectorJob) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the CollectorJob instance
	instance := &troubleshootv1beta1.CollectorJob{}
	err := r.Get(context.Background(), request.NamespacedName, instance)
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// for a new object, create the http server
	if !instance.Status.IsServerReady {
		if err := r.createCollectorServer(instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	namespace := instance.Namespace
	if instance.Spec.Collector.Namespace != "" {
		namespace = instance.Spec.Collector.Namespace
	}

	collectorSpec, err := r.getCollectorSpec(namespace, instance.Spec.Collector.Name)
	if err != nil {
		return reconcile.Result{}, err
	}

	for _, collector := range collectorSpec.Spec {
		if err := r.reconileOneCollectorJob(instance, collector); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileCollectorJob) createCollectorServer(instance *troubleshootv1beta1.CollectorJob) error {
	name := fmt.Sprintf("%s-%s", instance.Name, "collector")

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
	podLabels["collector"] = instance.Name
	podLabels["troubleshoot-role"] = "collector"

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
					Name:            "collector",
					Command:         []string{"collector"},
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

func (r *ReconcileCollectorJob) getCollectorSpec(namespace string, name string) (*troubleshootv1beta1.Collector, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	troubleshootClient, err := troubleshootclientv1beta1.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	collector, err := troubleshootClient.Collectors(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return collector, nil
}

func (r *ReconcileCollectorJob) reconileOneCollectorJob(instance *troubleshootv1beta1.CollectorJob, collect *troubleshootv1beta1.Collect) error {
	if contains(instance.Status.Running, idForCollector(collect)) {
		collectorPod, err := r.getCollectorPod(instance, collect)
		if err != nil {
			return err
		}

		if collectorPod.Status.Phase == corev1.PodFailed {
			instance.Status.Failed = append(instance.Status.Failed, idForCollector(collect))
			instance.Status.Running = remove(instance.Status.Running, idForCollector(collect))

			if err := r.Update(context.Background(), instance); err != nil {
				return err
			}

			return nil
		}
		if collectorPod.Status.Phase == corev1.PodSucceeded {
			// Get the logs
			podLogOpts := corev1.PodLogOptions{}

			cfg, err := config.GetConfig()
			if err != nil {
				return err
			}

			k8sClient, err := kubernetes.NewForConfig(cfg)
			if err != nil {
				return err
			}
			req := k8sClient.CoreV1().Pods(collectorPod.Namespace).GetLogs(collectorPod.Name, &podLogOpts)
			podLogs, err := req.Stream()
			if err != nil {
				return err
			}
			defer podLogs.Close()
			buf := new(bytes.Buffer)
			_, err = io.Copy(buf, podLogs)
			if err != nil {
				return err
			}

			client := &http.Client{}

			serviceURI := ""

			// For local dev, it's useful to run the manager out of the cluster
			// but this is difficult to connect to the collector service running in-cluster
			// so, we can create a local port-foward to get back into the cluster.
			stopCh := make(chan struct{}, 1)
			if os.Getenv("TROUBLESHOOT_EXTERNAL_MANAGER") != "" {
				fmt.Printf("setting up port forwarding because the manager is not running in the cluster\n")

				// this isn't likely to be very solid
				r := rand.New(rand.NewSource(time.Now().UnixNano()))
				localPort := 3000 + r.Intn(999)

				homeDir := os.Getenv("HOME")
				if homeDir == "" {
					homeDir = os.Getenv("USERPROFILE")
				}
				kubeContext := filepath.Join(homeDir, ".kube", "config")
				ch, err := k8sutil.PortForward(kubeContext, localPort, 8000, instance.Namespace, instance.Name+"-collector")
				if err != nil {
					return err
				}

				stopCh = ch
				serviceURI = fmt.Sprintf("http://localhost:%d", localPort)
			} else {
				serviceURI = fmt.Sprintf("http://%s-collector.%s.svc.cluster.local:8000", instance.Name, instance.Namespace)
			}

			request, err := http.NewRequest("PUT", serviceURI, buf)
			if err != nil {
				return err
			}
			request.ContentLength = int64(len(buf.String()))
			request.Header.Add("collector-id", idForCollector(collect))
			resp, err := client.Do(request)
			if err != nil {
				return err
			}

			if resp.StatusCode != 201 {
				return errors.New("failed to send logs to collector")
			}

			if os.Getenv("TROUBLESHOOT_EXTERNAL_MANAGER") != "" {
				fmt.Printf("stopping port forwarding\n")
				close(stopCh)
			}

			instance.Status.Successful = append(instance.Status.Successful, idForCollector(collect))
			instance.Status.Running = remove(instance.Status.Running, idForCollector(collect))

			if err := r.Update(context.Background(), instance); err != nil {
				return err
			}

			return nil
		}
		return nil
	}

	if err := r.createSpecInConfigMap(instance, collect); err != nil {
		return err
	}
	if err := r.createCollectorPod(instance, collect); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileCollectorJob) createSpecInConfigMap(instance *troubleshootv1beta1.CollectorJob, collector *troubleshootv1beta1.Collect) error {
	name := fmt.Sprintf("%s-%s", instance.Name, idForCollector(collector))

	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: instance.Namespace,
	}

	found := &corev1.ConfigMap{}
	err := r.Get(context.Background(), namespacedName, found)
	if err == nil || !kuberneteserrors.IsNotFound(err) {
		return err
	}

	specContents, err := yaml.Marshal(collector)
	if err != nil {
		return err
	}

	specData := make(map[string]string)
	specData["collector.yaml"] = string(specContents)

	configMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		Data: specData,
	}

	if err := controllerutil.SetControllerReference(instance, &configMap, r.scheme); err != nil {
		return err
	}

	if err := r.Create(context.Background(), &configMap); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileCollectorJob) getCollectorPod(instance *troubleshootv1beta1.CollectorJob, collector *troubleshootv1beta1.Collect) (*corev1.Pod, error) {
	name := fmt.Sprintf("%s-%s", instance.Name, idForCollector(collector))

	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: instance.Namespace,
	}

	pod := &corev1.Pod{}
	err := r.Get(context.Background(), namespacedName, pod)
	if err != nil {
		return nil, err
	}

	return pod, nil
}

func (r *ReconcileCollectorJob) createCollectorPod(instance *troubleshootv1beta1.CollectorJob, collector *troubleshootv1beta1.Collect) error {
	name := fmt.Sprintf("%s-%s", instance.Name, idForCollector(collector))

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

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
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
					Name:            idForCollector(collector),
					Command:         []string{"collector"},
					Args: []string{
						"run",
						"--collector",
						"/troubleshoot/specs/collector.yaml",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "collector",
							MountPath: "/troubleshoot/specs",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "collector",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: name,
							},
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

	instance.Status.Running = append(instance.Status.Running, idForCollector(collector))
	if err := r.Update(context.Background(), instance); err != nil {
		return err
	}

	return nil
}

func idForCollector(collector *troubleshootv1beta1.Collect) string {
	if collector.ClusterInfo != nil {
		return "cluster-info"
	} else if collector.ClusterResources != nil {
		return "cluster-resources"
	}

	return ""
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}
