package cli

import (
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
