package collect

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/replicatedhq/troubleshoot/pkg/redact"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1beta1clientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type ClusterResourcesOutput struct {
	Namespaces                      []byte            `json:"cluster-resources/namespaces.json,omitempty"`
	NamespacesErrors                []byte            `json:"cluster-resources/namespaces-errors.json,omitempty"`
	Pods                            map[string][]byte `json:"cluster-resources/pods,omitempty"`
	PodsErrors                      []byte            `json:"cluster-resources/pods-errors.json,omitempty"`
	Services                        map[string][]byte `json:"cluster-resources/services,omitempty"`
	ServicesErrors                  []byte            `json:"cluster-resources/services-errors.json,omitempty"`
	Deployments                     map[string][]byte `json:"cluster-resources/deployments,omitempty"`
	DeploymentsErrors               []byte            `json:"cluster-resources/deployments-errors.json,omitempty"`
	Ingress                         map[string][]byte `json:"cluster-resources/ingress,omitempty"`
	IngressErrors                   []byte            `json:"cluster-resources/ingress-errors.json,omitempty"`
	StorageClasses                  []byte            `json:"cluster-resources/storage-classes.json,omitempty"`
	StorageErrors                   []byte            `json:"cluster-resources/storage-errors.json,omitempty"`
	CustomResourceDefinitions       []byte            `json:"cluster-resources/custom-resource-definitions.json,omitempty"`
	CustomResourceDefinitionsErrors []byte            `json:"cluster-resources/custom-resource-definitions-errors.json,omitempty"`
	ImagePullSecrets                map[string][]byte `json:"cluster-resources/image-pull-secrets,omitempty"`
	ImagePullSecretsErrors          []byte            `json:"cluster-resources/image-pull-secrets-errors.json,omitempty"`
	Nodes                           []byte            `json:"cluster-resources/nodes.json,omitempty"`
	NodesErrors                     []byte            `json:"cluster-resources/nodes-errors.json,omitempty"`
	Groups                          []byte            `json:"cluster-resources/groups.json,omitempty"`
	Resources                       []byte            `json:"cluster-resources/resources.json,omitempty"`
	GroupsResourcesErrors           []byte            `json:"cluster-resources/groups-resources-errors.json,omitempty"`
	LimitRanges                     map[string][]byte `json:"cluster-resources/limitranges,omitempty"`
	LimitRangesErrors               []byte            `json:"cluster-ressources/limitranges-errors.json,omitempty"`

	// TODO these should be considered for relocation to an rbac or auth package.  cluster resources might not be the right place
	AuthCanI       map[string][]byte `json:"cluster-resources/auth-cani-list,omitempty"`
	AuthCanIErrors []byte            `json:"cluster-resources/auth-cani-list-errors.json,omitempty"`
}

func ClusterResources(ctx *Context) ([]byte, error) {
	client, err := kubernetes.NewForConfig(ctx.ClientConfig)
	if err != nil {
		return nil, err
	}

	clusterResourcesOutput := &ClusterResourcesOutput{}
	// namespaces
	var namespaceNames []string
	if ctx.Namespace == "" {
		namespaces, namespaceList, namespaceErrors := namespaces(client)
		clusterResourcesOutput.Namespaces = namespaces
		clusterResourcesOutput.NamespacesErrors, err = marshalNonNil(namespaceErrors)
		if err != nil {
			return nil, err
		}
		if namespaceList != nil {
			for _, namespace := range namespaceList.Items {
				namespaceNames = append(namespaceNames, namespace.Name)
			}
		}
	} else {
		namespaces, namespaceErrors := getNamespace(client, ctx.Namespace)
		clusterResourcesOutput.Namespaces = namespaces
		clusterResourcesOutput.NamespacesErrors, err = marshalNonNil(namespaceErrors)
		if err != nil {
			return nil, err
		}
		namespaceNames = append(namespaceNames, ctx.Namespace)
	}
	pods, podErrors := pods(client, namespaceNames)
	clusterResourcesOutput.Pods = pods
	clusterResourcesOutput.PodsErrors, err = marshalNonNil(podErrors)
	if err != nil {
		return nil, err
	}

	// services
	services, servicesErrors := services(client, namespaceNames)
	clusterResourcesOutput.Services = services
	clusterResourcesOutput.ServicesErrors, err = marshalNonNil(servicesErrors)
	if err != nil {
		return nil, err
	}

	// deployments
	deployments, deploymentsErrors := deployments(client, namespaceNames)
	clusterResourcesOutput.Deployments = deployments
	clusterResourcesOutput.DeploymentsErrors, err = marshalNonNil(deploymentsErrors)
	if err != nil {
		return nil, err
	}

	// ingress
	ingress, ingressErrors := ingress(client, namespaceNames)
	clusterResourcesOutput.Ingress = ingress
	clusterResourcesOutput.IngressErrors, err = marshalNonNil(ingressErrors)
	if err != nil {
		return nil, err
	}

	// storage classes
	storageClasses, storageErrors := storageClasses(client)
	clusterResourcesOutput.StorageClasses = storageClasses
	clusterResourcesOutput.StorageErrors, err = marshalNonNil(storageErrors)
	if err != nil {
		return nil, err
	}

	// crds
	crdClient, err := apiextensionsv1beta1clientset.NewForConfig(ctx.ClientConfig)
	if err != nil {
		return nil, err
	}
	customResourceDefinitions, crdErrors := crds(crdClient)
	clusterResourcesOutput.CustomResourceDefinitions = customResourceDefinitions
	clusterResourcesOutput.CustomResourceDefinitionsErrors, err = marshalNonNil(crdErrors)
	if err != nil {
		return nil, err
	}

	// imagepullsecrets
	imagePullSecrets, pullSecretsErrors := imagePullSecrets(client, namespaceNames)
	clusterResourcesOutput.ImagePullSecrets = imagePullSecrets
	clusterResourcesOutput.ImagePullSecretsErrors, err = marshalNonNil(pullSecretsErrors)
	if err != nil {
		return nil, err
	}

	// nodes
	nodes, nodeErrors := nodes(client)
	clusterResourcesOutput.Nodes = nodes
	clusterResourcesOutput.NodesErrors, err = marshalNonNil(nodeErrors)
	if err != nil {
		return nil, err
	}

	groups, resources, groupsResourcesErrors := apiResources(client)
	clusterResourcesOutput.Groups = groups
	clusterResourcesOutput.Resources = resources
	clusterResourcesOutput.GroupsResourcesErrors, err = marshalNonNil(groupsResourcesErrors)
	if err != nil {
		return nil, err
	}

	// limit ranges
	limitRanges, limitRangesErrors := limitRanges(client, namespaceNames)
	clusterResourcesOutput.LimitRanges = limitRanges
	clusterResourcesOutput.LimitRangesErrors, err = marshalNonNil(limitRangesErrors)
	if err != nil {
		return nil, err
	}

	// auth cani
	authCanI, authCanIErrors := authCanI(client, namespaceNames)
	clusterResourcesOutput.AuthCanI = authCanI
	clusterResourcesOutput.AuthCanIErrors, err = marshalNonNil(authCanIErrors)
	if err != nil {
		return nil, err
	}

	if ctx.Redact {
		clusterResourcesOutput, err = clusterResourcesOutput.Redact()
		if err != nil {
			return nil, err
		}
	}

	b, err := json.MarshalIndent(clusterResourcesOutput, "", "  ")
	if err != nil {
		return nil, err
	}

	return b, nil
}

func namespaces(client *kubernetes.Clientset) ([]byte, *corev1.NamespaceList, []string) {
	namespaces, err := client.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return nil, nil, []string{err.Error()}
	}

	b, err := json.MarshalIndent(namespaces.Items, "", "  ")
	if err != nil {
		return nil, nil, []string{err.Error()}
	}

	return b, namespaces, nil
}

func getNamespace(client *kubernetes.Clientset, namespace string) ([]byte, []string) {
	namespaces, err := client.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	if err != nil {
		return nil, []string{err.Error()}
	}

	b, err := json.MarshalIndent(namespaces, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}

	return b, nil
}

func pods(client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	podsByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		pods, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{})
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

func services(client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	servicesByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		services, err := client.CoreV1().Services(namespace).List(metav1.ListOptions{})
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

func deployments(client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	deploymentsByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		deployments, err := client.AppsV1().Deployments(namespace).List(metav1.ListOptions{})
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

func ingress(client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	ingressByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		ingress, err := client.ExtensionsV1beta1().Ingresses(namespace).List(metav1.ListOptions{})
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

func storageClasses(client *kubernetes.Clientset) ([]byte, []string) {
	storageClasses, err := client.StorageV1beta1().StorageClasses().List(metav1.ListOptions{})
	if err != nil {
		return nil, []string{err.Error()}
	}

	b, err := json.MarshalIndent(storageClasses.Items, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}

	return b, nil
}

func crds(client *apiextensionsv1beta1clientset.ApiextensionsV1beta1Client) ([]byte, []string) {
	crds, err := client.CustomResourceDefinitions().List(metav1.ListOptions{})
	if err != nil {
		return nil, []string{err.Error()}
	}

	b, err := json.MarshalIndent(crds.Items, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}

	return b, nil
}

func imagePullSecrets(client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
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
		secrets, err := client.CoreV1().Secrets(namespace).List(metav1.ListOptions{})
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

func limitRanges(client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	limitRangesByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		limitRanges, err := client.CoreV1().LimitRanges(namespace).List(metav1.ListOptions{})
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

func nodes(client *kubernetes.Clientset) ([]byte, []string) {
	nodes, err := client.CoreV1().Nodes().List(metav1.ListOptions{})
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
func apiResources(client *kubernetes.Clientset) ([]byte, []byte, []string) {
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

func authCanI(client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubectl/cmd/auth/cani.go

	authListByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		sar := &authorizationv1.SelfSubjectRulesReview{
			Spec: authorizationv1.SelfSubjectRulesReviewSpec{
				Namespace: namespace,
			},
		}
		response, err := client.AuthorizationV1().SelfSubjectRulesReviews().Create(sar)
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

func (c *ClusterResourcesOutput) Redact() (*ClusterResourcesOutput, error) {
	namespaces, err := redact.Redact(c.Namespaces)
	if err != nil {
		return nil, err
	}
	nodes, err := redact.Redact(c.Nodes)
	if err != nil {
		return nil, err
	}
	pods, err := redactMap(c.Pods)
	if err != nil {
		return nil, err
	}
	services, err := redactMap(c.Services)
	if err != nil {
		return nil, err
	}
	deployments, err := redactMap(c.Deployments)
	if err != nil {
		return nil, err
	}
	ingress, err := redactMap(c.Ingress)
	if err != nil {
		return nil, err
	}
	storageClasses, err := redact.Redact(c.StorageClasses)
	if err != nil {
		return nil, err
	}
	crds, err := redact.Redact(c.CustomResourceDefinitions)
	if err != nil {
		return nil, err
	}
	groups, err := redact.Redact(c.Groups)
	if err != nil {
		return nil, err
	}
	resources, err := redact.Redact(c.Resources)
	if err != nil {
		return nil, err
	}

	return &ClusterResourcesOutput{
		Namespaces:                      namespaces,
		NamespacesErrors:                c.NamespacesErrors,
		Nodes:                           nodes,
		NodesErrors:                     c.NodesErrors,
		Pods:                            pods,
		PodsErrors:                      c.PodsErrors,
		Services:                        services,
		ServicesErrors:                  c.ServicesErrors,
		Deployments:                     deployments,
		DeploymentsErrors:               c.DeploymentsErrors,
		Ingress:                         ingress,
		IngressErrors:                   c.IngressErrors,
		StorageClasses:                  storageClasses,
		StorageErrors:                   c.StorageErrors,
		CustomResourceDefinitions:       crds,
		CustomResourceDefinitionsErrors: c.CustomResourceDefinitionsErrors,
		ImagePullSecrets:                c.ImagePullSecrets,
		ImagePullSecretsErrors:          c.ImagePullSecretsErrors,
		AuthCanI:                        c.AuthCanI,
		AuthCanIErrors:                  c.AuthCanIErrors,
		Groups:                          groups,
		Resources:                       resources,
		GroupsResourcesErrors:           c.GroupsResourcesErrors,
		LimitRanges:                     c.LimitRanges,
		LimitRangesErrors:               c.LimitRangesErrors,
	}, nil
}
