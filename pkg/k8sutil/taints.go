package k8sutil

import corev1 "k8s.io/api/core/v1"

// TaintExists checks if the given taint exists in list of taints. Returns true
// if exists false otherwise.
//
// Copied from k8s.io/kubernetes/pkg/util/taints so we don't have to import
// k8s.io/kubernetes.
func TaintExists(taints []corev1.Taint, taintToFind *corev1.Taint) bool {
	for _, taint := range taints {
		if taint.MatchTaint(taintToFind) {
			return true
		}
	}
	return false
}
