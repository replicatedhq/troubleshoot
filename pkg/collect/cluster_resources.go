package collect

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path" // this code uses 'path' and not 'path/filepath' because we don't want backslashes on windows
	"path/filepath"
	"sort"
	"strings"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"gopkg.in/yaml.v2"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsv1clientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	apiextensionsv1beta1clientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/replicatedhq/troubleshoot/pkg/k8sutil/discovery"
)

type CollectClusterResources struct {
	Collector    *troubleshootv1beta2.ClusterResources
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	RBACErrors
}

func (c *CollectClusterResources) Title() string {
	return collectorTitleOrDefault(c.Collector.CollectorMeta, "Cluster Resources")
}

func (c *CollectClusterResources) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectClusterResources) Merge(allCollectors []Collector) ([]Collector, error) {
	result := append(allCollectors, c)
	return result, nil
}

func (c *CollectClusterResources) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	output := NewResult()

	// namespaces
	nsListedFromCluster := false
	var namespaceNames []string
	if len(c.Collector.Namespaces) > 0 {
		namespaces, namespaceErrors := getNamespaces(ctx, client, c.Collector.Namespaces)
		namespaceNames = c.Collector.Namespaces
		output.SaveResult(c.BundlePath, "cluster-resources/namespaces.json", bytes.NewBuffer(namespaces))
		output.SaveResult(c.BundlePath, "cluster-resources/namespaces-errors.json", marshalErrors(namespaceErrors))
	} else if c.Namespace != "" {
		namespace, namespaceErrors := getNamespace(ctx, client, c.Namespace)
		output.SaveResult(c.BundlePath, "cluster-resources/namespaces.json", bytes.NewBuffer(namespace))
		output.SaveResult(c.BundlePath, "cluster-resources/namespaces-errors.json", marshalErrors(namespaceErrors))
		namespaceNames = append(namespaceNames, c.Namespace)
	} else {
		namespaces, namespaceList, namespaceErrors := getAllNamespaces(ctx, client)
		output.SaveResult(c.BundlePath, "cluster-resources/namespaces.json", bytes.NewBuffer(namespaces))
		output.SaveResult(c.BundlePath, "cluster-resources/namespaces-errors.json", marshalErrors(namespaceErrors))
		if namespaceList != nil {
			for _, namespace := range namespaceList.Items {
				namespaceNames = append(namespaceNames, namespace.Name)
			}
		}
		nsListedFromCluster = true
	}

	reviewStatuses, reviewStatusErrors := getSelfSubjectRulesReviews(ctx, client, namespaceNames)

	// auth cani
	authCanI := authCanI(reviewStatuses, namespaceNames)
	for k, v := range authCanI {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/auth-cani-list", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/auth-cani-list-errors.json", marshalErrors(reviewStatusErrors))

	if nsListedFromCluster && !c.Collector.IgnoreRBAC {
		filteredNamespaces := []string{}
		for _, ns := range namespaceNames {
			status := reviewStatuses[ns]
			if status == nil || canCollectNamespaceResources(status) { // TODO: exclude nil ones?
				filteredNamespaces = append(filteredNamespaces, ns)
			}
		}
		namespaceNames = filteredNamespaces
	}

	// pods
	pods, podErrors, unhealthyPods := pods(ctx, client, namespaceNames)
	for k, v := range pods {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/pods", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/pods-errors.json", marshalErrors(podErrors))

	for _, pod := range unhealthyPods {
		allContainers := append(pod.Spec.InitContainers, pod.Spec.Containers...)
		for _, container := range allContainers {
			logsRoot := ""
			if c.BundlePath != "" {
				logsRoot = path.Join(c.BundlePath, "cluster-resources", "pods", "logs", pod.Namespace)
			}
			limits := &troubleshootv1beta2.LogLimits{
				MaxLines: 500,
			}
			podLogs, err := savePodLogs(ctx, logsRoot, client, pod, "", container.Name, limits, false)
			if err != nil {
				errPath := filepath.Join("cluster-resources", "pods", "logs", pod.Namespace, pod.Name, fmt.Sprintf("%s-logs-errors.log", container.Name))
				output.SaveResult(c.BundlePath, errPath, bytes.NewBuffer([]byte(err.Error())))
			}
			for k, v := range podLogs {
				output[filepath.Join("cluster-resources", "pods", "logs", pod.Namespace, k)] = v
			}
		}
	}

	// pod disruption budgets (for all existing namespaces)

	PodDisruptionBudgets, pdbError := getPodDisruptionBudgets(ctx, client, namespaceNames)
	for k, v := range PodDisruptionBudgets {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/pod-disruption-budgets", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/pod-disruption-budgets-info.json", marshalErrors(pdbError))

	// services
	services, servicesErrors := services(ctx, client, namespaceNames)
	for k, v := range services {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/services", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/services-errors.json", marshalErrors(servicesErrors))

	// deployments
	deployments, deploymentsErrors := deployments(ctx, client, namespaceNames)
	for k, v := range deployments {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/deployments", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/deployments-errors.json", marshalErrors(deploymentsErrors))

	// statefulsets
	statefulsets, statefulsetsErrors := statefulsets(ctx, client, namespaceNames)
	for k, v := range statefulsets {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/statefulsets", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/statefulsets-errors.json", marshalErrors(statefulsetsErrors))

	// replicasets
	replicasets, replicasetsErrors := replicasets(ctx, client, namespaceNames)
	for k, v := range replicasets {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/replicasets", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/replicasets-errors.json", marshalErrors(replicasetsErrors))

	// jobs
	jobs, jobsErrors := jobs(ctx, client, namespaceNames)
	for k, v := range jobs {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/jobs", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/jobs-errors.json", marshalErrors(jobsErrors))

	// cronJobs
	cronJobs, cronJobsErrors := cronJobs(ctx, client, namespaceNames)
	for k, v := range cronJobs {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/cronjobs", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/cronjobs-errors.json", marshalErrors(cronJobsErrors))

	// ingress
	ingress, ingressErrors := ingress(ctx, client, namespaceNames)
	for k, v := range ingress {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/ingress", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/ingress-errors.json", marshalErrors(ingressErrors))

	// network policy
	networkPolicy, networkPolicyErrors := networkPolicy(ctx, client, namespaceNames)
	for k, v := range networkPolicy {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/network-policy", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/network-policy-errors.json", marshalErrors(networkPolicyErrors))

	// resource quotas
	resourceQuota, resourceQuotaErrors := resourceQuota(ctx, client, namespaceNames)
	for k, v := range resourceQuota {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/resource-quotas", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/resource-quota-errors.json", marshalErrors(resourceQuotaErrors))

	// storage classes
	storageClasses, storageErrors := storageClasses(ctx, client)
	output.SaveResult(c.BundlePath, "cluster-resources/storage-classes.json", bytes.NewBuffer(storageClasses))
	output.SaveResult(c.BundlePath, "cluster-resources/storage-errors.json", marshalErrors(storageErrors))

	// crds
	customResourceDefinitions, crdErrors := crds(ctx, client, c.ClientConfig)
	output.SaveResult(c.BundlePath, "cluster-resources/custom-resource-definitions.json", bytes.NewBuffer(customResourceDefinitions))
	output.SaveResult(c.BundlePath, "cluster-resources/custom-resource-definitions-errors.json", marshalErrors(crdErrors))

	// crs
	customResources, crErrors := crs(ctx, dynamicClient, client, c.ClientConfig, namespaceNames)
	for k, v := range customResources {
		output.SaveResult(c.BundlePath, fmt.Sprintf("cluster-resources/custom-resources/%v", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/custom-resources/custom-resources-errors.json", marshalErrors(crErrors))

	// imagepullsecrets
	imagePullSecrets, pullSecretsErrors := imagePullSecrets(ctx, client, namespaceNames)
	for k, v := range imagePullSecrets {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/image-pull-secrets", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/image-pull-secrets-errors.json", marshalErrors(pullSecretsErrors))

	// nodes
	nodes, nodeErrors := nodes(ctx, client)
	output.SaveResult(c.BundlePath, "cluster-resources/nodes.json", bytes.NewBuffer(nodes))
	output.SaveResult(c.BundlePath, "cluster-resources/nodes-errors.json", marshalErrors(nodeErrors))

	groups, resources, groupsResourcesErrors := apiResources(ctx, client)
	output.SaveResult(c.BundlePath, "cluster-resources/groups.json", bytes.NewBuffer(groups))
	output.SaveResult(c.BundlePath, "cluster-resources/resources.json", bytes.NewBuffer(resources))
	output.SaveResult(c.BundlePath, "cluster-resources/groups-resources-errors.json", marshalErrors(groupsResourcesErrors))

	// limit ranges
	limitRanges, limitRangesErrors := limitRanges(ctx, client, namespaceNames)
	for k, v := range limitRanges {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/limitranges", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/limitranges-errors.json", marshalErrors(limitRangesErrors))

	//Events
	events, eventsErrors := events(ctx, client, namespaceNames)
	for k, v := range events {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/events", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/events-errors.json", marshalErrors(eventsErrors))

	//Persistent Volumes
	pvs, pvsErrors := pvs(ctx, client)
	output.SaveResult(c.BundlePath, "cluster-resources/pvs.json", bytes.NewBuffer(pvs))
	output.SaveResult(c.BundlePath, "cluster-resources/pvs-errors.json", marshalErrors(pvsErrors))

	//Persistent Volume Claims
	pvcs, pvcsErrors := pvcs(ctx, client, namespaceNames)
	for k, v := range pvcs {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/pvcs", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/pvcs-errors.json", marshalErrors(pvcsErrors))

	//Roles
	roles, rolesErrors := roles(ctx, client, namespaceNames)
	for k, v := range roles {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/roles", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/roles-errors.json", marshalErrors(rolesErrors))

	//Role Bindings
	roleBindings, roleBindingsErrors := roleBindings(ctx, client, namespaceNames)
	for k, v := range roleBindings {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/rolebindings", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/rolebindings-errors.json", marshalErrors(roleBindingsErrors))

	//Cluster Roles
	clusterRoles, clusterRolesErrors := clusterRoles(ctx, client)
	output.SaveResult(c.BundlePath, "cluster-resources/clusterroles.json", bytes.NewBuffer(clusterRoles))
	output.SaveResult(c.BundlePath, "cluster-resources/clusterroles-errors.json", marshalErrors(clusterRolesErrors))

	//Cluster Role Bindings
	clusterRoleBindings, clusterRoleBindingsErrors := clusterRoleBindings(ctx, client)
	output.SaveResult(c.BundlePath, "cluster-resources/clusterRoleBindings.json", bytes.NewBuffer(clusterRoleBindings))
	output.SaveResult(c.BundlePath, "cluster-resources/clusterRoleBindings-errors.json", marshalErrors(clusterRoleBindingsErrors))

	return output, nil
}

func getAllNamespaces(ctx context.Context, client *kubernetes.Clientset) ([]byte, *corev1.NamespaceList, []string) {
	namespaces, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, []string{err.Error()}
	}

	gvk, err := apiutil.GVKForObject(namespaces, scheme.Scheme)
	if err == nil {
		namespaces.GetObjectKind().SetGroupVersionKind(gvk)
	}

	for i, o := range namespaces.Items {
		gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
		if err == nil {
			namespaces.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
		}
	}

	b, err := json.MarshalIndent(namespaces, "", "  ")
	if err != nil {
		return nil, nil, []string{err.Error()}
	}

	return b, namespaces, nil
}

func getNamespaces(ctx context.Context, client *kubernetes.Clientset, namespaces []string) ([]byte, []string) {
	namespacesArr := []*corev1.Namespace{}
	errorsArr := []string{}

	for _, namespace := range namespaces {
		ns, err := client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if err != nil {
			errorsArr = append(errorsArr, err.Error())
			continue
		}
		namespacesArr = append(namespacesArr, ns)
	}

	b, err := json.MarshalIndent(namespacesArr, "", "  ")
	if err != nil {
		errorsArr = append(errorsArr, err.Error())
		return nil, errorsArr
	}

	return b, errorsArr
}

func getNamespace(ctx context.Context, client *kubernetes.Clientset, namespace string) ([]byte, []string) {
	ns, err := client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return nil, []string{err.Error()}
	}

	gvk, err := apiutil.GVKForObject(ns, scheme.Scheme)
	if err == nil {
		ns.GetObjectKind().SetGroupVersionKind(gvk)
	}

	b, err := json.MarshalIndent(ns, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}

	return b, nil
}

func pods(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string, []corev1.Pod) {
	podsByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)
	unhealthyPods := []corev1.Pod{}

	for _, namespace := range namespaces {
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		gvk, err := apiutil.GVKForObject(pods, scheme.Scheme)
		if err == nil {
			pods.GetObjectKind().SetGroupVersionKind(gvk)
		}

		for i, o := range pods.Items {
			gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
			if err == nil {
				pods.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
			}
		}

		b, err := json.MarshalIndent(pods, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		for _, pod := range pods.Items {
			if k8sutil.IsPodUnhealthy(&pod) {
				unhealthyPods = append(unhealthyPods, pod)
			}
		}

		podsByNamespace[namespace+".json"] = b
	}

	return podsByNamespace, errorsByNamespace, unhealthyPods
}

func getPodDisruptionBudgets(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	pdbByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		PodDisruptionBudgets, err := client.PolicyV1beta1().PodDisruptionBudgets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		gvk, err := apiutil.GVKForObject(PodDisruptionBudgets, scheme.Scheme)
		if err == nil {
			PodDisruptionBudgets.GetObjectKind().SetGroupVersionKind(gvk)
		}

		b, err := json.MarshalIndent(PodDisruptionBudgets, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		pdbByNamespace[namespace+".json"] = b
	}

	return pdbByNamespace, errorsByNamespace
}

func services(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	servicesByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		services, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		gvk, err := apiutil.GVKForObject(services, scheme.Scheme)
		if err == nil {
			services.GetObjectKind().SetGroupVersionKind(gvk)
		}

		for i, o := range services.Items {
			gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
			if err == nil {
				services.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
			}
		}

		b, err := json.MarshalIndent(services, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		servicesByNamespace[namespace+".json"] = b
	}

	return servicesByNamespace, errorsByNamespace
}

func deployments(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	deploymentsByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		deployments, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		gvk, err := apiutil.GVKForObject(deployments, scheme.Scheme)
		if err == nil {
			deployments.GetObjectKind().SetGroupVersionKind(gvk)
		}

		for i, o := range deployments.Items {
			gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
			if err == nil {
				deployments.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
			}
		}

		b, err := json.MarshalIndent(deployments, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		deploymentsByNamespace[namespace+".json"] = b
	}

	return deploymentsByNamespace, errorsByNamespace
}

func statefulsets(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	statefulsetsByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		statefulsets, err := client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		gvk, err := apiutil.GVKForObject(statefulsets, scheme.Scheme)
		if err == nil {
			statefulsets.GetObjectKind().SetGroupVersionKind(gvk)
		}

		for i, o := range statefulsets.Items {
			gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
			if err == nil {
				statefulsets.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
			}
		}

		b, err := json.MarshalIndent(statefulsets, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		statefulsetsByNamespace[namespace+".json"] = b
	}

	return statefulsetsByNamespace, errorsByNamespace
}

func replicasets(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	replicasetsByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		replicasets, err := client.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		gvk, err := apiutil.GVKForObject(replicasets, scheme.Scheme)
		if err == nil {
			replicasets.GetObjectKind().SetGroupVersionKind(gvk)
		}

		for i, o := range replicasets.Items {
			gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
			if err == nil {
				replicasets.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
			}
		}

		b, err := json.MarshalIndent(replicasets, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		replicasetsByNamespace[namespace+".json"] = b
	}

	return replicasetsByNamespace, errorsByNamespace
}

func jobs(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	jobsByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		nsJobs, err := client.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		gvk, err := apiutil.GVKForObject(nsJobs, scheme.Scheme)
		if err == nil {
			nsJobs.GetObjectKind().SetGroupVersionKind(gvk)
		}

		for i, o := range nsJobs.Items {
			gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
			if err == nil {
				nsJobs.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
			}
		}

		b, err := json.MarshalIndent(nsJobs, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		jobsByNamespace[namespace+".json"] = b
	}

	return jobsByNamespace, errorsByNamespace
}

func cronJobs(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	cronJobsByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		nsCronJobs, err := client.BatchV1beta1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		gvk, err := apiutil.GVKForObject(nsCronJobs, scheme.Scheme)
		if err == nil {
			nsCronJobs.GetObjectKind().SetGroupVersionKind(gvk)
		}

		for i, o := range nsCronJobs.Items {
			gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
			if err == nil {
				nsCronJobs.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
			}
		}

		b, err := json.MarshalIndent(nsCronJobs, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		cronJobsByNamespace[namespace+".json"] = b
	}

	return cronJobsByNamespace, errorsByNamespace
}

func ingress(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	ok, err := discovery.HasResource(client, "networking.k8s.io/v1", "Ingress")
	if err != nil {
		return nil, map[string]string{"": err.Error()}
	}
	if ok {
		return ingressV1(ctx, client, namespaces)
	}

	return ingressV1beta(ctx, client, namespaces)
}

func ingressV1(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	ingressByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		ingress, err := client.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		gvk, err := apiutil.GVKForObject(ingress, scheme.Scheme)
		if err == nil {
			ingress.GetObjectKind().SetGroupVersionKind(gvk)
		}

		for i, o := range ingress.Items {
			gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
			if err == nil {
				ingress.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
			}
		}

		b, err := json.MarshalIndent(ingress, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		ingressByNamespace[namespace+".json"] = b
	}

	return ingressByNamespace, errorsByNamespace
}

func ingressV1beta(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	ingressByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		ingress, err := client.ExtensionsV1beta1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		gvk, err := apiutil.GVKForObject(ingress, scheme.Scheme)
		if err == nil {
			ingress.GetObjectKind().SetGroupVersionKind(gvk)
		}

		for i, o := range ingress.Items {
			gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
			if err == nil {
				ingress.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
			}
		}

		b, err := json.MarshalIndent(ingress, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		ingressByNamespace[namespace+".json"] = b
	}

	return ingressByNamespace, errorsByNamespace
}

func networkPolicy(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	networkPolicyByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		networkPolicy, err := client.NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		gvk, err := apiutil.GVKForObject(networkPolicy, scheme.Scheme)
		if err == nil {
			networkPolicy.GetObjectKind().SetGroupVersionKind(gvk)
		}

		for i, o := range networkPolicy.Items {
			gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
			if err == nil {
				networkPolicy.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
			}
		}

		b, err := json.MarshalIndent(networkPolicy, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		networkPolicyByNamespace[namespace+".json"] = b
	}

	return networkPolicyByNamespace, errorsByNamespace
}

func resourceQuota(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	resourceQuotaByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		resourceQuota, err := client.CoreV1().ResourceQuotas(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		gvk, err := apiutil.GVKForObject(resourceQuota, scheme.Scheme)
		if err == nil {
			resourceQuota.GetObjectKind().SetGroupVersionKind(gvk)
		}

		for i, o := range resourceQuota.Items {
			gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
			if err == nil {
				resourceQuota.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
			}
		}

		b, err := json.MarshalIndent(resourceQuota, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		resourceQuotaByNamespace[namespace+".json"] = b
	}

	return resourceQuotaByNamespace, errorsByNamespace
}

func storageClasses(ctx context.Context, client *kubernetes.Clientset) ([]byte, []string) {
	ok, err := discovery.HasResource(client, "storage.k8s.io/v1", "StorageClass")
	if err != nil {
		return nil, []string{err.Error()}
	}
	if ok {
		return storageClassesV1(ctx, client)
	}

	return storageClassesV1beta(ctx, client)
}

func storageClassesV1(ctx context.Context, client *kubernetes.Clientset) ([]byte, []string) {
	storageClasses, err := client.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, []string{err.Error()}
	}

	gvk, err := apiutil.GVKForObject(storageClasses, scheme.Scheme)
	if err == nil {
		storageClasses.GetObjectKind().SetGroupVersionKind(gvk)
	}

	for i, o := range storageClasses.Items {
		gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
		if err == nil {
			storageClasses.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
		}
	}

	b, err := json.MarshalIndent(storageClasses, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}

	return b, nil
}

func storageClassesV1beta(ctx context.Context, client *kubernetes.Clientset) ([]byte, []string) {
	storageClasses, err := client.StorageV1beta1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, []string{err.Error()}
	}

	gvk, err := apiutil.GVKForObject(storageClasses, scheme.Scheme)
	if err == nil {
		storageClasses.GetObjectKind().SetGroupVersionKind(gvk)
	}

	for i, o := range storageClasses.Items {
		gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
		if err == nil {
			storageClasses.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
		}
	}

	b, err := json.MarshalIndent(storageClasses, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}

	return b, nil
}

func crds(ctx context.Context, client *kubernetes.Clientset, config *rest.Config) ([]byte, []string) {
	ok, err := discovery.HasResource(client, "apiextensions.k8s.io/v1", "CustomResourceDefinition")
	if err != nil {
		return nil, []string{err.Error()}
	}
	if ok {
		return crdsV1(ctx, config)
	}

	return crdsV1beta(ctx, config)
}

func crdsV1(ctx context.Context, config *rest.Config) ([]byte, []string) {
	client, err := apiextensionsv1clientset.NewForConfig(config)
	if err != nil {
		return nil, []string{err.Error()}
	}

	crds, err := client.CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, []string{err.Error()}
	}

	gvk, err := apiutil.GVKForObject(crds, scheme.Scheme)
	if err == nil {
		crds.GetObjectKind().SetGroupVersionKind(gvk)
	}

	for i, o := range crds.Items {
		gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
		if err == nil {
			crds.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
		}
	}

	b, err := json.MarshalIndent(crds, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}

	return b, nil
}

func crdsV1beta(ctx context.Context, config *rest.Config) ([]byte, []string) {
	client, err := apiextensionsv1beta1clientset.NewForConfig(config)
	if err != nil {
		return nil, []string{err.Error()}
	}

	crds, err := client.CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, []string{err.Error()}
	}

	gvk, err := apiutil.GVKForObject(crds, scheme.Scheme)
	if err == nil {
		crds.GetObjectKind().SetGroupVersionKind(gvk)
	}

	for i, o := range crds.Items {
		gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
		if err == nil {
			crds.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
		}
	}

	b, err := json.MarshalIndent(crds, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}

	return b, nil
}

func crs(ctx context.Context, dyn dynamic.Interface, client *kubernetes.Clientset, config *rest.Config, namespaces []string) (map[string][]byte, map[string]string) {
	ok, err := discovery.HasResource(client, "apiextensions.k8s.io/v1", "CustomResourceDefinition")
	if err != nil {
		return nil, map[string]string{"discover apiextensions.k8s.io/v1": err.Error()}
	}
	if ok {
		return crsV1(ctx, dyn, config, namespaces)
	}

	return crsV1beta(ctx, dyn, config, namespaces)
}

// Selects the newest version by kube-aware priority.
func selectCRDVersionByPriority(versions []string) string {
	if len(versions) == 0 {
		return ""
	}

	sort.Slice(versions, func(i, j int) bool {
		return version.CompareKubeAwareVersionStrings(versions[i], versions[j]) < 0
	})
	return versions[len(versions)-1]
}

func crsV1(ctx context.Context, client dynamic.Interface, config *rest.Config, namespaces []string) (map[string][]byte, map[string]string) {
	customResources := make(map[string][]byte)
	errorList := make(map[string]string)

	crdClient, err := apiextensionsv1clientset.NewForConfig(config)
	if err != nil {
		errorList["crdClient"] = err.Error()
		return customResources, errorList
	}

	crds, err := crdClient.CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		errorList["crdList"] = err.Error()
		return customResources, errorList
	}

	metaAccessor := meta.NewAccessor()

	// Loop through CRDs to fetch the CRs
	for _, crd := range crds.Items {
		// A resource that contains '/' is a subresource type and it has no
		// object instances
		if strings.ContainsAny(crd.Name, "/") {
			continue
		}

		var version string
		if len(crd.Spec.Versions) > 0 {
			versions := []string{}
			for _, v := range crd.Spec.Versions {
				versions = append(versions, v.Name)
			}

			version = versions[0]
			if len(versions) > 1 {
				version = selectCRDVersionByPriority(versions)
			}
		}
		gvr := schema.GroupVersionResource{
			Group:    crd.Spec.Group,
			Version:  version,
			Resource: crd.Spec.Names.Plural,
		}
		isNamespacedResource := crd.Spec.Scope == apiextensionsv1.NamespaceScoped

		// Fetch all resources of given type
		customResourceList, err := client.Resource(gvr).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorList[crd.Name] = err.Error()
			continue
		}

		if len(customResourceList.Items) == 0 {
			continue
		}

		if !isNamespacedResource {
			objects := []map[string]interface{}{}
			for _, item := range customResourceList.Items {
				objects = append(objects, item.Object)
			}
			b, err := yaml.Marshal(objects)
			if err != nil {
				errorList[crd.Name] = err.Error()
				continue
			}
			customResources[fmt.Sprintf("%s.yaml", crd.Name)] = b
		} else {
			// Group fetched resources by the namespace
			perNamespace := map[string][]map[string]interface{}{}
			errors := []string{}

			for _, item := range customResourceList.Items {
				ns, err := metaAccessor.Namespace(&item)
				if err != nil {
					errors = append(errors, err.Error())
					continue
				}
				if perNamespace[ns] == nil {
					perNamespace[ns] = []map[string]interface{}{}
				}
				perNamespace[ns] = append(perNamespace[ns], item.Object)
			}

			if len(errors) > 0 {
				errorList[crd.Name] = strings.Join(errors, "\n")
			}

			// Only include resources from requested namespaces
			for _, ns := range namespaces {
				if len(perNamespace[ns]) == 0 {
					continue
				}

				namespacedName := fmt.Sprintf("%s/%s", crd.Name, ns)
				b, err := yaml.Marshal(perNamespace[ns])
				if err != nil {
					errorList[namespacedName] = err.Error()
					continue
				}

				customResources[fmt.Sprintf("%s.yaml", namespacedName)] = b
			}
		}
	}

	return customResources, errorList
}

func crsV1beta(ctx context.Context, client dynamic.Interface, config *rest.Config, namespaces []string) (map[string][]byte, map[string]string) {
	customResources := make(map[string][]byte)
	errorList := make(map[string]string)

	crdClient, err := apiextensionsv1beta1clientset.NewForConfig(config)
	if err != nil {
		errorList["crdClient"] = err.Error()
		return customResources, errorList
	}

	crds, err := crdClient.CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		errorList["crdList"] = err.Error()
		return customResources, errorList
	}

	metaAccessor := meta.NewAccessor()

	// Loop through CRDs to fetch the CRs
	for _, crd := range crds.Items {
		// A resource that contains '/' is a subresource type and it has no
		// object instances
		if strings.ContainsAny(crd.Name, "/") {
			continue
		}

		gvr := schema.GroupVersionResource{
			Group:    crd.Spec.Group,
			Version:  crd.Spec.Version,
			Resource: crd.Spec.Names.Plural,
		}

		if len(crd.Spec.Versions) > 0 {
			versions := []string{}
			for _, v := range crd.Spec.Versions {
				versions = append(versions, v.Name)
			}

			version := versions[0]
			if len(versions) > 1 {
				version = selectCRDVersionByPriority(versions)
			}
			gvr.Version = version
		}

		isNamespacedResource := crd.Spec.Scope == apiextensionsv1beta1.NamespaceScoped

		// Fetch all resources of given type
		customResourceList, err := client.Resource(gvr).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorList[crd.Name] = err.Error()
			continue
		}

		if len(customResourceList.Items) == 0 {
			continue
		}

		if !isNamespacedResource {
			objects := []map[string]interface{}{}
			for _, item := range customResourceList.Items {
				objects = append(objects, item.Object)
			}
			b, err := yaml.Marshal(customResourceList.Items)
			if err != nil {
				errorList[crd.Name] = err.Error()
				continue
			}
			customResources[fmt.Sprintf("%s.yaml", crd.Name)] = b
		} else {
			// Group fetched resources by the namespace
			perNamespace := map[string][]map[string]interface{}{}
			errors := []string{}

			for _, item := range customResourceList.Items {
				ns, err := metaAccessor.Namespace(&item)
				if err != nil {
					errors = append(errors, err.Error())
					continue
				}
				if perNamespace[ns] == nil {
					perNamespace[ns] = []map[string]interface{}{}
				}
				perNamespace[ns] = append(perNamespace[ns], item.Object)
			}

			if len(errors) > 0 {
				errorList[crd.Name] = strings.Join(errors, "\n")
			}

			// Only include resources from requested namespaces
			for _, ns := range namespaces {
				if len(perNamespace[ns]) == 0 {
					continue
				}

				namespacedName := fmt.Sprintf("%s/%s", crd.Name, ns)
				b, err := yaml.Marshal(perNamespace[ns])
				if err != nil {
					errorList[namespacedName] = err.Error()
					continue
				}

				customResources[fmt.Sprintf("%s.yaml", namespacedName)] = b
			}
		}
	}

	return customResources, errorList
}

func imagePullSecrets(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	imagePullSecrets := make(map[string][]byte)
	errors := make(map[string]string)

	// better than vendoring in.... kubernetes
	type DockerConfigEntry struct {
		Auth string `json:"auth"`
	}
	type DockerConfigJSON struct {
		Auths map[string]DockerConfigEntry `json:"auths"`
	}

	for _, namespace := range namespaces {
		secrets, err := client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errors[namespace] = err.Error()
			continue
		}

		for _, secret := range secrets.Items {
			if secret.Type != corev1.SecretTypeDockerConfigJson {
				continue
			}

			dockerConfigJSON := DockerConfigJSON{}
			if err := json.Unmarshal(secret.Data[corev1.DockerConfigJsonKey], &dockerConfigJSON); err != nil {
				errors[fmt.Sprintf("%s/%s", namespace, secret.Name)] = err.Error()
				continue
			}

			for registry, registryAuth := range dockerConfigJSON.Auths {
				decoded, err := base64.StdEncoding.DecodeString(registryAuth.Auth)
				if err != nil {
					errors[fmt.Sprintf("%s/%s/%s", namespace, secret.Name, registry)] = err.Error()
					continue
				}

				registryAndUsername := make(map[string]string)
				registryAndUsername[registry] = strings.Split(string(decoded), ":")[0]
				b, err := json.Marshal(registryAndUsername)
				if err != nil {
					errors[fmt.Sprintf("%s/%s/%s", namespace, secret.Name, registry)] = err.Error()
					continue
				}
				imagePullSecrets[fmt.Sprintf("%s/%s.json", namespace, secret.Name)] = b
			}
		}
	}

	return imagePullSecrets, errors
}

func limitRanges(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	limitRangesByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		limitRanges, err := client.CoreV1().LimitRanges(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		gvk, err := apiutil.GVKForObject(limitRanges, scheme.Scheme)
		if err == nil {
			limitRanges.GetObjectKind().SetGroupVersionKind(gvk)
		}

		for i, o := range limitRanges.Items {
			gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
			if err == nil {
				limitRanges.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
			}
		}

		b, err := json.MarshalIndent(limitRanges, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		limitRangesByNamespace[namespace+".json"] = b
	}

	return limitRangesByNamespace, errorsByNamespace
}

func nodes(ctx context.Context, client *kubernetes.Clientset) ([]byte, []string) {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, []string{err.Error()}
	}

	gvk, err := apiutil.GVKForObject(nodes, scheme.Scheme)
	if err == nil {
		nodes.GetObjectKind().SetGroupVersionKind(gvk)
	}

	for i, o := range nodes.Items {
		gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
		if err == nil {
			nodes.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
		}
	}

	b, err := json.MarshalIndent(nodes, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}

	return b, nil
}

// get the list of API resources, similar to 'kubectl api-resources'
func apiResources(ctx context.Context, client *kubernetes.Clientset) ([]byte, []byte, []string) {
	var errorArray []string
	groups, resources, err := client.Discovery().ServerGroupsAndResources()
	if err != nil {
		errorArray = append(errorArray, err.Error())
	}

	groupBytes, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		errorArray = append(errorArray, err.Error())
	}

	resourcesBytes, err := json.MarshalIndent(resources, "", "  ")
	if err != nil {
		errorArray = append(errorArray, err.Error())
	}

	return groupBytes, resourcesBytes, errorArray
}

func getSelfSubjectRulesReviews(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string]*authorizationv1.SubjectRulesReviewStatus, map[string]string) {
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubectl/cmd/auth/cani.go

	statusByNamespace := make(map[string]*authorizationv1.SubjectRulesReviewStatus)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		sar := &authorizationv1.SelfSubjectRulesReview{
			Spec: authorizationv1.SelfSubjectRulesReviewSpec{
				Namespace: namespace,
			},
		}
		response, err := client.AuthorizationV1().SelfSubjectRulesReviews().Create(ctx, sar, metav1.CreateOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		statusByNamespace[namespace] = response.Status.DeepCopy()
	}

	return statusByNamespace, errorsByNamespace
}

func authCanI(accessStatuses map[string]*authorizationv1.SubjectRulesReviewStatus, namespaces []string) map[string][]byte {
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubectl/cmd/auth/cani.go

	authListByNamespace := make(map[string][]byte)

	for _, namespace := range namespaces {
		accessStatus := accessStatuses[namespace]
		if accessStatus == nil {
			continue
		}

		rules := convertToPolicyRule(accessStatus)
		b, _ := json.MarshalIndent(rules, "", "  ")
		authListByNamespace[namespace+".json"] = b
	}

	return authListByNamespace
}

func events(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	eventsByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		events, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		gvk, err := apiutil.GVKForObject(events, scheme.Scheme)
		if err == nil {
			events.GetObjectKind().SetGroupVersionKind(gvk)
		}

		for i, o := range events.Items {
			gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
			if err == nil {
				events.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
			}
		}

		b, err := json.MarshalIndent(events, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		eventsByNamespace[namespace+".json"] = b
	}

	return eventsByNamespace, errorsByNamespace
}

func canCollectNamespaceResources(status *authorizationv1.SubjectRulesReviewStatus) bool {
	// This is all very approximate

	for _, resource := range status.ResourceRules {
		hasGet := false
		for _, verb := range resource.Verbs {
			if verb == "*" || verb == "get" {
				hasGet = true
				break
			}
		}

		hasAPI := false
		for _, group := range resource.APIGroups {
			if group == "*" || group == "" {
				hasAPI = true
				break
			}
		}

		hasPods := false
		for _, resource := range resource.Resources {
			if resource == "*" || resource == "pods" { // pods is the bare minimum
				hasPods = true
				break
			}
		}

		if hasGet && hasAPI && hasPods {
			return true
		}
	}

	return false
}

// not exported from: https://github.com/kubernetes/kubernetes/blob/master/pkg/kubectl/cmd/auth/cani.go#L339
func convertToPolicyRule(status *authorizationv1.SubjectRulesReviewStatus) []rbacv1.PolicyRule {
	ret := []rbacv1.PolicyRule{}
	for _, resource := range status.ResourceRules {
		ret = append(ret, rbacv1.PolicyRule{
			Verbs:         resource.Verbs,
			APIGroups:     resource.APIGroups,
			Resources:     resource.Resources,
			ResourceNames: resource.ResourceNames,
		})
	}

	for _, nonResource := range status.NonResourceRules {
		ret = append(ret, rbacv1.PolicyRule{
			Verbs:           nonResource.Verbs,
			NonResourceURLs: nonResource.NonResourceURLs,
		})
	}

	return ret
}

func pvs(ctx context.Context, client *kubernetes.Clientset) ([]byte, []string) {
	pv, err := client.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, []string{err.Error()}
	}

	gvk, err := apiutil.GVKForObject(pv, scheme.Scheme)
	if err == nil {
		pv.GetObjectKind().SetGroupVersionKind(gvk)
	}

	for i, o := range pv.Items {
		gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
		if err == nil {
			pv.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
		}
	}

	b, err := json.MarshalIndent(pv, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}
	return b, nil
}

func pvcs(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	pvcsByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		pvcs, err := client.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		gvk, err := apiutil.GVKForObject(pvcs, scheme.Scheme)
		if err == nil {
			pvcs.GetObjectKind().SetGroupVersionKind(gvk)
		}

		for i, o := range pvcs.Items {
			gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
			if err == nil {
				pvcs.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
			}
		}

		b, err := json.MarshalIndent(pvcs, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		pvcsByNamespace[namespace+".json"] = b
	}

	return pvcsByNamespace, errorsByNamespace
}

func roles(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	rolesByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		roles, err := client.RbacV1().Roles(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		gvk, err := apiutil.GVKForObject(roles, scheme.Scheme)
		if err == nil {
			roles.GetObjectKind().SetGroupVersionKind(gvk)
		}

		for i, o := range roles.Items {
			gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
			if err == nil {
				roles.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
			}
		}

		b, err := json.MarshalIndent(roles, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		rolesByNamespace[namespace+".json"] = b
	}

	return rolesByNamespace, errorsByNamespace
}

func roleBindings(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	roleBindingsByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		roleBindings, err := client.RbacV1().RoleBindings(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		gvk, err := apiutil.GVKForObject(roleBindings, scheme.Scheme)
		if err == nil {
			roleBindings.GetObjectKind().SetGroupVersionKind(gvk)
		}

		for i, o := range roleBindings.Items {
			gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
			if err == nil {
				roleBindings.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
			}
		}

		b, err := json.MarshalIndent(roleBindings, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		roleBindingsByNamespace[namespace+".json"] = b
	}

	return roleBindingsByNamespace, errorsByNamespace
}

func clusterRoles(ctx context.Context, client *kubernetes.Clientset) ([]byte, []string) {
	clusterRoles, err := client.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, []string{err.Error()}
	}

	gvk, err := apiutil.GVKForObject(clusterRoles, scheme.Scheme)
	if err == nil {
		clusterRoles.GetObjectKind().SetGroupVersionKind(gvk)
	}

	for i, o := range clusterRoles.Items {
		gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
		if err == nil {
			clusterRoles.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
		}
	}

	b, err := json.MarshalIndent(clusterRoles, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}
	return b, nil
}

func clusterRoleBindings(ctx context.Context, client *kubernetes.Clientset) ([]byte, []string) {
	clusterRoleBindings, err := client.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, []string{err.Error()}
	}

	gvk, err := apiutil.GVKForObject(clusterRoleBindings, scheme.Scheme)
	if err == nil {
		clusterRoleBindings.GetObjectKind().SetGroupVersionKind(gvk)
	}

	for i, o := range clusterRoleBindings.Items {
		gvk, err := apiutil.GVKForObject(&o, scheme.Scheme)
		if err == nil {
			clusterRoleBindings.Items[i].GetObjectKind().SetGroupVersionKind(gvk)
		}
	}

	b, err := json.MarshalIndent(clusterRoleBindings, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}
	return b, nil
}
