package supportbundle

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mholt/archiver/v3"
	"github.com/pkg/errors"
	types "github.com/replicatedhq/troubleshoot/pkg/supportbundle/types"
	corev1 "k8s.io/api/core/v1"
)

var (
	SupportBundleNameRegex = regexp.MustCompile(`^\/?support-bundle-(\d{4})-(\d{2})-(\d{2})T(\d{2})_(\d{2})_(\d{2})\/?`)
)

func getPodsFilePath(namespace string) string {
	return filepath.Join("cluster-resources", "pods", fmt.Sprintf("%s.json", namespace))
}

func getContainerLogsFilePath(namespace string, podName string, containerName string) string {
	return filepath.Join("cluster-resources", "pods", "logs", namespace, podName, fmt.Sprintf("%s.log", containerName))
}

func getEventsFilePath(namespace string) string {
	return filepath.Join("cluster-resources", "events", fmt.Sprintf("%s.json", namespace))
}

func GetPodDetails(bundleArchive string, podNamespace string, podName string) (*types.PodDetails, error) {
	nsPodsFilePath := getPodsFilePath(podNamespace)
	nsEventsFilePath := getEventsFilePath(podNamespace)

	files, err := GetFilesContents(bundleArchive, []string{nsPodsFilePath, nsEventsFilePath})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get files contents")
	}

	return getPodDetailsFromFiles(files, podNamespace, podName)
}

func getPodDetailsFromFiles(files map[string][]byte, podNamespace string, podName string) (*types.PodDetails, error) {
	podDetails := types.PodDetails{}

	nsPodsFilePath := getPodsFilePath(podNamespace)
	nsEventsFilePath := getEventsFilePath(podNamespace)

	var nsEvents []corev1.Event
	if err := json.Unmarshal(files[nsEventsFilePath], &nsEvents); err != nil {
		// try new format
		var nsEventsList corev1.EventList
		if err := json.Unmarshal(files[nsEventsFilePath], &nsEventsList); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal events")
		}
		nsEvents = nsEventsList.Items
	}
	podEvents := []corev1.Event{}
	for _, event := range nsEvents {
		if event.InvolvedObject.Kind == "Pod" && event.InvolvedObject.Name == podName && event.InvolvedObject.Namespace == podNamespace {
			podEvents = append(podEvents, event)
		}
	}
	podDetails.PodEvents = podEvents

	var podsArr []corev1.Pod
	if err := json.Unmarshal(files[nsPodsFilePath], &podsArr); err != nil {
		// try new format
		var podsList corev1.PodList
		if err := json.Unmarshal(files[nsPodsFilePath], &podsList); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal pods")
		}
		podsArr = podsList.Items
	}
	for _, pod := range podsArr {
		if pod.Name == podName && pod.Namespace == podNamespace {
			podDetails.PodDefinition = pod
			podDetails.PodContainers = []types.PodContainer{}
			for _, i := range pod.Spec.InitContainers {
				podDetails.PodContainers = append(podDetails.PodContainers, types.PodContainer{
					Name:            i.Name,
					LogsFilePath:    getContainerLogsFilePath(pod.Namespace, pod.Name, i.Name),
					IsInitContainer: true,
				})
			}
			for _, c := range pod.Spec.Containers {
				podDetails.PodContainers = append(podDetails.PodContainers, types.PodContainer{
					Name:            c.Name,
					LogsFilePath:    getContainerLogsFilePath(pod.Namespace, pod.Name, c.Name),
					IsInitContainer: false,
				})
			}
			break
		}
	}

	return &podDetails, nil
}

// GetFilesContents will return the file contents for filenames matching the filenames parameter.
func GetFilesContents(bundleArchive string, filenames []string) (map[string][]byte, error) {
	bundleDir, err := ioutil.TempDir("", "troubleshoot")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create tmp dir")
	}
	defer os.RemoveAll(bundleDir)

	tarGz := archiver.TarGz{
		Tar: &archiver.Tar{
			ImplicitTopLevelFolder: false,
		},
	}
	if err := tarGz.Unarchive(bundleArchive, bundleDir); err != nil {
		return nil, errors.Wrap(err, "failed to unarchive")
	}

	files := map[string][]byte{}
	err = filepath.Walk(bundleDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if len(path) <= len(bundleDir) {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		// the following tries to find the actual file path of the desired files in the support bundle
		// this is needed to handle old and new support bundle formats
		// where old support bundles don't include a top level subdirectory and the new ones do
		// this basically compares file paths after trimming the subdirectory path from both (if exists)
		// for example: "support-bundle-2021-09-10T18_50_35/support-bundle-2021-09-10T18_50_35/path/to/file"
		relPath, err := filepath.Rel(bundleDir, path) // becomes: "support-bundle-2021-09-10T18_50_35/path/to/file"
		if err != nil {
			return errors.Wrap(err, "failed to get relative path")
		}

		trimmedRelPath := SupportBundleNameRegex.ReplaceAllString(relPath, "")        // becomes: "path/to/file"
		trimmedRelPath = strings.TrimPrefix(trimmedRelPath, string(os.PathSeparator)) // extra measure to ensure no leading slashes. for example: "/path/to/file"
		if trimmedRelPath == "" {
			return nil
		}

		for _, filename := range filenames {
			trimmedFileName := SupportBundleNameRegex.ReplaceAllString(filename, "")
			trimmedFileName = strings.TrimPrefix(trimmedFileName, string(os.PathSeparator))
			if trimmedFileName == "" {
				continue
			}
			if trimmedRelPath == trimmedFileName {
				content, err := ioutil.ReadFile(path)
				if err != nil {
					return errors.Wrap(err, "failed to read file")
				}

				files[filename] = content
				return nil
			}
		}

		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to walk")
	}

	return files, nil
}
