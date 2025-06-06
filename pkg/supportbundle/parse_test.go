package supportbundle

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	types "github.com/replicatedhq/troubleshoot/pkg/supportbundle/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_getPodDetailsFromFiles(t *testing.T) {
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

func TestGetPodDetails(t *testing.T) {
	req := require.New(t)

	// Create a temporary directory for our test files
	testDir, err := os.MkdirTemp("", "troubleshoot-test")
	req.NoError(err)
	defer os.RemoveAll(testDir)

	// Set up test data
	podNamespace := "default"
	podName := "hello-27436131-pxhk9"

	// Define our test files
	podsFileContent := `[
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

	eventsFileContent := `[
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

	initContainerLogs := "some logs here"
	containerLogs := "Tue Mar 1 20:53:00 UTC 2022 Hello"

	// Create the directory structure for our test files
	podsFilePath := getPodsFilePath(podNamespace)
	eventsFilePath := getEventsFilePath(podNamespace)
	initContainerLogsPath := getContainerLogsFilePath(podNamespace, podName, "remove-lost-found")
	containerLogsPath := getContainerLogsFilePath(podNamespace, podName, "hello")

	// Create the target file
	archivePath := filepath.Join(testDir, "test-bundle.tar.gz")
	file, err := os.Create(archivePath)
	req.NoError(err)

	// Create a gzip writer
	gw := gzip.NewWriter(file)

	// Create a tar writer
	tw := tar.NewWriter(gw)

	err = tw.WriteHeader(&tar.Header{
		Name: podsFilePath,
		Mode: 0644,
		Size: int64(len(podsFileContent)),
	})
	req.NoError(err)
	_, err = tw.Write([]byte(podsFileContent))
	req.NoError(err)

	err = tw.WriteHeader(&tar.Header{
		Name: eventsFilePath,
		Mode: 0644,
		Size: int64(len(eventsFileContent)),
	})
	req.NoError(err)
	_, err = tw.Write([]byte(eventsFileContent))
	req.NoError(err)

	err = tw.WriteHeader(&tar.Header{
		Name: initContainerLogsPath,
		Mode: 0644,
		Size: int64(len(initContainerLogs)),
	})
	req.NoError(err)
	_, err = tw.Write([]byte(initContainerLogs))
	req.NoError(err)

	err = tw.WriteHeader(&tar.Header{
		Name: containerLogsPath,
		Mode: 0644,
		Size: int64(len(containerLogs)),
	})
	req.NoError(err)
	_, err = tw.Write([]byte(containerLogs))
	req.NoError(err)

	req.NoError(tw.Close())
	req.NoError(gw.Close())
	req.NoError(file.Close())

	// Call GetPodDetails and verify the results
	podDetails, err := GetPodDetails(archivePath, podNamespace, podName)
	req.NoError(err)
	req.NotNil(podDetails)

	// Verify pod definition
	req.Equal(podName, podDetails.PodDefinition.Name)
	req.Equal(podNamespace, podDetails.PodDefinition.Namespace)

	// Verify init containers
	req.Len(podDetails.PodDefinition.Spec.InitContainers, 1)
	req.Equal("remove-lost-found", podDetails.PodDefinition.Spec.InitContainers[0].Name)

	// Verify containers
	req.Len(podDetails.PodDefinition.Spec.Containers, 1)
	req.Equal("hello", podDetails.PodDefinition.Spec.Containers[0].Name)

	// Verify events
	req.Len(podDetails.PodEvents, 1)
	req.Equal(podName, podDetails.PodEvents[0].InvolvedObject.Name)

	// Verify pod containers
	req.Len(podDetails.PodContainers, 2)

	// One should be an init container
	var initContainerFound, regularContainerFound bool
	for _, container := range podDetails.PodContainers {
		if container.Name == "remove-lost-found" {
			req.True(container.IsInitContainer)
			req.Equal(getContainerLogsFilePath(podNamespace, podName, "remove-lost-found"), container.LogsFilePath)
			initContainerFound = true
		} else if container.Name == "hello" {
			req.False(container.IsInitContainer)
			req.Equal(getContainerLogsFilePath(podNamespace, podName, "hello"), container.LogsFilePath)
			regularContainerFound = true
		}
	}
	req.True(initContainerFound)
	req.True(regularContainerFound)
}
