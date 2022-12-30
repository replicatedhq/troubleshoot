package collect

import (
	"context"
	"regexp"

	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func checkInstallersAddOn(config *rest.Config, name string) (bool, error) {
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return false, errors.Wrap(err, "failed to create kubernetes client")
	}

	installersResourcesBytes, err := client.RESTClient().Get().AbsPath("/apis/cluster.kurl.sh/v1beta1/installers/").
		DoRaw(context.TODO())

	if err != nil {
		return false, errors.Wrap(err, string(installersResourcesBytes))
	}

	r, _ := regexp.Compile(name)

	installersResources := string(installersResourcesBytes)
	addOneEnabled := r.MatchString(installersResources)

	return addOneEnabled, nil
}
