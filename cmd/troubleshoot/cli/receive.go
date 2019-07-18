package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/mholt/archiver"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func receiveSupportBundle(collectorJobNamespace string, collectorJobName string) error {
	// poll until there are no more "running" collectors
	troubleshootClient, err := createTroubleshootK8sClient()
	if err != nil {
		return err
	}

	bundlePath, err := ioutil.TempDir("", "troubleshoot")
	if err != nil {
		return err
	}
	defer os.RemoveAll(bundlePath)

	receivedCollectors := []string{}
	for {
		job, err := troubleshootClient.CollectorJobs(collectorJobNamespace).Get(collectorJobName, metav1.GetOptions{})
		if err != nil && kuberneteserrors.IsNotFound(err) {
			// where did it go!
			return nil
		} else if err != nil {
			return err
		}

		for _, readyCollector := range job.Status.Successful {
			alreadyReceived := false
			for _, receivedCollector := range receivedCollectors {
				if receivedCollector == readyCollector {
					alreadyReceived = true
				}
			}

			if alreadyReceived {
				continue
			}

			collectorResp, err := http.Get(fmt.Sprintf("http://localhost:8000/collector/%s", readyCollector))
			if err != nil {
				return err
			}

			defer collectorResp.Body.Close()
			body, err := ioutil.ReadAll(collectorResp.Body)
			if err != nil {
				return err
			}

			decoded, err := base64.StdEncoding.DecodeString(string(body))
			if err != nil {
				fmt.Printf("failed to output for collector %s\n", readyCollector)
				continue
			}

			files := make(map[string]interface{})
			if err := json.Unmarshal(decoded, &files); err != nil {
				fmt.Printf("failed to unmarshal output for collector %s\n", readyCollector)
			}

			for filename, maybeContents := range files {
				fileDir, fileName := filepath.Split(filename)
				outPath := filepath.Join(bundlePath, fileDir)

				if err := os.MkdirAll(outPath, 0777); err != nil {
					return err
				}

				switch maybeContents.(type) {
				case string:
					decoded, err := base64.StdEncoding.DecodeString(maybeContents.(string))
					if err != nil {
						return err
					}
					if err := writeFile(filepath.Join(outPath, fileName), decoded); err != nil {
						return err
					}

				case map[string]interface{}:
					for k, v := range maybeContents.(map[string]interface{}) {
						s, _ := filepath.Split(filepath.Join(outPath, fileName, k))
						if err := os.MkdirAll(s, 0777); err != nil {
							return err
						}

						decoded, err := base64.StdEncoding.DecodeString(v.(string))
						if err != nil {
							return err
						}
						if err := writeFile(filepath.Join(outPath, fileName, k), decoded); err != nil {
							return err
						}
					}
				}
			}

			receivedCollectors = append(receivedCollectors, readyCollector)
		}

		if len(job.Status.Running) == 0 {
			tarGz := archiver.TarGz{
				Tar: &archiver.Tar{
					ImplicitTopLevelFolder: false,
				},
			}

			paths := make([]string, 0, 0)
			for _, id := range receivedCollectors {
				paths = append(paths, filepath.Join(bundlePath, id))
			}

			if err := tarGz.Archive(paths, "support-bundle.tar.gz"); err != nil {
				return err
			}
			return nil
		}
	}
}

func writeFile(filename string, contents []byte) error {
	if err := ioutil.WriteFile(filename, contents, 0644); err != nil {
		return err
	}

	return nil
}
