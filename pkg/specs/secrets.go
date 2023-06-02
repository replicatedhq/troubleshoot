package specs

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

func LoadFromSecret(namespace string, secretName string, key string) ([]byte, error) {
	config, err := k8sutil.GetRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert create k8s client")
	}

	foundSecret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get secret")
	}

	spec, ok := foundSecret.Data[key]
	if !ok {
		return nil, errors.Errorf("spec not found in secret %s", secretName)
	}

	klog.V(1).InfoS("Loaded spec from secret", "name",
		foundSecret.Name, "namespace", foundSecret.Namespace, "data key", key,
	)
	return spec, nil
}

func LoadFromSecretMatchingLabel(client kubernetes.Interface, labelSelector string, namespace string, key string) ([]string, error) {
	var secretsMatchingKey []string

	secrets, err := client.CoreV1().Secrets(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, errors.Wrap(err, "failed to search for secrets in the cluster")
	}

	for _, secret := range secrets.Items {
		spec, ok := secret.Data[key]
		if !ok {
			continue
		}

		klog.V(1).InfoS("Loaded spec from secret", "name", secret.Name,
			"namespace", secret.Namespace, "data key", key, "label selector", labelSelector,
		)
		secretsMatchingKey = append(secretsMatchingKey, string(spec))
	}

	return secretsMatchingKey, nil
}

// SplitTroubleshootSecretLabelSelector splits a label selector into two selectors, if applicable:
// 1. troubleshoot.io/kind=support-bundle and non-troubleshoot (if contains) labels selector.
// 2. troubleshoot.sh/kind=support-bundle and non-troubleshoot (if contains) labels selector.
func SplitTroubleshootSecretLabelSelector(client kubernetes.Interface, labelSelector labels.Selector) ([]string, error) {

	klog.V(1).Infof("Split %q selector into troubleshoot and non-troubleshoot labels selector separately, if applicable", labelSelector.String())

	selectorRequirements, selectorSelectable := labelSelector.Requirements()
	if !selectorSelectable {
		return nil, errors.Errorf("Selector %q is not selectable", labelSelector.String())
	}

	var troubleshootReqs, otherReqs []labels.Requirement

	for _, req := range selectorRequirements {
		if req.Key() == TroubleshootIOLabelKey || req.Key() == TroubleshootSHLabelKey {
			troubleshootReqs = append(troubleshootReqs, req)
		} else {
			otherReqs = append(otherReqs, req)
		}
	}

	parsedSelectorStrings := make([]string, 0)
	// Combine each troubleshoot requirement with other requirements to form new selectors
	if len(troubleshootReqs) == 0 {
		return []string{labelSelector.String()}, nil
	}

	for _, tReq := range troubleshootReqs {
		reqs := append(otherReqs, tReq)
		newSelector := labels.NewSelector().Add(reqs...)
		parsedSelectorStrings = append(parsedSelectorStrings, newSelector.String())
	}

	return parsedSelectorStrings, nil
}
