package specs

import (
	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

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
		if req.Key() == constants.TroubleshootIOLabelKey || req.Key() == constants.TroubleshootSHLabelKey {
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
