package collect

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1beta1clientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type ClusterResourcesOutput struct {
	Namespaces                []byte            `json:"cluster-resources/namespaces.json,omitempty"`
	Pods                      map[string][]byte `json:"cluster-resources/pods,omitempty"`
	Services                  map[string][]byte `json:"cluster-resources/services,omitempty"`
	Deployments               map[string][]byte `json:"cluster-resources/deployments,omitempty"`
	Ingress                   map[string][]byte `json:"cluster-resources/ingress,omitempty"`
	StorageClasses            []byte            `json:"cluster-resources/storage-classes.json,omitempty"`
	CustomResourceDefinitions []byte            `json:"cluster-resources/custom-resource-definitions.json,omitempty"`
}

func ClusterResources() error {
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	clusterResourcesOutput := ClusterResourcesOutput{}

	// namespaces
	namespaces, namespaceList, err := namespaces(client)
	if err != nil {
		return err
	}
	clusterResourcesOutput.Namespaces = namespaces

	namespaceNames := make([]string, 0, 0)
	for _, namespace := range namespaceList.Items {
		namespaceNames = append(namespaceNames, namespace.Name)
	}

	pods, err := pods(client, namespaceNames)
	if err != nil {
		return err
	}
	clusterResourcesOutput.Pods = pods

	// services
	services, err := services(client, namespaceNames)
	if err != nil {
		return err
	}
	clusterResourcesOutput.Services = services

	// deployments
	deployments, err := deployments(client, namespaceNames)
	if err != nil {
		return err
	}
	clusterResourcesOutput.Deployments = deployments

	// ingress
	ingress, err := ingress(client, namespaceNames)
	if err != nil {
		return err
	}
	clusterResourcesOutput.Ingress = ingress

	// storage classes
	storageClasses, err := storageClasses(client)
	if err != nil {
		return err
	}
	clusterResourcesOutput.StorageClasses = storageClasses

	// crds
	crdClient, err := apiextensionsv1beta1clientset.NewForConfig(cfg)
	if err != nil {
		return err
	}
	customResourceDefinitions, err := crds(crdClient)
	if err != nil {
		return err
	}
	clusterResourcesOutput.CustomResourceDefinitions = customResourceDefinitions

	b, err := json.MarshalIndent(clusterResourcesOutput, "", "  ")
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", b)

	return nil
}

func namespaces(client *kubernetes.Clientset) ([]byte, *corev1.NamespaceList, error) {
	namespaces, err := client.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}

	b, err := json.MarshalIndent(namespaces.Items, "", "  ")
	if err != nil {
		return nil, nil, err
	}

	return b, namespaces, nil
}

func pods(client *kubernetes.Clientset, namespaces []string) (map[string][]byte, error) {
	podsByNamespace := make(map[string][]byte)

	for _, namespace := range namespaces {
		pods, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		b, err := json.MarshalIndent(pods.Items, "", "  ")
		if err != nil {
			return nil, err
		}

		podsByNamespace[namespace+".json"] = b
	}

	return podsByNamespace, nil
}

func services(client *kubernetes.Clientset, namespaces []string) (map[string][]byte, error) {
	servicesByNamespace := make(map[string][]byte)

	for _, namespace := range namespaces {
		services, err := client.CoreV1().Services(namespace).List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		b, err := json.MarshalIndent(services.Items, "", "  ")
		if err != nil {
			return nil, err
		}

		servicesByNamespace[namespace+".json"] = b
	}

	return servicesByNamespace, nil
}

func deployments(client *kubernetes.Clientset, namespaces []string) (map[string][]byte, error) {
	deploymentsByNamespace := make(map[string][]byte)

	for _, namespace := range namespaces {
		deployments, err := client.AppsV1().Deployments(namespace).List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		b, err := json.MarshalIndent(deployments.Items, "", "  ")
		if err != nil {
			return nil, err
		}

		deploymentsByNamespace[namespace+".json"] = b
	}

	return deploymentsByNamespace, nil
}

func ingress(client *kubernetes.Clientset, namespaces []string) (map[string][]byte, error) {
	ingressByNamespace := make(map[string][]byte)

	for _, namespace := range namespaces {
		ingress, err := client.ExtensionsV1beta1().Ingresses(namespace).List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		b, err := json.MarshalIndent(ingress.Items, "", "  ")
		if err != nil {
			return nil, err
		}

		ingressByNamespace[namespace+".json"] = b
	}

	return ingressByNamespace, nil
}

func storageClasses(client *kubernetes.Clientset) ([]byte, error) {
	storageClasses, err := client.StorageV1beta1().StorageClasses().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	b, err := json.MarshalIndent(storageClasses.Items, "", "  ")
	if err != nil {
		return nil, err
	}

	return b, nil
}

func crds(client *apiextensionsv1beta1clientset.ApiextensionsV1beta1Client) ([]byte, error) {
	crds, err := client.CustomResourceDefinitions().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	b, err := json.MarshalIndent(crds.Items, "", "  ")
	if err != nil {
		return nil, err
	}

	return b, nil
}
