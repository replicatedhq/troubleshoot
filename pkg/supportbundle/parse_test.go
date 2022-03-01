package supportbundle

import (
	"fmt"
	"testing"

	types "github.com/replicatedhq/troubleshoot/pkg/supportbundle/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_getPodDetails(t *testing.T) {
	podNamespace := "default"
	podName := "hello-27436131-pxhk9"

	podsFileContentOldFormat := `[
		{
			"kind": "Pod",
			"apiVersion": "v1",
			"metadata": {
				"name": "hello-27436131-pxhk9",
				"namespace": "default"
			},
			"spec": {
				"initContainers": [
					{
						"name": "remove-lost-found"
					}
				],
				"containers": [
					{
						"name": "hello"
					}
				]
			}
		}
	]`

	podsFileContentNewFormat := fmt.Sprintf(`{
		"kind": "PodList",
		"apiVersion": "v1",
		"metadata": {
			"resourceVersion": "5389414"
		},
		"items": %s
	}`, podsFileContentOldFormat)

	eventsFileContentOldFormat := `[
		{
			"kind": "Event",
			"apiVersion": "v1",
			"metadata": {
				"name": "example-nginx.16d85cebe302a9b1",
				"namespace": "default"
			},
			"involvedObject": {
				"kind": "Deployment",
				"namespace": "default",
				"name": "example-nginx"
			}
		},
		{
			"kind": "Event",
			"apiVersion": "v1",
			"metadata": {
				"name": "hello-27436131-pxhk9.16d85cf27380b4fa",
				"namespace": "default"
			},
			"involvedObject": {
				"kind": "Pod",
				"namespace": "default",
				"name": "hello-27436131-pxhk9"
			}
		}
	]`

	eventsFileContentNewFormat := fmt.Sprintf(`{
		"kind": "EventList",
		"apiVersion": "v1",
		"metadata": {
			"resourceVersion": "5389515"
		},
		"items": %s
	}`, eventsFileContentOldFormat)

	removeLostAndFoundInitContainerLogs := `some logs here`
	helloContainerLogs := `Tue Mar 1 20:53:00 UTC 2022 Hello`

	tests := []struct {
		name   string
		files  map[string][]byte
		expect *types.PodDetails
	}{
		{
			name: "old support bundle format",
			files: map[string][]byte{
				getPodsFilePath(podNamespace):                                        []byte(podsFileContentOldFormat),
				getEventsFilePath(podNamespace):                                      []byte(eventsFileContentOldFormat),
				getContainerLogsFilePath(podNamespace, podName, "remove-lost-found"): []byte(removeLostAndFoundInitContainerLogs),
				getContainerLogsFilePath(podNamespace, podName, "hello"):             []byte(helloContainerLogs),
			},
			expect: &types.PodDetails{
				PodDefinition: corev1.Pod{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: podNamespace,
					},
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{
							{
								Name: "remove-lost-found",
							},
						},
						Containers: []corev1.Container{
							{
								Name: "hello",
							},
						},
					},
				},
				PodEvents: []corev1.Event{
					{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Event",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "hello-27436131-pxhk9.16d85cf27380b4fa",
							Namespace: podNamespace,
						},
						InvolvedObject: corev1.ObjectReference{
							Kind:      "Pod",
							Namespace: podNamespace,
							Name:      podName,
						},
					},
				},
				PodContainers: []types.PodContainer{
					{
						Name:            "remove-lost-found",
						LogsFilePath:    getContainerLogsFilePath(podNamespace, podName, "remove-lost-found"),
						IsInitContainer: true,
					},
					{
						Name:         "hello",
						LogsFilePath: getContainerLogsFilePath(podNamespace, podName, "hello"),
					},
				},
			},
		},
		{
			name: "new support bundle format",
			files: map[string][]byte{
				getPodsFilePath(podNamespace):                                        []byte(podsFileContentNewFormat),
				getEventsFilePath(podNamespace):                                      []byte(eventsFileContentNewFormat),
				getContainerLogsFilePath(podNamespace, podName, "remove-lost-found"): []byte(removeLostAndFoundInitContainerLogs),
				getContainerLogsFilePath(podNamespace, podName, "hello"):             []byte(helloContainerLogs),
			},
			expect: &types.PodDetails{
				PodDefinition: corev1.Pod{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: podNamespace,
					},
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{
							{
								Name: "remove-lost-found",
							},
						},
						Containers: []corev1.Container{
							{
								Name: "hello",
							},
						},
					},
				},
				PodEvents: []corev1.Event{
					{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Event",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "hello-27436131-pxhk9.16d85cf27380b4fa",
							Namespace: podNamespace,
						},
						InvolvedObject: corev1.ObjectReference{
							Kind:      "Pod",
							Namespace: podNamespace,
							Name:      podName,
						},
					},
				},
				PodContainers: []types.PodContainer{
					{
						Name:            "remove-lost-found",
						LogsFilePath:    getContainerLogsFilePath(podNamespace, podName, "remove-lost-found"),
						IsInitContainer: true,
					},
					{
						Name:         "hello",
						LogsFilePath: getContainerLogsFilePath(podNamespace, podName, "hello"),
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			actual, err := getPodDetailsFromFiles(test.files, podNamespace, podName)
			req.NoError(err)

			assert.Equal(t, test.expect, actual)
		})
	}
}
