package collect

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path" // this code uses 'path' and not 'path/filepath' because we don't want backslashes on windows
	"strings"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1beta1clientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func ClusterResources(c *Collector) (map[string][]byte, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	clusterResourcesOutput := map[string][]byte{}
	// namespaces
	var namespaceNames []string
	if c.Namespace == "" {
		namespaces, namespaceList, namespaceErrors := namespaces(ctx, client)
		clusterResourcesOutput["cluster-resources/namespaces.json"] = namespaces
		clusterResourcesOutput["cluster-resources/namespaces-errors.json"], err = marshalNonNil(namespaceErrors)
		if err != nil {
			return nil, err
		}
		if namespaceList != nil {
			for _, namespace := range namespaceList.Items {
				namespaceNames = append(namespaceNames, namespace.Name)
			}
		}
	} else {
		namespaces, namespaceErrors := getNamespace(ctx, client, c.Namespace)
		clusterResourcesOutput["cluster-resources/namespaces.json"] = namespaces
		clusterResourcesOutput["cluster-resources/namespaces-errors.json"], err = marshalNonNil(namespaceErrors)
		if err != nil {
			return nil, err
		}
		namespaceNames = append(namespaceNames, c.Namespace)
	}
	pods, podErrors := pods(ctx, client, namespaceNames)
	for k, v := range pods {
		clusterResourcesOutput[path.Join("cluster-resources/pods", k)] = v
	}
	clusterResourcesOutput["cluster-resources/pods-errors.json"], err = marshalNonNil(podErrors)
	if err != nil {
		return nil, err
	}

	// services
	services, servicesErrors := services(ctx, client, namespaceNames)
	for k, v := range services {
		clusterResourcesOutput[path.Join("cluster-resources/services", k)] = v
	}
	clusterResourcesOutput["cluster-resources/services-errors.json"], err = marshalNonNil(servicesErrors)
	if err != nil {
		return nil, err
	}

	// deployments
	deployments, deploymentsErrors := deployments(ctx, client, namespaceNames)
	for k, v := range deployments {
		clusterResourcesOutput[path.Join("cluster-resources/deployments", k)] = v
	}
	clusterResourcesOutput["cluster-resources/deployments-errors.json"], err = marshalNonNil(deploymentsErrors)
	if err != nil {
		return nil, err
	}

	// statefulsets
	statefulsets, statefulsetsErrors := statefulsets(ctx, client, namespaceNames)
	for k, v := range statefulsets {
		clusterResourcesOutput[path.Join("cluster-resources/statefulsets", k)] = v
	}
	clusterResourcesOutput["cluster-resources/statefulsets-errors.json"], err = marshalNonNil(statefulsetsErrors)
	if err != nil {
		return nil, err
	}

	// jobs
	jobs, jobsErrors := jobs(ctx, client, namespaceNames)
	for k, v := range jobs {
		clusterResourcesOutput[path.Join("cluster-resources/jobs", k)] = v
	}
	clusterResourcesOutput["cluster-resources/jobs-errors.json"], err = marshalNonNil(jobsErrors)
	if err != nil {
		return nil, err
	}

	// cronJobs
	cronJobs, cronJobsErrors := cronJobs(ctx, client, namespaceNames)
	for k, v := range cronJobs {
		clusterResourcesOutput[path.Join("cluster-resources/cronjobs", k)] = v
	}
	clusterResourcesOutput["cluster-resources/cronjobs-errors.json"], err = marshalNonNil(cronJobsErrors)
	if err != nil {
		return nil, err
	}

	// ingress
	ingress, ingressErrors := ingress(ctx, client, namespaceNames)
	for k, v := range ingress {
		clusterResourcesOutput[path.Join("cluster-resources/ingress", k)] = v
	}
	clusterResourcesOutput["cluster-resources/ingress-errors.json"], err = marshalNonNil(ingressErrors)
	if err != nil {
		return nil, err
	}

	// storage classes
	storageClasses, storageErrors := storageClasses(ctx, client)
	clusterResourcesOutput["cluster-resources/storage-classes.json"] = storageClasses
	clusterResourcesOutput["cluster-resources/storage-errors.json"], err = marshalNonNil(storageErrors)
	if err != nil {
		return nil, err
	}

	// crds
	crdClient, err := apiextensionsv1beta1clientset.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	}
	customResourceDefinitions, crdErrors := crds(ctx, crdClient)
	clusterResourcesOutput["cluster-resources/custom-resource-definitions.json"] = customResourceDefinitions
	clusterResourcesOutput["cluster-resources/custom-resource-definitions-errors.json"], err = marshalNonNil(crdErrors)
	if err != nil {
		return nil, err
	}

	// imagepullsecrets
	imagePullSecrets, pullSecretsErrors := imagePullSecrets(ctx, client, namespaceNames)
	for k, v := range imagePullSecrets {
		clusterResourcesOutput[path.Join("cluster-resources/image-pull-secrets", k)] = v
	}
	clusterResourcesOutput["cluster-resources/image-pull-secrets-errors.json"], err = marshalNonNil(pullSecretsErrors)
	if err != nil {
		return nil, err
	}

	// nodes
	nodes, nodeErrors := nodes(ctx, client)
	clusterResourcesOutput["cluster-resources/nodes.json"] = nodes
	clusterResourcesOutput["cluster-resources/nodes-errors.json"], err = marshalNonNil(nodeErrors)
	if err != nil {
		return nil, err
	}

	groups, resources, groupsResourcesErrors := apiResources(ctx, client)
	clusterResourcesOutput["cluster-resources/groups.json"] = groups
	clusterResourcesOutput["cluster-resources/resources.json"] = resources
	clusterResourcesOutput["cluster-resources/groups-resources-errors.json"], err = marshalNonNil(groupsResourcesErrors)
	if err != nil {
		return nil, err
	}

	// limit ranges
	limitRanges, limitRangesErrors := limitRanges(ctx, client, namespaceNames)
	for k, v := range limitRanges {
		clusterResourcesOutput[path.Join("cluster-resources/limitranges", k)] = v
	}
	clusterResourcesOutput["cluster-resources/limitranges-errors.json"], err = marshalNonNil(limitRangesErrors)
	if err != nil {
		return nil, err
	}

	// auth cani
	authCanI, authCanIErrors := authCanI(ctx, client, namespaceNames)
	for k, v := range authCanI {
		clusterResourcesOutput[path.Join("cluster-resources/auth-cani-list", k)] = v
	}
	clusterResourcesOutput["cluster-resources/auth-cani-list-errors.json"], err = marshalNonNil(authCanIErrors)
	if err != nil {
		return nil, err
	}

	//Events
	events, eventsErrors := events(ctx, client, namespaceNames)
	for k, v := range events {
		clusterResourcesOutput[path.Join("cluster-resources/events", k)] = v
	}
	clusterResourcesOutput["cluster-resources/events-errors.json"], err = marshalNonNil(eventsErrors)
	if err != nil {
		return nil, err
	}

	//Persistent Volumes
	pvs, pvsErrors := pvs(ctx, client)
	clusterResourcesOutput["cluster-resources/pvs.json"] = pvs
	clusterResourcesOutput["cluster-resources/pvs-errors.json"], err = marshalNonNil(pvsErrors)
	if err != nil {
		return nil, err
	}

	//Persistent Volume Claims
	pvcs, pvcsErrors := pvcs(ctx, client, namespaceNames)
	for k, v := range pvcs {
		clusterResourcesOutput[path.Join("cluster-resources/pvcs", k)] = v
	}
	clusterResourcesOutput["cluster-resources/pvcs-errors.json"], err = marshalNonNil(pvcsErrors)
	if err != nil {
		return nil, err
	}

	return clusterResourcesOutput, nil
}

func namespaces(ctx context.Context, client *kubernetes.Clientset) ([]byte, *corev1.NamespaceList, []string) {
	namespaces, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, []string{err.Error()}
	}

	b, err := json.MarshalIndent(namespaces.Items, "", "  ")
	if err != nil {
		return nil, nil, []string{err.Error()}
	}

	return b, namespaces, nil
}

func getNamespace(ctx context.Context, client *kubernetes.Clientset, namespace string) ([]byte, []string) {
	namespaces, err := client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return nil, []string{err.Error()}
	}

	b, err := json.MarshalIndent(namespaces, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}

	return b, nil
}

func pods(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	podsByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		b, err := json.MarshalIndent(pods.Items, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		podsByNamespace[namespace+".json"] = b
	}

	return podsByNamespace, errorsByNamespace
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

		b, err := json.MarshalIndent(services.Items, "", "  ")
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

		b, err := json.MarshalIndent(deployments.Items, "", "  ")
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

		b, err := json.MarshalIndent(statefulsets.Items, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		statefulsetsByNamespace[namespace+".json"] = b
	}

	return statefulsetsByNamespace, errorsByNamespace
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

		b, err := json.MarshalIndent(nsJobs.Items, "", "  ")
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

		b, err := json.MarshalIndent(nsCronJobs.Items, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		cronJobsByNamespace[namespace+".json"] = b
	}

	return cronJobsByNamespace, errorsByNamespace
}

func ingress(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	ingressByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		ingress, err := client.ExtensionsV1beta1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		b, err := json.MarshalIndent(ingress.Items, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		ingressByNamespace[namespace+".json"] = b
	}

	return ingressByNamespace, errorsByNamespace
}

func storageClasses(ctx context.Context, client *kubernetes.Clientset) ([]byte, []string) {
	storageClasses, err := client.StorageV1beta1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, []string{err.Error()}
	}

	b, err := json.MarshalIndent(storageClasses.Items, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}

	return b, nil
}

func crds(ctx context.Context, client *apiextensionsv1beta1clientset.ApiextensionsV1beta1Client) ([]byte, []string) {
	crds, err := client.CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, []string{err.Error()}
	}

	b, err := json.MarshalIndent(crds.Items, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}

	return b, nil
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

		b, err := json.MarshalIndent(limitRanges.Items, "", "  ")
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

	b, err := json.MarshalIndent(nodes.Items, "", "  ")
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

func authCanI(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubectl/cmd/auth/cani.go

	authListByNamespace := make(map[string][]byte)
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

		rules := convertToPolicyRule(response.Status)
		b, err := json.MarshalIndent(rules, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		authListByNamespace[namespace+".json"] = b
	}

	return authListByNamespace, errorsByNamespace
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

		b, err := json.MarshalIndent(events.Items, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		eventsByNamespace[namespace+".json"] = b
	}

	return eventsByNamespace, errorsByNamespace
}

// not exprted from: https://github.com/kubernetes/kubernetes/blob/master/pkg/kubectl/cmd/auth/cani.go#L339
func convertToPolicyRule(status authorizationv1.SubjectRulesReviewStatus) []rbacv1.PolicyRule {
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

	b, err := json.MarshalIndent(pv.Items, "", "  ")
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

		b, err := json.MarshalIndent(pvcs.Items, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		pvcsByNamespace[namespace+".json"] = b
	}

	return pvcsByNamespace, errorsByNamespace
}
