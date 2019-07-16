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

package preflightjob

import (
	"context"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	troubleshootclientv1beta1 "github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/typed/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/preflight"
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

// Add creates a new PreflightJob Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePreflightJob{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("preflightjob-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to PreflightJob
	err = c.Watch(&source.Kind{Type: &troubleshootv1beta1.PreflightJob{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcilePreflightJob{}

// ReconcilePreflightJob reconciles a PreflightJob object
type ReconcilePreflightJob struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a PreflightJob object and makes changes based on the state read
// and what is in the PreflightJob.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=troubleshoot.replicated.com,resources=preflightjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=troubleshoot.replicated.com,resources=preflightjobs/status,verbs=get;update;patch
func (r *ReconcilePreflightJob) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the PreflightJob instance
	instance := &troubleshootv1beta1.PreflightJob{}
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

	if !instance.Status.IsServerReady {
		preflightServerOptions := preflight.PreflightServerOptions{
			ImageName:      instance.Spec.Image,
			PullPolicy:     instance.Spec.ImagePullPolicy,
			Name:           instance.Name,
			Namespace:      instance.Namespace,
			OwnerReference: instance,
		}
		pod, _, err := preflight.CreatePreflightServer(r.Client, r.scheme, preflightServerOptions)
		if err != nil {
			return reconcile.Result{}, err
		}

		instance.Status.ServerPodName = pod.Name
		instance.Status.ServerPodNamespace = pod.Namespace
		instance.Status.ServerPodPort = 8000
		instance.Status.IsServerReady = true

		if err := r.Update(context.Background(), instance); err != nil {
			return reconcile.Result{}, err
		}

	}

	namespace := instance.Namespace
	if instance.Spec.Preflight.Namespace != "" {
		namespace = instance.Spec.Preflight.Namespace
	}

	preflightSpec, err := r.getPreflightSpec(namespace, instance.Spec.Preflight.Name)
	if err != nil {
		return reconcile.Result{}, err
	}

	if len(instance.Status.AnalyzersRunning) == 0 && len(instance.Status.AnalyzersSuccessful) == 0 && len(instance.Status.AnalyzersFailed) == 0 {
		// Add them all!
		analyzersRunning := []string{}
		for _, analyzer := range preflightSpec.Spec.Analyzers {
			analyzersRunning = append(analyzersRunning, idForAnalyzer(analyzer))
		}

		instance.Status.AnalyzersRunning = analyzersRunning
		if err := r.Update(context.Background(), instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	if err := r.reconcilePreflightCollectors(instance, preflightSpec); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.reconcilePreflightAnalyzers(instance, preflightSpec); err != nil {
		return reconcile.Result{}, err
	}

	// just finished, nothing to do
	return reconcile.Result{}, nil
}

func (r *ReconcilePreflightJob) getPreflightSpec(namespace string, name string) (*troubleshootv1beta1.Preflight, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	troubleshootClient, err := troubleshootclientv1beta1.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	preflight, err := troubleshootClient.Preflights(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return preflight, nil
}
