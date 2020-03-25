package cli

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	troubleshootclientv1beta1 "github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/typed/troubleshoot/v1beta1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func createTroubleshootK8sClient(configFlags *genericclioptions.ConfigFlags) (*troubleshootclientv1beta1.TroubleshootV1beta1Client, error) {
	config, err := configFlags.ToRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	troubleshootClient, err := troubleshootclientv1beta1.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create troubleshoot client")
	}

	return troubleshootClient, nil
}

func findFileName(basename, extension string) (string, error) {
	n := 1
	name := basename
	for {
		filename := name + "." + extension
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			return filename, nil
		} else if err != nil {
			return "", errors.Wrap(err, "check file exists")
		}

		name = fmt.Sprintf("%s (%d)", basename, n)
		n = n + 1
	}
}
