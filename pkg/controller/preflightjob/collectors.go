package preflightjob

import (
	"context"
	"fmt"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ReconcilePreflightJob) reconcilePreflightCollectors(instance *troubleshootv1beta1.PreflightJob, preflight *troubleshootv1beta1.Preflight) error {
	requestedCollectorIDs := make([]string, 0, 0)
	for _, collector := range preflight.Spec.Collectors {
		requestedCollectorIDs = append(requestedCollectorIDs, idForCollector(collector))
		if err := r.reconcileOnePreflightCollector(instance, collector); err != nil {
			return err
		}
	}

	if !contains(requestedCollectorIDs, "cluster-info") {
		clusterInfo := troubleshootv1beta1.Collect{
			ClusterInfo: &troubleshootv1beta1.ClusterInfo{},
		}
		if err := r.reconcileOnePreflightCollector(instance, &clusterInfo); err != nil {
			return err
		}
	}
	if !contains(requestedCollectorIDs, "cluster-resources") {
		clusterResources := troubleshootv1beta1.Collect{
			ClusterResources: &troubleshootv1beta1.ClusterResources{},
		}
		if err := r.reconcileOnePreflightCollector(instance, &clusterResources); err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcilePreflightJob) reconcileOnePreflightCollector(instance *troubleshootv1beta1.PreflightJob, collect *troubleshootv1beta1.Collect) error {
	if contains(instance.Status.CollectorsRunning, idForCollector(collect)) {
		// preflight just leaves these stopped containers.
		// it's playing with fire a little, but the analyzers can just
		// read from the stdout of the stopped container
		//
		// in the very common use case (what we are building for today)
		// there's not too much risk in something destroying and reaping that stopped pod
		// immediately.  this is a longer term problem to solve, maybe something,
		// the mananger? can broker these collector results.  but, ya know...

		instance.Status.CollectorsSuccessful = append(instance.Status.CollectorsSuccessful, idForCollector(collect))
		instance.Status.CollectorsRunning = remove(instance.Status.CollectorsRunning, idForCollector(collect))

		if len(instance.Status.CollectorsRunning) == 0 {
			instance.Status.IsCollectorsComplete = true
		}

		if err := r.Update(context.Background(), instance); err != nil {
			return err
		}

		return nil
	}

	if err := r.createCollectorSpecInConfigMap(instance, collect); err != nil {
		return err
	}
	if err := r.createCollectorPod(instance, collect); err != nil {
		return err
	}

	return nil
}

func (r *ReconcilePreflightJob) createCollectorSpecInConfigMap(instance *troubleshootv1beta1.PreflightJob, collector *troubleshootv1beta1.Collect) error {
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

func (r *ReconcilePreflightJob) createCollectorPod(instance *troubleshootv1beta1.PreflightJob, collector *troubleshootv1beta1.Collect) error {
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

	instance.Status.CollectorsRunning = append(instance.Status.CollectorsRunning, idForCollector(collector))
	if err := r.Update(context.Background(), instance); err != nil {
		return err
	}

	return nil
}

// Todo these will overlap with troubleshoot containers running at the same time
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
