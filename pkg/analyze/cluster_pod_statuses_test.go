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

			actual, err := clusterPodStatuses(&test.analyzer, getFiles)
			req.NoError(err)
			req.Equal(len(test.expectResult), len(actual))
			for _, a := range actual {
				assert.Contains(t, test.expectResult, a)
			}
		})
	}
}
