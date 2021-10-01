package k8sutil

import v1 "k8s.io/api/core/v1"

const UnreachableTaint = "node.kubernetes.io/unreachable"
const NotReadyTaint = "node.kubernetes.io/not-ready"
const UnschedulableTaint = "node.kubernetes.io/unschedulable"

func NodeIsReady(node v1.Node) bool {
	for _, taint := range node.Spec.Taints {
		switch taint.Key {
		case NotReadyTaint:
			return false
		case UnreachableTaint:
			return false
		case UnschedulableTaint:
			return false
		}
	}
	return true
}
