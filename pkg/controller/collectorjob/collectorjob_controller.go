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
	"context"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	troubleshootclientv1beta1 "github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/typed/troubleshoot/v1beta1"
	// corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
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

	// err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
	// 	IsController: true,
	// 	OwnerType:    &troubleshootv1beta1.CollectorJob{},
	// })
	// if err != nil {
	// 	return err
	// }

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
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
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
			return reconcile.Result{}, nil
		}
	}

	return reconcile.Result{}, nil
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
	if contains(instance.Status.Successful, idForCollector(collect)) {
		return nil
	}
	if contains(instance.Status.Failed, idForCollector(collect)) {
		return nil
	}

	// if it's running already...
	if contains(instance.Status.Running, idForCollector(collect)) {
		return nil
	}

	return nil
}

func idForCollector(collector *troubleshootv1beta1.Collect) string {
	if collector.ClusterInfo != nil {
		return "cluster-info"
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
