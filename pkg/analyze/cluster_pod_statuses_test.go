package analyzer

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func Test_ClusterPodStatuses(t *testing.T) {
	tests := []struct {
		name         string
		analyzer     troubleshootv1beta2.ClusterPodStatuses
		expectResult []*AnalyzeResult
		files        map[string][]byte
		eventFiles   map[string][]byte
	}{
		{
			name: "pass_when_all_pods_are_healthy_in_specific_namespace",
			analyzer: troubleshootv1beta2.ClusterPodStatuses{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "pass_when_all_pods_are_healthy_in_specific_namespace",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "!= Healthy",
							Message: "A Pod, {{ .Name }}, is unhealthy with a status of: {{ .Status.Reason }}. Restarting the pod may fix the issue.",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "",
							Message: "All Pods are OK.",
						},
					},
				},
				Namespaces: []string{"default"},
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  true,
					IsWarn:  false,
					IsFail:  false,
					Title:   "pass_when_all_pods_are_healthy_in_specific_namespace",
					Message: "All Pods are OK.",
					InvolvedObject: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default",
						Name:       "random-pod-75b66db9b9-nqhp8",
					},
				},
			},
			files: map[string][]byte{
				"cluster-resources/pods/default.json":           []byte(defaultPods),
				"cluster-resources/pods/other.json":             []byte(otherPods),
				"cluster-resources/pods/default-unhealthy.json": []byte(defaultPodsUnhealthy),
				"cluster-resources/pods/other-unhealthy.json":   []byte(otherPodsUnhealthy),
			},
		},
		{
			name: "pass_when_pods_are_healthy_in_all_namespaces",
			analyzer: troubleshootv1beta2.ClusterPodStatuses{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "pass_when_pods_are_healthy_in_all_namespaces",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "!= Healthy",
							Message: "A Pod, {{ .Name }}, is unhealthy with a status of: {{ .Status.Reason }}. Restarting the pod may fix the issue.",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "",
							Message: "All Pods are OK.",
						},
					},
				},
				Namespaces: []string{},
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  true,
					IsWarn:  false,
					IsFail:  false,
					Title:   "pass_when_pods_are_healthy_in_all_namespaces",
					Message: "All Pods are OK.",
					InvolvedObject: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default",
						Name:       "random-pod-75b66db9b9-nqhp8",
					},
				},
				{
					IsPass:  true,
					IsWarn:  false,
					IsFail:  false,
					Title:   "pass_when_pods_are_healthy_in_all_namespaces",
					Message: "All Pods are OK.",
					InvolvedObject: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "other",
						Name:       "other-pod-75b66db9b9-nqhp8",
					},
				},
			},
			files: map[string][]byte{
				"cluster-resources/pods/default.json": []byte(defaultPods),
				"cluster-resources/pods/other.json":   []byte(otherPods),
			},
		},
		{
			name: "fail_when_pods_are_unhealthy_in_specific_namespace",
			analyzer: troubleshootv1beta2.ClusterPodStatuses{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "fail_when_pods_are_unhealthy_in_specific_namespace",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "!= Healthy",
							Message: "A pod is unhealthy",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "",
							Message: "All Pods are OK.",
						},
					},
				},
				Namespaces: []string{"default-unhealthy"},
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "fail_when_pods_are_unhealthy_in_specific_namespace",
					Message: "A pod is unhealthy",
					InvolvedObject: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default-unhealthy",
						Name:       "random-pod-75b66db9b9-nqhp8",
					},
				},
			},
			files: map[string][]byte{
				"cluster-resources/pods/default.json":           []byte(defaultPods),
				"cluster-resources/pods/other.json":             []byte(otherPods),
				"cluster-resources/pods/default-unhealthy.json": []byte(defaultPodsUnhealthy),
				"cluster-resources/pods/other-unhealthy.json":   []byte(otherPodsUnhealthy),
			},
		},
		{
			name: "fail_when_pods_are_unhealthy_in_specific_namespace_using_double_ne",
			analyzer: troubleshootv1beta2.ClusterPodStatuses{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "fail_when_pods_are_unhealthy_in_specific_namespace_using_double_ne",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "!== Healthy",
							Message: "A pod is unhealthy",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "",
							Message: "All Pods are OK.",
						},
					},
				},
				Namespaces: []string{"default-unhealthy"},
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "fail_when_pods_are_unhealthy_in_specific_namespace_using_double_ne",
					Message: "A pod is unhealthy",
					InvolvedObject: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default-unhealthy",
						Name:       "random-pod-75b66db9b9-nqhp8",
					},
				},
			},
			files: map[string][]byte{
				"cluster-resources/pods/default.json":           []byte(defaultPods),
				"cluster-resources/pods/other.json":             []byte(otherPods),
				"cluster-resources/pods/default-unhealthy.json": []byte(defaultPodsUnhealthy),
				"cluster-resources/pods/other-unhealthy.json":   []byte(otherPodsUnhealthy),
			},
		},
		{
			name: "fail_when_pods_are_unhealthy_in_any_namespace",
			analyzer: troubleshootv1beta2.ClusterPodStatuses{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "fail_when_pods_are_unhealthy_in_all_namespaces",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "!= Healthy",
							Message: "A pod is unhealthy",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "",
							Message: "All Pods are OK.",
						},
					},
				},
				Namespaces: []string{},
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "fail_when_pods_are_unhealthy_in_all_namespaces",
					Message: "A pod is unhealthy",
					InvolvedObject: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default-unhealthy",
						Name:       "random-pod-75b66db9b9-nqhp8",
					},
				},
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "fail_when_pods_are_unhealthy_in_all_namespaces",
					Message: "A pod is unhealthy",
					InvolvedObject: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "other-unhealthy",
						Name:       "other-pod-75b66db9b9-nqhp8",
					},
				},
				{
					IsPass:  true,
					IsWarn:  false,
					IsFail:  false,
					Title:   "fail_when_pods_are_unhealthy_in_all_namespaces",
					Message: "All Pods are OK.",
					InvolvedObject: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default",
						Name:       "random-pod-75b66db9b9-nqhp8",
					},
				},
				{
					IsPass:  true,
					IsWarn:  false,
					IsFail:  false,
					Title:   "fail_when_pods_are_unhealthy_in_all_namespaces",
					Message: "All Pods are OK.",
					InvolvedObject: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "other",
						Name:       "other-pod-75b66db9b9-nqhp8",
					},
				},
			},
			files: map[string][]byte{
				"cluster-resources/pods/default.json":           []byte(defaultPods),
				"cluster-resources/pods/other.json":             []byte(otherPods),
				"cluster-resources/pods/default-unhealthy.json": []byte(defaultPodsUnhealthy),
				"cluster-resources/pods/other-unhealthy.json":   []byte(otherPodsUnhealthy),
			},
		},
		{
			name: "warn_when_pods_are_unhealthy_in_specific_namespace_using_double_equals",
			analyzer: troubleshootv1beta2.ClusterPodStatuses{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "warn_when_pods_are_unhealthy_in_specific_namespace_using_double_equals",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "== CrashLoopBackOff",
							Message: "A pod is unhealthy.",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "",
							Message: "All Pods are OK.",
						},
					},
				},
				Namespaces: []string{"default-unhealthy"},
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  true,
					IsFail:  false,
					Title:   "warn_when_pods_are_unhealthy_in_specific_namespace_using_double_equals",
					Message: "A pod is unhealthy.",
					InvolvedObject: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default-unhealthy",
						Name:       "random-pod-75b66db9b9-nqhp8",
					},
				},
			},
			files: map[string][]byte{
				"cluster-resources/pods/default.json":           []byte(defaultPods),
				"cluster-resources/pods/other.json":             []byte(otherPods),
				"cluster-resources/pods/default-unhealthy.json": []byte(defaultPodsUnhealthy),
				"cluster-resources/pods/other-unhealthy.json":   []byte(otherPodsUnhealthy),
			},
		},
		{
			name: "warn_when_pods_are_unhealthy_in_specific_namespace_using_triple_equals",
			analyzer: troubleshootv1beta2.ClusterPodStatuses{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "warn_when_pods_are_unhealthy_in_specific_namespace_using_triple_equals",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "=== CrashLoopBackOff",
							Message: "A pod is unhealthy.",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "",
							Message: "All Pods are OK.",
						},
					},
				},
				Namespaces: []string{"default-unhealthy"},
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  true,
					IsFail:  false,
					Title:   "warn_when_pods_are_unhealthy_in_specific_namespace_using_triple_equals",
					Message: "A pod is unhealthy.",
					InvolvedObject: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default-unhealthy",
						Name:       "random-pod-75b66db9b9-nqhp8",
					},
				},
			},
			files: map[string][]byte{
				"cluster-resources/pods/default.json":           []byte(defaultPods),
				"cluster-resources/pods/other.json":             []byte(otherPods),
				"cluster-resources/pods/default-unhealthy.json": []byte(defaultPodsUnhealthy),
				"cluster-resources/pods/other-unhealthy.json":   []byte(otherPodsUnhealthy),
			},
		},
		{
			name: "show_message_of_pending_pods_with_wrong_node_affinity",
			analyzer: troubleshootv1beta2.ClusterPodStatuses{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "show_message_of_pending_pods_with_wrong_node_affinity",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "!= Healthy",
							Message: "A Pod, {{ .Name }}, is unhealthy with a status of: {{ .Status.Reason }}. Message is: {{ .Status.Message }}",
						},
					},
				},
				Namespaces: []string{"message-pending-node-affinity"},
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  true,
					IsFail:  false,
					Title:   "show_message_of_pending_pods_with_wrong_node_affinity",
					Message: "A Pod, kotsadm-b6cb54c8f-zgzrn, is unhealthy with a status of: Pending. Message is: 0/1 nodes are available: 1 node(s) didn't match Pod's node affinity/selector. preemption: 0/1 nodes are available: 1 Preemption is not helpful for scheduling.",
					InvolvedObject: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "message-pending-node-affinity",
						Name:       "kotsadm-b6cb54c8f-zgzrn",
					},
				},
			},
			files: map[string][]byte{
				"cluster-resources/pods/message-pending-node-affinity.json": []byte(messagePendingNodeAffinity),
			},
		},
		{
			name: "show_message_of_container_creating_pod_with_failed_mount",
			analyzer: troubleshootv1beta2.ClusterPodStatuses{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "show_message_of_container_creating_pod_with_failed_mount",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "!= Healthy",
							Message: "A Pod, {{ .Name }}, is unhealthy with a status of: {{ .Status.Reason }}. Message is: {{ .Status.Message }}",
						},
					},
				},
				Namespaces: []string{"message-container-creating-failed-mount"},
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  true,
					IsFail:  false,
					Title:   "show_message_of_container_creating_pod_with_failed_mount",
					Message: "A Pod, troubleshoot-copyfromhost-4m79m-psdjm, is unhealthy with a status of: ContainerCreating. Message is: MountVolume.SetUp failed for volume \"host\" : hostPath type check failed: /var/lib/collectd is not a directory. Unable to attach or mount volumes: unmounted volumes=[host], unattached volumes=[host kube-api-access-xddvj]: timed out waiting for the condition",
					InvolvedObject: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "message-container-creating-failed-mount",
						Name:       "troubleshoot-copyfromhost-4m79m-psdjm",
					},
				},
			},
			files: map[string][]byte{
				"cluster-resources/pods/message-container-creating-failed-mount.json": []byte(messageContainerCreatingFailedMount),
			},
			eventFiles: map[string][]byte{
				"cluster-resources/events/message-container-creating-failed-mount.json": []byte(messageContainerCreatingFailedMountEvents),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			getFiles := func(n string, _ []string) (map[string][]byte, error) {
				if file, ok := test.files[n]; ok {
					return map[string][]byte{n: file}, nil
				}
				return test.files, nil
			}

			getEventFiles := func(n string, _ []string) (map[string][]byte, error) {
				if file, ok := test.eventFiles[n]; ok {
					return map[string][]byte{n: file}, nil
				}
				return test.files, nil
			}

			actual, err := clusterPodStatuses(&test.analyzer, getFiles, getEventFiles)
			req.NoError(err)
			req.Equal(len(test.expectResult), len(actual))
			for _, a := range actual {
				assert.Contains(t, test.expectResult, a)
			}
		})
	}
}
