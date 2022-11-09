package analyzer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type providers struct {
	microk8s      bool
	dockerDesktop bool
	eks           bool
	gke           bool
	digitalOcean  bool
	openShift     bool
	tanzu         bool
	kurl          bool
	aks           bool
	ibm           bool
	minikube      bool
	rke2          bool
	k3s           bool
}

type Provider int

const (
	unknown       Provider = iota
	microk8s      Provider = iota
	dockerDesktop Provider = iota
	eks           Provider = iota
	gke           Provider = iota
	digitalOcean  Provider = iota
	openShift     Provider = iota
	tanzu         Provider = iota
	kurl          Provider = iota
	aks           Provider = iota
	ibm           Provider = iota
	minikube      Provider = iota
	rke2          Provider = iota
	k3s           Provider = iota
)

func CheckApiResourcesForProviders(foundProviders *providers, apiResources []*metav1.APIResourceList, provider string) string {
	for _, resource := range apiResources {
		if strings.HasPrefix(resource.GroupVersion, "apps.openshift.io/") {
			foundProviders.openShift = true
			return "openShift"
		}
		if strings.HasPrefix(resource.GroupVersion, "run.tanzu.vmware.com/") {
			foundProviders.tanzu = true
			return "tanzu"
		}
	}

	return provider
}

func ParseNodesForProviders(nodes []corev1.Node) (providers, string) {
	foundProviders := providers{}
	foundMaster := false
	stringProvider := ""

	for _, node := range nodes {
		for k, v := range node.ObjectMeta.Labels {

			if k == "kurl.sh/cluster" && v == "true" {
				foundProviders.kurl = true
				stringProvider = "kurl"
			} else if k == "microk8s.io/cluster" && v == "true" {
				foundProviders.microk8s = true
				stringProvider = "microk8s"
			}
			if k == "node-role.kubernetes.io/master" {
				foundMaster = true
			}
			if k == "node-role.kubernetes.io/control-plane" {
				foundMaster = true
			}
			if k == "kubernetes.azure.com/role" {
				foundProviders.aks = true
				stringProvider = "aks"
			}
			if k == "minikube.k8s.io/version" {
				foundProviders.minikube = true
				stringProvider = "minikube"
			}
			if k == "node.kubernetes.io/instance-type" && v == "k3s" {
				foundProviders.k3s = true
				stringProvider = "k3s"
			}
			if k == "beta.kubernetes.io/instance-type" && v == "k3s" {
				foundProviders.k3s = true
				stringProvider = "k3s"
			}
		}

		for k := range node.ObjectMeta.Annotations {
			if k == "rke2.io/node-args" {
				foundProviders.rke2 = true
				stringProvider = "rke2"
			}
		}

		if node.Status.NodeInfo.OSImage == "Docker Desktop" {
			foundProviders.dockerDesktop = true
			stringProvider = "dockerDesktop"
		}

		if strings.HasPrefix(node.Spec.ProviderID, "digitalocean:") {
			foundProviders.digitalOcean = true
			stringProvider = "digitalOcean"
		}
		if strings.HasPrefix(node.Spec.ProviderID, "aws:") {
			foundProviders.eks = true
			stringProvider = "eks"
		}
		if strings.HasPrefix(node.Spec.ProviderID, "gce:") {
			foundProviders.gke = true
			stringProvider = "gke"
		}
		if strings.HasPrefix(node.Spec.ProviderID, "ibm:") {
			foundProviders.ibm = true
			stringProvider = "ibm"
		}
	}

	if foundMaster {
		// eks does not have masters within the node list
		foundProviders.eks = false
		if stringProvider == "eks" {
			stringProvider = ""
		}
	}

	return foundProviders, stringProvider
}

func analyzeDistribution(analyzer *troubleshootv1beta2.Distribution, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	var unknownDistribution string
	collected, err := getCollectedFileContents("cluster-resources/nodes.json")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get contents of nodes.json")
	}

	var nodes corev1.NodeList
	if err := json.Unmarshal(collected, &nodes); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal node list")
	}

	foundProviders, _ := ParseNodesForProviders(nodes.Items)

	apiResourcesBytes, err := getCollectedFileContents("cluster-resources/resources.json")
	// if the file is not found, that is not a fatal error
	// troubleshoot 0.9.15 and earlier did not collect that file
	if err == nil {
		var apiResources []*metav1.APIResourceList
		if err := json.Unmarshal(apiResourcesBytes, &apiResources); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal api resource list")
		}
		_ = CheckApiResourcesForProviders(&foundProviders, apiResources, "")
	}

	title := analyzer.CheckName
	if title == "" {
		title = "Kubernetes Distribution"
	}
	result := &AnalyzeResult{
		Title:   title,
		IconKey: "kubernetes_distribution",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/distribution.svg?w=20&h=14",
	}

	// ordering is important for passthrough
	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}

			isMatch, err := compareDistributionConditionalToActual(outcome.Fail.When, foundProviders, &unknownDistribution)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare distribution conditional")
			}

			if isMatch {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}

			isMatch, err := compareDistributionConditionalToActual(outcome.Warn.When, foundProviders, &unknownDistribution)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare distribution conditional")
			}

			if isMatch {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}

			isMatch, err := compareDistributionConditionalToActual(outcome.Pass.When, foundProviders, &unknownDistribution)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare distribution conditional")
			}

			if isMatch {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}

		}
	}

	result.IsWarn = true
	if unknownDistribution != "" {
		result.Message = unknownDistribution
	} else {
		result.Message = "None of the conditionals were met"
	}

	return result, nil
}

func compareDistributionConditionalToActual(conditional string, actual providers, unknownDistribution *string) (bool, error) {
	parts := strings.Split(strings.TrimSpace(conditional), " ")

	// we can make this a lot more flexible
	if len(parts) == 1 {
		parts = []string{
			"=",
			parts[0],
		}
	}

	if len(parts) != 2 {
		return false, errors.New("unable to parse conditional")
	}

	normalizedName := mustNormalizeDistributionName(parts[1])

	if normalizedName == unknown {
		*unknownDistribution += fmt.Sprintf("- Unknown distribution: %s ", parts[1])
		return false, nil
	}

	isMatch := false
	switch normalizedName {
	case microk8s:
		isMatch = actual.microk8s
	case dockerDesktop:
		isMatch = actual.dockerDesktop
	case eks:
		isMatch = actual.eks
	case gke:
		isMatch = actual.gke
	case digitalOcean:
		isMatch = actual.digitalOcean
	case openShift:
		isMatch = actual.openShift
	case kurl:
		isMatch = actual.kurl
	case aks:
		isMatch = actual.aks
	case ibm:
		isMatch = actual.ibm
	case minikube:
		isMatch = actual.minikube
	case rke2:
		isMatch = actual.rke2
	case k3s:
		isMatch = actual.k3s
	}

	switch parts[0] {
	case "=", "==", "===":
		return isMatch, nil
	case "!=", "!==":
		return !isMatch, nil
	}

	return false, nil
}

func mustNormalizeDistributionName(raw string) Provider {
	switch strings.ReplaceAll(strings.TrimSpace(strings.ToLower(raw)), "-", "") {
	case "microk8s":
		return microk8s
	case "dockerdesktop", "docker desktop", "docker-desktop":
		return dockerDesktop
	case "eks":
		return eks
	case "gke":
		return gke
	case "digitalocean":
		return digitalOcean
	case "openshift":
		return openShift
	case "tanzu":
		return tanzu
	case "kurl":
		return kurl
	case "aks":
		return aks
	case "ibm", "ibmcloud", "ibm cloud":
		return ibm
	case "minikube":
		return minikube
	case "rke2":
		return rke2
	case "k3s":
		return k3s
	}

	return unknown
}
