package preflight

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/internal/specs"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/loader"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
)

func readSpecs(args []string) (*loader.TroubleshootKinds, error) {
	config, err := k8sutil.GetRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert create k8s client")
	}

	ctx := context.Background()
	kinds, err := specs.LoadFromCLIArgs(ctx, client, args, viper.GetViper())
	if err != nil {
		return nil, err
	}

	// Concatenate all preflight specs that don't have an upload destination
	specs := []troubleshootv1beta2.Preflight{}
	var concatenatedSpec *troubleshootv1beta2.Preflight
	for _, v := range kinds.PreflightsV1Beta2 {
		v := v // https://golang.org/doc/faq#closures_and_goroutines
		if v.Spec.UploadResultsTo == "" {
			concatenatedSpec = ConcatPreflightSpec(concatenatedSpec, &v)
		} else {
			specs = append(specs, v)
		}
	}

	if concatenatedSpec != nil {
		specs = append(specs, *concatenatedSpec)
	}
	kinds.PreflightsV1Beta2 = specs

	var hostSpec *troubleshootv1beta2.HostPreflight
	for _, v := range kinds.HostPreflightsV1Beta2 {
		v := v // https://golang.org/doc/faq#closures_and_goroutines
		hostSpec = ConcatHostPreflightSpec(hostSpec, &v)
	}
	if hostSpec != nil {
		kinds.HostPreflightsV1Beta2 = []troubleshootv1beta2.HostPreflight{*hostSpec}
	}

	return kinds, nil
}
