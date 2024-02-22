package analyzer

import (
	"testing"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestAnalyzeEvent(t *testing.T) {
	tests := []struct {
		name         string
		analyzer     troubleshootv1beta2.EventAnalyze
		expectResult []AnalyzeResult
		files        map[string][]byte
		err          error
	}{
		{
			name: "reason is required",
			analyzer: troubleshootv1beta2.EventAnalyze{
				CollectorName: "event-collector-0",
				Kind:          "Pod",
				Namespace:     "default",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "true",
							Message: "test",
						},
					},
				},
			},
			err: errors.New("reason is required"),
			files: map[string][]byte{
				"cluster-resources/events/default.json": []byte(`
					{
						"kind": "EventList",
						"apiVersion": "v1",
						"metadata": {
						  "resourceVersion": "722"
						},
						"items": [
						  {
							"kind": "Event",
							"apiVersion": "v1",
							"metadata": {
							  "name": "nginx-rc",
							  "namespace": "default",
							  "creationTimestamp": "2022-01-01T00:00:00Z"
							},
							"involvedObject": {
							  "kind": "Pod",
							  "name": "nginx-rc-12345",
							  "namespace": "default"
							},
							"reason": "OOMKilled",
							"message": "The container was killed due to an out-of-memory condition.",
							"type": "Warning"
						  }
						]
					}
					`),
			},
		},
		{
			name: "fail when OOMKilled event is present",
			analyzer: troubleshootv1beta2.EventAnalyze{
				CollectorName: "event-collector-1",
				Kind:          "Pod",
				Namespace:     "default",
				Reason:        "OOMKilled",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "true",
							Message: "Detect OOMKilled event with {{ .InvolvedObject.Kind }}-{{ .InvolvedObject.Name }} with message {{ .Message }}",
						},
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "false",
							Message: "No OOMKilled event detected",
						},
					},
				},
			},
			expectResult: []AnalyzeResult{
				{
					Title:   "event-collector-1",
					IconKey: "kubernetes_event",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/kubernetes.svg?w=16&h=16",
					IsFail:  true,
					IsWarn:  false,
					IsPass:  false,
					Message: "Detect OOMKilled event with Pod-nginx-rc-12345 with message The container was killed due to an out-of-memory condition.",
				},
			},
			files: map[string][]byte{
				"cluster-resources/events/default.json": []byte(`
				{
					"kind": "EventList",
					"apiVersion": "v1",
					"metadata": {
					  "resourceVersion": "722"
					},
					"items": [
					  {
						"kind": "Event",
						"apiVersion": "v1",
						"metadata": {
						  "name": "nginx-rc",
						  "namespace": "default",
						  "creationTimestamp": "2022-01-01T00:00:00Z"
						},
						"involvedObject": {
						  "kind": "Pod",
						  "name": "nginx-rc-12345",
						  "namespace": "default"
						},
						"reason": "OOMKilled",
						"message": "The container was killed due to an out-of-memory condition.",
						"type": "Warning"
					  }
					]
				}
				`),
			},
		},
		{
			name: "pass when no FailedMount event is present",
			analyzer: troubleshootv1beta2.EventAnalyze{
				CollectorName: "event-collector-2",
				Kind:          "Pod",
				Namespace:     "default",
				Reason:        "FailedMount",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "false",
							Message: "No FailedMount event detected",
						},
					},
				},
			},
			expectResult: []AnalyzeResult{
				{
					Title:   "event-collector-2",
					IconKey: "kubernetes_event",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/kubernetes.svg?w=16&h=16",
					IsFail:  false,
					IsWarn:  false,
					IsPass:  true,
					Message: "No FailedMount event detected",
				},
			},
			files: map[string][]byte{
				"cluster-resources/events/default.json": []byte(`
				{
					"kind": "EventList",
					"apiVersion": "v1",
					"metadata": {
					  "resourceVersion": "722"
					},
					"items": [
					  {
						"kind": "Event",
						"apiVersion": "v1",
						"metadata": {
						  "name": "nginx-rc-1",
						  "namespace": "default",
						  "creationTimestamp": "2022-01-01T00:00:00Z"
						},
						"involvedObject": {
						  "kind": "Pod",
						  "name": "nginx-rc-12345",
						  "namespace": "default"
						},
						"reason": "Created",
						"message": "Created container",
						"type": "Normal"
					  },
					  {
						"kind": "Event",
						"apiVersion": "v1",
						"metadata": {
						  "name": "nginx-rc-2",
						  "namespace": "default",
						  "creationTimestamp": "2022-01-01T00:00:00Z"
						},
						"involvedObject": {
						  "kind": "Pod",
						  "name": "nginx-rc-67890",
						  "namespace": "default"
						},
						"reason": "Started",
						"message": "Started container",
						"type": "Normal"
					  }
					]
				  }
				`),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			getFile := func(n string) ([]byte, error) {
				if b, ok := test.files[n]; ok {
					return b, nil
				}
				return nil, errors.New("file not found")
			}

			findFiles := func(n string, _ []string) (map[string][]byte, error) {
				return nil, errors.New("method not implemented")
			}

			a := &AnalyzeEvent{
				analyzer: &test.analyzer,
			}
			actual, err := a.Analyze(getFile, findFiles)
			if test.err != nil {
				req.EqualError(err, test.err.Error())
				return
			}

			req.NoError(err)
			unPointered := []AnalyzeResult{}
			for _, v := range actual {
				unPointered = append(unPointered, *v)
			}
			req.ElementsMatch(test.expectResult, unPointered)
		})
	}
}

func TestAnalyzeEventResult(t *testing.T) {
	event := &corev1.Event{
		InvolvedObject: corev1.ObjectReference{
			Kind: "Pod",
			Name: "foo-pod",
		},
		Reason:  "Unhealthy",
		Message: "foo-message",
		Type:    "Warning",
	}

	outcomes := []*troubleshootv1beta2.Outcome{
		{
			Fail: &troubleshootv1beta2.SingleOutcome{
				When:    "true",
				Message: "No unhealthy pods allowed",
			},
			Pass: &troubleshootv1beta2.SingleOutcome{
				When:    "false",
				Message: "No unhealthy pod detected",
			},
		},
	}

	checkName := "Test Event"

	expectedResults := []*AnalyzeResult{
		{
			Title:   "Test Event",
			IconKey: "kubernetes_event",
			IconURI: "https://troubleshoot.sh/images/analyzer-icons/kubernetes.svg?w=16&h=16",
			IsFail:  true,
			IsWarn:  false,
			IsPass:  false,
			Message: "No unhealthy pods allowed",
		},
	}

	results, err := analyzeEventResult(event, outcomes, checkName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != len(expectedResults) {
		t.Fatalf("unexpected number of results, got %d, want %d", len(results), len(expectedResults))
	}

	for i, result := range results {
		expectedResult := expectedResults[i]

		if result.Title != expectedResult.Title {
			t.Errorf("unexpected title, got %s, want %s", result.Title, expectedResult.Title)
		}

		if result.IsFail != expectedResult.IsFail {
			t.Errorf("unexpected IsFail value, got %v, want %v", result.IsFail, expectedResult.IsFail)
		}

		if result.IsWarn != expectedResult.IsWarn {
			t.Errorf("unexpected IsWarn value, got %v, want %v", result.IsWarn, expectedResult.IsWarn)
		}

		if result.IsPass != expectedResult.IsPass {
			t.Errorf("unexpected IsPass value, got %v, want %v", result.IsPass, expectedResult.IsPass)
		}

		if result.Message != expectedResult.Message {
			t.Errorf("unexpected message, got %s, want %s", result.Message, expectedResult.Message)
		}
	}
}
