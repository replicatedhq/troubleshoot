package cli

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/replicatedhq/troubleshoot/pkg/logger"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func receivePreflightResults(preflightJobNamespace string, preflightJobName string) error {
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

	receivedPreflights := []string{}
	for {
		job, err := troubleshootClient.PreflightJobs(preflightJobNamespace).Get(preflightJobName, metav1.GetOptions{})
		if err != nil && kuberneteserrors.IsNotFound(err) {
			// where did it go!
			return nil
		} else if err != nil {
			return err
		}

		for _, readyPreflight := range job.Status.AnalyzersSuccessful {
			alreadyReceived := false
			for _, receivedPreflight := range receivedPreflights {
				if receivedPreflight == readyPreflight {
					alreadyReceived = true
				}
			}

			if alreadyReceived {
				continue
			}

			preflightResp, err := http.Get(fmt.Sprintf("http://localhost:8000/preflight/%s", readyPreflight))
			if err != nil {
				return err
			}

			defer preflightResp.Body.Close()
			body, err := ioutil.ReadAll(preflightResp.Body)
			if err != nil {
				return err
			}

			logger.Printf("%s\n", body)
			receivedPreflights = append(receivedPreflights, readyPreflight)
		}

		if len(job.Status.AnalyzersRunning) == 0 {
			return nil
		}
	}
}
