package supportbundle

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	redact2 "github.com/replicatedhq/troubleshoot/pkg/redact"
	corev1 "k8s.io/api/core/v1"
)

type PodDetails struct {
	PodDefinition corev1.Pod     `json:"podDefinition"`
	PodEvents     []corev1.Event `json:"podEvents"`
	PodContainers []PodContainer `json:"podContainers"`
}

type PodContainer struct {
	Name            string `json:"name"`
	LogsFilePath    string `json:"logsFilePath"`
	IsInitContainer bool   `json:"isInitContainer"`
}

func GetPodDetails(bundleArchivePath string, podNamespace string, podName string) (*PodDetails, error) {
	podDetails := PodDetails{}

	nsPodsFilePath := filepath.Join("cluster-resources", "pods", fmt.Sprintf("%s.json", podNamespace))
	nsEventsFilePath := filepath.Join("cluster-resources", "events", fmt.Sprintf("%s.json", podNamespace))

	files, err := GetFilesContents(bundleArchivePath, []string{nsPodsFilePath, nsEventsFilePath})
	if err != nil {
		logger.Error(errors.Wrap(err, "failed to get files contents"))
		getPodDetailsFromSupportBundleResponse.Error = fmt.Sprintf("failed to get file %s", nsPodsFilePath)
		JSON(w, 500, getPodDetailsFromSupportBundleResponse)
		return
	}

	var nsEvents []corev1.Event
	if err := json.Unmarshal(files[nsEventsFilePath], &nsEvents); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal events")
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
		return nil, errors.Wrap(err, "failed to unmarshal pods")
	}
	for _, pod := range podsArr {
		if pod.Name == podName && pod.Namespace == podNamespace {
			podDetails.PodDefinition = pod
			podDetails.PodContainers = []PodContainer{}
			for _, i := range pod.Spec.InitContainers {
				podDetails.PodContainers = append(podDetails.PodContainers, PodContainer{
					Name:            i.Name,
					LogsFilePath:    filepath.Join("cluster-resources", "pods", "logs", pod.Namespace, pod.Name, fmt.Sprintf("%s.log", i.Name)),
					IsInitContainer: true,
				})
			}
			for _, c := range pod.Spec.Containers {
				podDetails.PodContainers = append(podDetails.PodContainers, PodContainer{
					Name:            c.Name,
					LogsFilePath:    filepath.Join("cluster-resources", "pods", "logs", pod.Namespace, pod.Name, fmt.Sprintf("%s.log", c.Name)),
					IsInitContainer: false,
				})
			}
			break
		}
	}

	return podDetails, nil
}

// GetFilesContents will return the file contents for filenames matching the filenames
// parameter.
func GetFilesContents(bundleArchivePath string, filenames []string) (map[string][]byte, error) {
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
